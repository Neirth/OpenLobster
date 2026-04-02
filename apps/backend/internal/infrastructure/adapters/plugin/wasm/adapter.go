package wasm

import (
	"context"
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
	mu        sync.Mutex
	failures  int
	retryAt   time.Time
	lastError error
	builtin   bool
	available bool

	callTimeout time.Duration
	allowFS     bool
	dataDir     string
}

// NewAdapter loads and instantiates an Extism plugin.
func NewAdapter(ctx context.Context, rt *Runtime, wasmPath string, callTimeout time.Duration, allowFS bool, dataDir string) (*Adapter, error) {
	if callTimeout <= 0 {
		callTimeout = 10 * time.Second
	}
	callTimeoutMs := uint64(callTimeout / time.Millisecond)
	plugin, err := rt.newPlugin(ctx, wasmPath, callTimeoutMs, allowFS, dataDir)
	if err != nil {
		return nil, fmt.Errorf("plugin: instantiate %s: %w", wasmPath, err)
	}

	stem := moduleStem(wasmPath)
	return &Adapter{
		id:          stem,
		plugin:      plugin,
		ctx:         ctx,
		rt:          rt,
		wasmPath:    wasmPath,
		callTimeout: callTimeout,
		available:   true,
		allowFS:     allowFS,
		dataDir:     dataDir,
	}, nil
}

// ID returns the plugin's file stem (e.g. "openlobster-ai-openai").
func (a *Adapter) ID() string { return a.id }

// Name calls openlobster_get_name and returns the result.
func (a *Adapter) Name() string {
	out, err := a.callNoInput("get_name")
	if err != nil || len(out) == 0 {
		return a.id
	}
	return string(out)
}

// Version calls openlobster_get_version.
func (a *Adapter) Version() string {
	out, _ := a.callNoInput("get_version")
	return string(out)
}

// Description calls openlobster_get_description.
func (a *Adapter) Description() string {
	out, _ := a.callNoInput("get_description")
	return string(out)
}

// Type calls openlobster_get_type and returns "ai", "messaging", "memory", or "tool".
func (a *Adapter) Type() string {
	out, _ := a.callNoInput("get_type")
	return string(out)
}

// Schema calls openlobster_get_schema and returns raw JSON Schema bytes.
func (a *Adapter) Schema() ([]byte, error) {
	return a.callNoInput("get_schema")
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
		a.handleFailureLocked(function, err)
		return nil, err
	}
	a.available = true
	a.lastError = nil
	a.failures = 0
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
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.available
}

// LastError returns the last call/runtime error observed by this plugin.
func (a *Adapter) LastError() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastError == nil {
		return ""
	}
	return a.lastError.Error()
}

// Builtin reports whether this plugin is part of the builtin catalog.
func (a *Adapter) Builtin() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.builtin
}

// SetBuiltin marks the plugin as builtin/non-builtin.
func (a *Adapter) SetBuiltin(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.builtin = v
}

// ─── Internal helpers ────────────────────────────────────────────────────────

func (a *Adapter) callNoInput(fn string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensureReadyLocked(); err != nil {
		return nil, err
	}
	out, err := a.callLocked(fn, nil)
	if err != nil {
		a.handleFailureLocked(fn, err)
		return nil, err
	}
	return out, nil
}

func (a *Adapter) callLocked(fn string, input []byte) ([]byte, error) {
	if a.plugin == nil {
		return nil, fmt.Errorf("plugin %s unavailable", a.id)
	}
	if !a.plugin.FunctionExists(fn) {
		return nil, fmt.Errorf("plugin %s: function %q not exported", a.id, fn)
	}
	callCtx, cancel := context.WithTimeout(a.ctx, a.callTimeout)
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
			return nil, fmt.Errorf("plugin %s: %s failed (%d): %s", a.id, fn, rc, errMsg)
		}
		return nil, fmt.Errorf("plugin %s: %s failed (%d)", a.id, fn, rc)
	}
	return out, nil
}

func (a *Adapter) ensureReadyLocked() error {
	now := time.Now()
	if a.plugin != nil {
		return nil
	}
	if !a.retryAt.IsZero() && now.Before(a.retryAt) {
		wait := a.retryAt.Sub(now).Round(time.Millisecond)
		return fmt.Errorf("plugin %s unavailable, retry in %s", a.id, wait)
	}

	callTimeoutMs := uint64(a.callTimeout / time.Millisecond)
	plugin, err := a.rt.newPlugin(a.ctx, a.wasmPath, callTimeoutMs, a.allowFS, a.dataDir)
	if err != nil {
		return fmt.Errorf("plugin %s: reload instantiate: %w", a.id, err)
	}

	a.plugin = plugin
	a.available = true
	a.lastError = nil
	return nil
}

func (a *Adapter) handleFailureLocked(function string, err error) {
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

// moduleStem returns the filename without directory path and extension.
func moduleStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return base[:len(base)-len(ext)]
}
