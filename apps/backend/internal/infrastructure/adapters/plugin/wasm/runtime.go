// Package wasm provides the Extism-based WASM plugin runtime.
package wasm

import (
	"context"
	"fmt"

	extism "github.com/extism/go-sdk"
)

// Runtime stores host callbacks used for plugin instantiation.
type Runtime struct {
	onMessage func([]byte)
}

// NewRuntime builds the plugin runtime facade.
func NewRuntime(ctx context.Context, onMessage func([]byte)) (*Runtime, error) {
	_ = ctx
	return &Runtime{onMessage: onMessage}, nil
}

func (rt *Runtime) hostFunctions() []extism.HostFunction {
	if rt == nil || rt.onMessage == nil {
		return nil
	}
	emit := extism.NewHostFunctionWithStack(
		"emit_message",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			offset := stack[0]
			payload, err := p.ReadBytes(offset)
			if err != nil {
				p.Logf(extism.LogLevelError, "host emit_message read failed: %v", err)
				return
			}
			if len(payload) == 0 {
				return
			}
			rt.onMessage(payload)
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{},
	)
	emit.SetNamespace("openlobster")
	return []extism.HostFunction{emit}
}

func (rt *Runtime) newPlugin(ctx context.Context, wasmPath string, callTimeoutMs uint64, allowFS bool, dataDir string) (*extism.Plugin, error) {
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: wasmPath},
		},
		AllowedHosts: []string{"*"},
		Timeout:      callTimeoutMs,
	}
	if allowFS && dataDir != "" {
		manifest.AllowedPaths = map[string]string{dataDir: dataDir}
	}

	cfg := extism.PluginConfig{
		EnableWasi: true,
	}
	p, err := extism.NewPlugin(ctx, manifest, cfg, rt.hostFunctions())
	if err != nil {
		return nil, fmt.Errorf("extism new plugin: %w", err)
	}
	return p, nil
}
