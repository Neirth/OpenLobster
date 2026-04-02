package wasm

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Adapter implements ports.PluginPort for a single loaded WASM plugin.
//
// # Memory calling convention (host ↔ plugin)
//
// Each plugin must export:
//
//	openlobster_alloc_input(size uint32) uint32
//	    Allocates an input buffer of `size` bytes inside the plugin, returns a
//	    pointer to it. The plugin keeps a reference so the GC won't collect it.
//
//	openlobster_result_ptr() uint32
//	openlobster_result_len() uint32
//	    Return the pointer and byte length of the last function's result buffer.
//
//	openlobster_<fn>() int32
//	    Execute the function (reads inputBuf, writes resultBuf). Returns 0 on
//	    success, non-zero on error. For get_name/get_version/get_type functions
//	    that take no input, alloc_input is not called before invoking them.
//
// The host allocates the input, writes JSON to it, calls the function, then
// reads the result via result_ptr/result_len.
type Adapter struct {
	id        string
	mod       api.Module
	ctx       context.Context // per-plugin context carrying the WASI system instance
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

// NewAdapter loads the WASM file at wasmPath, gives it a fresh WASI system
// (including socket support) via rt.InstantiateWASI, compiles and instantiates
// it inside rt, and returns a ready-to-use Adapter.
func NewAdapter(ctx context.Context, rt *Runtime, wasmPath string, callTimeout time.Duration, allowFS bool, dataDir string) (*Adapter, error) {
	if callTimeout <= 0 {
		callTimeout = 10 * time.Second
	}
	code, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("plugin: read %s: %w", wasmPath, err)
	}

	compiled, err := rt.CompileModule(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("plugin: compile %s: %w", wasmPath, err)
	}
	defer compiled.Close(ctx)

	stem := moduleStem(wasmPath)

	// Give this plugin its own WASI system context (socket extensions included).
	pluginCtx, err := rt.InstantiateWASI(ctx, allowFS, dataDir)
	if err != nil {
		return nil, fmt.Errorf("plugin: wasi instantiate %s: %w", stem, err)
	}

	mod, err := rt.InstantiateModule(pluginCtx, compiled, wazero.NewModuleConfig().
		WithName(stem).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		// _initialize runs Go package init() functions without starting main().
		// Plugins must NOT use main() for logic — only for optional long-lived loops.
		WithStartFunctions("_initialize"),
	)
	if err != nil {
		return nil, fmt.Errorf("plugin: instantiate %s: %w", stem, err)
	}

	return &Adapter{
		id:          stem,
		mod:         mod,
		ctx:         pluginCtx,
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

	var (
		out []byte
		err error
	)
	if len(input) == 0 {
		out, err = a.callNoInputLocked(function)
	} else {
		out, err = a.callWithInputLocked(function, input)
	}
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
	if a.mod == nil {
		return nil
	}
	return a.mod.Close(a.ctx)
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
	out, err := a.callNoInputLocked(fn)
	if err != nil {
		a.handleFailureLocked(fn, err)
		return nil, err
	}
	return out, nil
}

func (a *Adapter) callNoInputLocked(fn string) ([]byte, error) {
	fnName := "openlobster_" + fn
	f := a.mod.ExportedFunction(fnName)
	if f == nil {
		return nil, fmt.Errorf("plugin %s: function %q not exported", a.id, fnName)
	}
	callCtx, cancel := context.WithTimeout(a.ctx, a.callTimeout)
	defer cancel()
	ret, err := f.Call(callCtx)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: call %s: %w", a.id, fnName, err)
	}
	if len(ret) > 0 && ret[0] != 0 {
		return nil, fmt.Errorf("plugin %s: %s returned error %d", a.id, fnName, ret[0])
	}
	return a.readResult()
}

func (a *Adapter) callWithInputLocked(fn string, input []byte) ([]byte, error) {
	// 1. Allocate an input buffer inside the plugin.
	allocFn := a.mod.ExportedFunction("openlobster_alloc_input")
	if allocFn == nil {
		return nil, fmt.Errorf("plugin %s: openlobster_alloc_input not exported", a.id)
	}
	callCtx, cancel := context.WithTimeout(a.ctx, a.callTimeout)
	defer cancel()
	res, err := allocFn.Call(callCtx, uint64(len(input)))
	if err != nil {
		return nil, fmt.Errorf("plugin %s: alloc_input: %w", a.id, err)
	}
	ptr := uint32(res[0])

	// 2. Write the JSON input into the plugin's memory.
	if !a.mod.Memory().Write(ptr, input) {
		return nil, fmt.Errorf("plugin %s: memory write out of bounds (ptr=%d len=%d)", a.id, ptr, len(input))
	}

	// 3. Call the plugin function (reads from inputBuf global, writes to resultBuf).
	fnName := "openlobster_" + fn
	f := a.mod.ExportedFunction(fnName)
	if f == nil {
		return nil, fmt.Errorf("plugin %s: function %q not exported", a.id, fnName)
	}
	ret, err := f.Call(callCtx)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: call %s: %w", a.id, fnName, err)
	}
	if len(ret) > 0 && ret[0] != 0 {
		return nil, fmt.Errorf("plugin %s: %s returned error %d", a.id, fnName, ret[0])
	}
	return a.readResult()
}

// readResult reads the output written by the plugin into its resultBuf global.
func (a *Adapter) readResult() ([]byte, error) {
	ptrFn := a.mod.ExportedFunction("openlobster_result_ptr")
	lenFn := a.mod.ExportedFunction("openlobster_result_len")
	if ptrFn == nil || lenFn == nil {
		return nil, nil
	}
	ptrRes, err := ptrFn.Call(a.ctx)
	if err != nil || len(ptrRes) == 0 {
		return nil, err
	}
	lenRes, err := lenFn.Call(a.ctx)
	if err != nil || len(lenRes) == 0 {
		return nil, err
	}
	ptr := uint32(ptrRes[0])
	length := uint32(lenRes[0])
	if ptr == 0 || length == 0 {
		return nil, nil
	}
	data, ok := a.mod.Memory().Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("plugin %s: result memory read out of bounds (ptr=%d len=%d)", a.id, ptr, length)
	}
	out := make([]byte, length)
	copy(out, data)
	return out, nil
}

func (a *Adapter) ensureReadyLocked() error {
	now := time.Now()
	if a.mod != nil {
		return nil
	}
	if !a.retryAt.IsZero() && now.Before(a.retryAt) {
		wait := a.retryAt.Sub(now).Round(time.Millisecond)
		return fmt.Errorf("plugin %s unavailable, retry in %s", a.id, wait)
	}

	code, err := os.ReadFile(a.wasmPath)
	if err != nil {
		return fmt.Errorf("plugin %s: reload read: %w", a.id, err)
	}
	compiled, err := a.rt.CompileModule(a.ctx, code)
	if err != nil {
		return fmt.Errorf("plugin %s: reload compile: %w", a.id, err)
	}
	defer compiled.Close(a.ctx)

	pluginCtx, err := a.rt.InstantiateWASI(a.ctx, a.allowFS, a.dataDir)
	if err != nil {
		return fmt.Errorf("plugin %s: reload wasi: %w", a.id, err)
	}
	mod, err := a.rt.InstantiateModule(pluginCtx, compiled, wazero.NewModuleConfig().
		WithName(moduleStem(a.wasmPath)).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithStartFunctions("_initialize"),
	)
	if err != nil {
		return fmt.Errorf("plugin %s: reload instantiate: %w", a.id, err)
	}

	a.ctx = pluginCtx
	a.mod = mod
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
	if a.mod != nil {
		_ = a.mod.Close(a.ctx)
		a.mod = nil
	}
}

// moduleStem returns the filename without directory path and extension.
func moduleStem(path string) string {
	base := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			base = path[i+1:]
			break
		}
	}
	for i := len(base) - 1; i > 0; i-- {
		if base[i] == '.' {
			return base[:i]
		}
	}
	return base
}
