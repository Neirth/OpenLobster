// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subprocess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

var errPluginFunctionFailed = errors.New("plugin function failed")

const (
	defaultCallTimeout        = 10 * time.Second
	minLongRunningCallTimeout = 02 * time.Minute
	minStartCallTimeout       = 05 * time.Minute
	handshakeTimeout          = 10 * time.Second
)

// Adapter implements ports.PluginPort using a native subprocess and JSON-RPC over stdin/stdout.
type Adapter struct {
	id         string
	binaryPath string
	onMessage  func([]byte)

	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	conn        *jrpcConn
	eventCancel context.CancelFunc

	stateMu   sync.RWMutex
	failures  int
	retryAt   time.Time
	lastError error
	available bool
	builtin   bool

	callTimeout time.Duration
	exports     map[string]struct{}

	name        string
	version     string
	description string
	pluginType  string
	schemaJSON  []byte
	properties  json.RawMessage
}

func NewAdapter(ctx context.Context, binaryPath string, onMessage func([]byte), callTimeout time.Duration) (*Adapter, error) {
	if strings.TrimSpace(binaryPath) == "" {
		return nil, fmt.Errorf("subprocess: binary path is required")
	}
	if callTimeout <= 0 {
		callTimeout = defaultCallTimeout
	}

	a := &Adapter{
		id:          moduleStem(binaryPath),
		binaryPath:  binaryPath,
		onMessage:   onMessage,
		callTimeout: callTimeout,
		available:   true,
		schemaJSON:  []byte("{}"),
		exports:     make(map[string]struct{}),
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.startLocked(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adapter) ID() string { return a.id }

func (a *Adapter) Name() string {
	if strings.TrimSpace(a.name) == "" {
		return a.id
	}
	return a.name
}

func (a *Adapter) Version() string { return a.version }

func (a *Adapter) Description() string { return a.description }

func (a *Adapter) Type() string { return a.pluginType }

func (a *Adapter) Schema() ([]byte, error) {
	if len(a.schemaJSON) == 0 {
		return []byte("{}"), nil
	}
	out := make([]byte, len(a.schemaJSON))
	copy(out, a.schemaJSON)
	return out, nil
}

func (a *Adapter) Properties() []byte {
	if len(a.properties) == 0 {
		return []byte("{}")
	}
	out := make([]byte, len(a.properties))
	copy(out, a.properties)
	return out
}

func (a *Adapter) HasFunction(function string) bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	_, ok := a.exports[strings.TrimSpace(function)]
	return ok
}

func (a *Adapter) Available() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.available
}

func (a *Adapter) LastError() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	if a.lastError == nil {
		return ""
	}
	return a.lastError.Error()
}

func (a *Adapter) Builtin() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.builtin
}

func (a *Adapter) SetBuiltin(v bool) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.builtin = v
}

// CreateLoopRunner spawns an independent plugin subprocess intended for
// blocking messaging start loops so primary calls (send/resolve) remain responsive.
func (a *Adapter) CreateLoopRunner() (ports.PluginPort, error) {
	runner, err := NewAdapter(context.Background(), a.binaryPath, a.onMessage, a.callTimeout)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: create loop runner: %w", a.id, err)
	}
	runner.SetBuiltin(a.Builtin())
	return runner, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopLocked()
}

func (a *Adapter) Call(function string, input []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureReadyLocked(context.Background()); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), effectiveCallTimeout(function, a.callTimeout))
	defer cancel()

	// Flat call: method IS the function name, params ARE the input.
	var resp callResponse
	if err := a.conn.call(ctx, function, json.RawMessage(input), &resp); err != nil {
		if shouldTripCircuit(function, err) {
			a.handleFailureLocked(function, err)
		}
		return nil, fmt.Errorf("plugin %s: call %s: %w", a.id, function, err)
	}
	if strings.TrimSpace(resp.Error) != "" {
		return nil, fmt.Errorf("%w: plugin %s: %s failed: %s", errPluginFunctionFailed, a.id, function, resp.Error)
	}

	a.stateMu.Lock()
	a.available = true
	a.lastError = nil
	a.failures = 0
	a.stateMu.Unlock()

	return append([]byte(nil), resp.Output...), nil
}

func (a *Adapter) ensureReadyLocked(ctx context.Context) error {
	if a.conn != nil {
		return nil
	}

	now := time.Now()
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

	return a.startLocked(ctx)
}

func (a *Adapter) handleFailureLocked(function string, err error) {
	if err == nil {
		return
	}

	a.stateMu.Lock()
	a.available = false
	a.lastError = fmt.Errorf("plugin %s %s failed: %w", a.id, function, err)
	a.failures++
	backoff := time.Second * time.Duration(1<<min(a.failures-1, 5))
	a.retryAt = time.Now().Add(backoff)
	a.stateMu.Unlock()

	_ = a.stopLocked()
}

func moduleStem(path string) string {
	base := filepath.Base(path)
	if base == "" {
		return ""
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

func effectiveCallTimeout(function string, configured time.Duration) time.Duration {
	timeout := configured
	if timeout <= 0 {
		timeout = defaultCallTimeout
	}

	switch function {
	case "start":
		if timeout < minStartCallTimeout {
			return minStartCallTimeout
		}
	case "chat", "tts", "stt":
		if timeout < minLongRunningCallTimeout {
			return minLongRunningCallTimeout
		}
	}

	return timeout
}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
