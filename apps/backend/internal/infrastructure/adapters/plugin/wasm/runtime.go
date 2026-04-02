// Package wasm provides the wazero-based WASM plugin runtime with full
// WASI Preview 1 support including WasmEdge socket extensions, enabling
// plugins to use standard Go networking (net.Dial, gorilla/websocket, etc.)
// via github.com/stealthrocket/net/wasip1.
package wasm

import (
	"context"
	"strings"

	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Runtime wraps a wazero.Runtime and a pre-configured WASI builder.
// Each plugin gets its own WASI system context by calling Build.
type Runtime struct {
	wazero.Runtime
}

// NewRuntime creates a wazero runtime configured with:
//   - The "openlobster" host module (host_emit_message for messaging plugins).
//   - A WASI builder pre-configured with WasmEdge socket extensions
//     (sock_open, sock_connect, sock_recv, sock_send, sock_getaddrinfo, …).
//
// onMessage is called by messaging plugins to deliver inbound messages.
// Call Close on the returned Runtime when the process exits.
func NewRuntime(ctx context.Context, onMessage func([]byte)) (*Runtime, error) {
	r := wazero.NewRuntime(ctx)

	// Register the openlobster host module so messaging plugins can deliver
	// inbound messages to the host via host_emit_message(ptr, len).
	_, err := r.NewHostModuleBuilder("openlobster").
		NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				ptr := uint32(stack[0])
				length := uint32(stack[1])
				if data, ok := mod.Memory().Read(ptr, length); ok && onMessage != nil {
					buf := make([]byte, length)
					copy(buf, data)
					onMessage(buf)
				}
			}),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{},
		).
		Export("host_emit_message").
		Instantiate(ctx)
	if err != nil {
		r.Close(ctx)
		return nil, err
	}

	return &Runtime{Runtime: r}, nil
}

// InstantiateWASI sets up a per-plugin WASI system (including socket support)
// and returns the context enriched with that system. The returned context must
// be used for all wazero calls related to the plugin that called this method.
func (rt *Runtime) InstantiateWASI(ctx context.Context, allowFS bool, dataDir string) (context.Context, error) {
	builder := imports.NewBuilder().WithSocketsExtension("wasmedgev1", nil)
	if allowFS {
		dataDir = strings.TrimSpace(dataDir)
		if dataDir != "" {
			builder = builder.WithDirs(dataDir)
		}
	}
	newCtx, _, err := builder.Instantiate(ctx, rt.Runtime)
	return newCtx, err
}
