package wasm

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	extism "github.com/extism/go-sdk"
)

// Adapter implements ports.PluginPort for a single loaded Extism plugin.
type Adapter struct {
	id        string
	plugin    *extism.Plugin
	ctx       context.Context
	rt        *Runtime
	wasmPath  string
	wasmData  []byte
	mu        sync.Mutex
	stateMu   sync.RWMutex
	failures  int
	retryAt   time.Time
	lastError error
	builtin   bool
	available bool

	callTimeout time.Duration
	allowFS     bool
	dataDir     string

	name        string
	version     string
	description string
	pluginType  string
	schemaJSON  []byte
}

var errPluginFunctionFailed = errors.New("plugin function failed")

const minLongRunningCallTimeout = 2 * time.Minute

// NewAdapter loads and instantiates an Extism plugin.
func NewAdapter(ctx context.Context, rt *Runtime, wasmPath string, callTimeout time.Duration, allowFS bool, dataDir string) (*Adapter, error) {
	if callTimeout <= 0 {
		callTimeout = 10 * time.Second
	}
	callTimeoutMs := uint64(callTimeout / time.Millisecond)
	plugin, err := rt.newPluginFromPath(ctx, wasmPath, callTimeoutMs, allowFS, dataDir)
	if err != nil {
		return nil, fmt.Errorf("plugin: instantiate %s: %w", wasmPath, err)
	}

	stem := moduleStem(wasmPath)
	a := &Adapter{
		id:          stem,
		plugin:      plugin,
		ctx:         ctx,
		rt:          rt,
		wasmPath:    wasmPath,
		callTimeout: callTimeout,
		available:   true,
		allowFS:     allowFS,
		dataDir:     dataDir,
	}
	a.primeStaticMetadata()
	return a, nil
}

// NewEmbeddedAdapter loads and instantiates an Extism plugin from in-memory WASM bytes.
func NewEmbeddedAdapter(ctx context.Context, rt *Runtime, wasmName string, wasmData []byte, callTimeout time.Duration, allowFS bool, dataDir string) (*Adapter, error) {
	if len(wasmData) == 0 {
		return nil, fmt.Errorf("plugin: instantiate %s: empty wasm payload", wasmName)
	}
	if callTimeout <= 0 {
		callTimeout = 10 * time.Second
	}
	callTimeoutMs := uint64(callTimeout / time.Millisecond)
	plugin, err := rt.newPluginFromData(ctx, wasmName, wasmData, callTimeoutMs, allowFS, dataDir)
	if err != nil {
		return nil, fmt.Errorf("plugin: instantiate %s: %w", wasmName, err)
	}

	stem := moduleStem(wasmName)
	copiedData := make([]byte, len(wasmData))
	copy(copiedData, wasmData)

	a := &Adapter{
		id:          stem,
		plugin:      plugin,
		ctx:         ctx,
		rt:          rt,
		wasmPath:    wasmName,
		wasmData:    copiedData,
		callTimeout: callTimeout,
		available:   true,
		allowFS:     allowFS,
		dataDir:     dataDir,
	}
	a.primeStaticMetadata()
	return a, nil
}

// ID returns the plugin's file stem (e.g. "openlobster-ai-openai").
func (a *Adapter) ID() string { return a.id }

// Name calls openlobster_get_name and returns the result.
func (a *Adapter) Name() string {
	if a.name == "" {
		return a.id
	}
	return a.name
}

// Version calls openlobster_get_version.
func (a *Adapter) Version() string {
	return a.version
}

// Description calls openlobster_get_description.
func (a *Adapter) Description() string {
	return a.description
}

// Type calls openlobster_get_type and returns "ai", "messaging", "memory", or "tool".
func (a *Adapter) Type() string {
	return a.pluginType
}

// Schema calls openlobster_get_schema and returns raw JSON Schema bytes.
func (a *Adapter) Schema() ([]byte, error) {
	if len(a.schemaJSON) == 0 {
		return []byte("{}"), nil
	}
	out := make([]byte, len(a.schemaJSON))
	copy(out, a.schemaJSON)
	return out, nil
}

// Call invokes an arbitrary named plugin function with JSON input.
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

// Close releases all WASM resources held by this plugin.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.plugin == nil {
		return nil
	}
	err := a.plugin.Close(a.ctx)
	a.plugin = nil
	return err
}

// Available returns whether the plugin is currently available for calls.
func (a *Adapter) Available() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.available
}

// LastError returns the last call/runtime error observed by this plugin.
func (a *Adapter) LastError() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	if a.lastError == nil {
		return ""
	}
	return a.lastError.Error()
}

// Builtin reports whether this plugin is part of the builtin catalog.
func (a *Adapter) Builtin() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.builtin
}

// SetBuiltin marks the plugin as builtin/non-builtin.
func (a *Adapter) SetBuiltin(v bool) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.builtin = v
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// shouldTripCircuit decides whether a function error should mark the plugin as
// unavailable with exponential backoff.
//
// Runtime request handlers such as "chat" can fail due provider/network issues
// while the WASM plugin remains healthy; tripping the circuit in those cases
// causes noisy "unavailable, retry in ..." loops.
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

func (a *Adapter) callLocked(fn string, input []byte) ([]byte, error) {
	if a.plugin == nil {
		return nil, fmt.Errorf("plugin %s unavailable", a.id)
	}
	if !a.plugin.FunctionExists(fn) {
		return nil, fmt.Errorf("plugin %s: function %q not exported", a.id, fn)
	}
	callCtx := a.ctx
	cancel := func() {}
	if fn != "start" {
		timeout := effectiveCallTimeout(fn, a.callTimeout)
		if timeout > 0 {
			callCtx, cancel = context.WithTimeout(a.ctx, timeout)
		}
	}
	defer cancel()
	rc, out, err := a.plugin.CallWithContext(callCtx, fn, input)
	if err != nil {
		errMsg := a.plugin.GetErrorWithContext(callCtx)
		if errMsg != "" {
			return nil, fmt.Errorf("plugin %s: call %s: %s (%w)", a.id, fn, errMsg, err)
		}
		return nil, fmt.Errorf("plugin %s: call %s: %w", a.id, fn, err)
	}
	if rc != 0 {
		errMsg := a.plugin.GetErrorWithContext(callCtx)
		if errMsg != "" {
			return nil, fmt.Errorf("%w: plugin %s: %s failed (%d): %s", errPluginFunctionFailed, a.id, fn, rc, errMsg)
		}
		return nil, fmt.Errorf("%w: plugin %s: %s failed (%d)", errPluginFunctionFailed, a.id, fn, rc)
	}
	return out, nil
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

func (a *Adapter) ensureReadyLocked() error {
	now := time.Now()
	if a.plugin != nil {
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

	callTimeoutMs := uint64(a.callTimeout / time.Millisecond)
	var (
		plugin *extism.Plugin
		err    error
	)
	if len(a.wasmData) > 0 {
		plugin, err = a.rt.newPluginFromData(a.ctx, a.wasmPath, a.wasmData, callTimeoutMs, a.allowFS, a.dataDir)
	} else {
		plugin, err = a.rt.newPluginFromPath(a.ctx, a.wasmPath, callTimeoutMs, a.allowFS, a.dataDir)
	}
	if err != nil {
		return fmt.Errorf("plugin %s: reload instantiate: %w", a.id, err)
	}

	a.plugin = plugin
	a.stateMu.Lock()
	a.available = true
	a.lastError = nil
	a.stateMu.Unlock()
	return nil
}

func (a *Adapter) handleFailureLocked(function string, err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.failures++
	a.available = false
	a.lastError = fmt.Errorf("plugin %s %s failed: %w", a.id, function, err)
	backoff := time.Second * time.Duration(1<<min(a.failures-1, 5))
	a.retryAt = time.Now().Add(backoff)
	if a.plugin != nil {
		_ = a.plugin.Close(a.ctx)
		a.plugin = nil
	}
}

func (a *Adapter) primeStaticMetadata() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.name = a.id
	a.version = ""
	a.description = ""
	a.pluginType = ""
	a.schemaJSON = []byte("{}")

	if out, err := a.callLocked("get_name", nil); err == nil && len(out) > 0 {
		a.name = string(out)
	}
	if out, err := a.callLocked("get_version", nil); err == nil {
		a.version = string(out)
	}
	if out, err := a.callLocked("get_description", nil); err == nil {
		a.description = string(out)
	}
	if out, err := a.callLocked("get_type", nil); err == nil {
		a.pluginType = string(out)
	}
	if out, err := a.callLocked("get_schema", nil); err == nil && len(out) > 0 {
		a.schemaJSON = make([]byte, len(out))
		copy(a.schemaJSON, out)
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
