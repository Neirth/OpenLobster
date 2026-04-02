package ipc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

var errPluginFunctionFailed = errors.New("plugin function failed")

const minLongRunningCallTimeout = 2 * time.Minute

// Adapter implements ports.PluginPort over a helper process using net/rpc.
type Adapter struct {
	id string

	helperPath   string
	sourcePath   string
	embeddedPath string
	allowFS      bool
	dataDir      string
	onMessage    func([]byte)

	mu       sync.Mutex
	stateMu  sync.RWMutex
	client   *Client
	failures int
	retryAt  time.Time

	lastError error
	available bool
	builtin   bool

	callTimeout time.Duration

	name        string
	version     string
	description string
	pluginType  string
	schemaJSON  []byte
}

// NewExternalAdapter starts one helper process for an external *.wasm plugin.
func NewExternalAdapter(helperPath, wasmPath string, callTimeout time.Duration, allowFS bool, dataDir string, onMessage func([]byte)) (*Adapter, error) {
	if callTimeout <= 0 {
		callTimeout = 10 * time.Second
	}
	a := &Adapter{
		id:          moduleStem(wasmPath),
		helperPath:  helperPath,
		sourcePath:  wasmPath,
		allowFS:     allowFS,
		dataDir:     dataDir,
		onMessage:   onMessage,
		available:   true,
		callTimeout: callTimeout,
		schemaJSON:  []byte("{}"),
	}
	if err := a.startLocked(); err != nil {
		return nil, err
	}
	return a, nil
}

// NewEmbeddedAdapter starts one helper process for an embedded plugin payload.
// embeddedPath must point to a path available in the helper binary embedded fs.
func NewEmbeddedAdapter(helperPath, embeddedPath string, callTimeout time.Duration, allowFS bool, dataDir string, onMessage func([]byte)) (*Adapter, error) {
	if callTimeout <= 0 {
		callTimeout = 10 * time.Second
	}
	a := &Adapter{
		id:           moduleStem(embeddedPath),
		helperPath:   helperPath,
		embeddedPath: embeddedPath,
		allowFS:      allowFS,
		dataDir:      dataDir,
		onMessage:    onMessage,
		available:    true,
		callTimeout:  callTimeout,
		schemaJSON:   []byte("{}"),
	}
	if err := a.startLocked(); err != nil {
		return nil, err
	}
	return a, nil
}

// ID returns the plugin unique identifier.
func (a *Adapter) ID() string { return a.id }

// Name returns the plugin name.
func (a *Adapter) Name() string {
	if a.name == "" {
		return a.id
	}
	return a.name
}

// Version returns the plugin version.
func (a *Adapter) Version() string { return a.version }

// Description returns the plugin description.
func (a *Adapter) Description() string { return a.description }

// Type returns plugin type.
func (a *Adapter) Type() string { return a.pluginType }

// Schema returns plugin config schema JSON.
func (a *Adapter) Schema() ([]byte, error) {
	if len(a.schemaJSON) == 0 {
		return []byte("{}"), nil
	}
	out := make([]byte, len(a.schemaJSON))
	copy(out, a.schemaJSON)
	return out, nil
}

// Call invokes a plugin function over IPC.
func (a *Adapter) Call(function string, input []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureReadyLocked(); err != nil {
		return nil, err
	}

	out, err := a.callLocked(function, input)
	if err != nil {
		if shouldTripCircuit(function, err) {
			a.handleFailureLocked(function, err)
		}
		return nil, err
	}

	a.stateMu.Lock()
	a.available = true
	a.lastError = nil
	a.failures = 0
	a.stateMu.Unlock()

	return out, nil
}

// Close terminates helper resources.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil {
		return nil
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = a.client.Call(closeCtx, "Plugin.Close", EmptyArgs{}, &EmptyReply{})
	err := a.client.Close()
	a.client = nil
	return err
}

// Available reports plugin runtime availability.
func (a *Adapter) Available() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.available
}

// LastError reports the latest runtime/call error.
func (a *Adapter) LastError() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	if a.lastError == nil {
		return ""
	}
	return a.lastError.Error()
}

// Builtin reports whether plugin belongs to builtin catalog.
func (a *Adapter) Builtin() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.builtin
}

// SetBuiltin marks plugin builtin/non-builtin.
func (a *Adapter) SetBuiltin(v bool) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.builtin = v
}

// CreateLoopRunner creates a dedicated plugin process instance suitable for
// long-running messaging start loops so outbound calls can continue using the
// primary adapter.
func (a *Adapter) CreateLoopRunner() (ports.PluginPort, error) {
	a.stateMu.RLock()
	builtin := a.builtin
	a.stateMu.RUnlock()

	var (
		runner *Adapter
		err    error
	)
	if a.sourcePath != "" {
		runner, err = NewExternalAdapter(a.helperPath, a.sourcePath, a.callTimeout, a.allowFS, a.dataDir, a.onMessage)
	} else {
		runner, err = NewEmbeddedAdapter(a.helperPath, a.embeddedPath, a.callTimeout, a.allowFS, a.dataDir, a.onMessage)
	}
	if err != nil {
		return nil, err
	}
	if builtin {
		runner.SetBuiltin(true)
	}
	return runner, nil
}

func (a *Adapter) startLocked() error {
	args := []string{"plugin-host"}
	if a.sourcePath != "" {
		args = append(args, "--wasm-path", a.sourcePath)
	}
	if a.embeddedPath != "" {
		args = append(args, "--embedded-path", a.embeddedPath)
	}
	args = append(args, "--call-timeout", a.callTimeout.String())
	if a.allowFS {
		args = append(args, "--allow-fs")
	}
	if a.dataDir != "" {
		args = append(args, "--data-dir", a.dataDir)
	}

	client, err := StartClient(a.helperPath, a.id, args, a.onMessage)
	if err != nil {
		return fmt.Errorf("plugin: start helper for %s: %w", a.id, err)
	}

	startupTimeout := 30 * time.Second
	if a.callTimeout > 0 {
		candidate := a.callTimeout * 3
		if candidate > startupTimeout {
			startupTimeout = candidate
		}
	}
	infoCtx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	var info PluginInfo
	if err := client.Call(infoCtx, "Plugin.Info", EmptyArgs{}, &info); err != nil {
		_ = client.Close()
		return fmt.Errorf("plugin %s: helper info handshake failed: %w", a.id, err)
	}

	a.client = client
	if info.ID != "" {
		a.id = info.ID
	}
	a.name = info.Name
	a.version = info.Version
	a.description = info.Description
	a.pluginType = info.Type
	if len(info.Schema) > 0 {
		a.schemaJSON = make([]byte, len(info.Schema))
		copy(a.schemaJSON, info.Schema)
	}

	a.stateMu.Lock()
	a.available = true
	a.lastError = nil
	a.stateMu.Unlock()

	return nil
}

func (a *Adapter) ensureReadyLocked() error {
	now := time.Now()
	if a.client != nil {
		return nil
	}

	a.stateMu.RLock()
	retryAt := a.retryAt
	lastError := a.lastError
	a.stateMu.RUnlock()

	if !retryAt.IsZero() && now.Before(retryAt) {
		wait := retryAt.Sub(now).Round(time.Millisecond)
		if lastError != nil {
			return fmt.Errorf("plugin %s unavailable, retry in %s (last error: %v)", a.id, wait, lastError)
		}
		return fmt.Errorf("plugin %s unavailable, retry in %s", a.id, wait)
	}

	if err := a.startLocked(); err != nil {
		return err
	}
	return nil
}

func (a *Adapter) callLocked(function string, input []byte) ([]byte, error) {
	if a.client == nil {
		return nil, fmt.Errorf("plugin %s unavailable", a.id)
	}

	callCtx := context.Background()
	cancel := func() {}
	if function != "start" {
		timeout := effectiveCallTimeout(function, a.callTimeout)
		if timeout > 0 {
			callCtx, cancel = context.WithTimeout(context.Background(), timeout)
		}
	}
	defer cancel()

	var reply CallReply
	err := a.client.Call(callCtx, "Plugin.Call", CallArgs{Function: function, Input: input}, &reply)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: call %s: %w", a.id, function, err)
	}
	if reply.Error != "" {
		return nil, fmt.Errorf("%w: plugin %s: %s failed: %s", errPluginFunctionFailed, a.id, function, reply.Error)
	}
	return reply.Output, nil
}

func effectiveCallTimeout(function string, configured time.Duration) time.Duration {
	timeout := configured
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	switch function {
	case "chat", "tts", "stt":
		if timeout < minLongRunningCallTimeout {
			return minLongRunningCallTimeout
		}
	}

	return timeout
}

func (a *Adapter) handleFailureLocked(function string, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	a.failures++
	a.available = false
	a.lastError = fmt.Errorf("plugin %s %s failed: %w", a.id, function, err)
	backoff := time.Second * time.Duration(1<<min(a.failures-1, 5))
	a.retryAt = time.Now().Add(backoff)

	if a.client != nil {
		_ = a.client.Close()
		a.client = nil
	}
}

// shouldTripCircuit decides whether a function error should mark the plugin as
// unavailable with exponential backoff.
func shouldTripCircuit(function string, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errPluginFunctionFailed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	switch function {
	case "chat", "tts", "stt", "send", "store", "retrieve", "query", "get", "set", "delete", "list":
		return false
	default:
		return true
	}
}

// moduleStem returns the filename without directory path and extension.
func moduleStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return base[:len(base)-len(ext)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
