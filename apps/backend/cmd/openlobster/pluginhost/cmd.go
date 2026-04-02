// Package pluginhost runs a hidden helper command used by the main process
// to host one plugin per subprocess over stdio RPC.
package pluginhost

import (
	"context"
	"fmt"
	"io/fs"
	"net/rpc"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/neirth/openlobster/internal/infrastructure/adapters/plugin/ipc"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/plugin/wasm"
)

type pluginRPC struct {
	adapter *wasm.Adapter
}

func (s *pluginRPC) Info(_ ipc.EmptyArgs, reply *ipc.PluginInfo) error {
	if s == nil || s.adapter == nil {
		return fmt.Errorf("plugin helper: adapter unavailable")
	}
	schema, err := s.adapter.Schema()
	if err != nil {
		return err
	}
	*reply = ipc.PluginInfo{
		ID:          s.adapter.ID(),
		Name:        s.adapter.Name(),
		Version:     s.adapter.Version(),
		Description: s.adapter.Description(),
		Type:        s.adapter.Type(),
		Schema:      schema,
	}
	return nil
}

func (s *pluginRPC) Call(args ipc.CallArgs, reply *ipc.CallReply) error {
	if s == nil || s.adapter == nil {
		return fmt.Errorf("plugin helper: adapter unavailable")
	}
	out, err := s.adapter.Call(args.Function, args.Input)
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.Output = out
	reply.Error = ""
	return nil
}

func (s *pluginRPC) Close(_ ipc.EmptyArgs, _ *ipc.EmptyReply) error {
	if s == nil || s.adapter == nil {
		return nil
	}
	return s.adapter.Close()
}

// Command returns the hidden plugin helper subcommand.
func Command(builtinPluginsFS fs.FS) *cobra.Command {
	var wasmPath string
	var embeddedPath string
	var allowFS bool
	var dataDir string
	var callTimeout time.Duration

	cmd := &cobra.Command{
		Use:           "plugin-host",
		Short:         "Internal plugin subprocess host",
		Hidden:        true,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(builtinPluginsFS, wasmPath, embeddedPath, callTimeout, allowFS, dataDir)
		},
	}

	cmd.Flags().StringVar(&wasmPath, "wasm-path", "", "Path to external wasm plugin")
	cmd.Flags().StringVar(&embeddedPath, "embedded-path", "", "Path to embedded wasm plugin")
	cmd.Flags().BoolVar(&allowFS, "allow-fs", false, "Allow plugin filesystem access")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Allowed plugin data directory")
	cmd.Flags().DurationVar(&callTimeout, "call-timeout", 10*time.Second, "Plugin call timeout")

	return cmd
}

func run(builtinPluginsFS fs.FS, wasmPath, embeddedPath string, callTimeout time.Duration, allowFS bool, dataDir string) error {
	if (wasmPath == "") == (embeddedPath == "") {
		return fmt.Errorf("plugin-host: exactly one of --wasm-path or --embedded-path is required")
	}

	hostClient, hostConn, err := hostCallbackClientFromEnv()
	if err != nil {
		return err
	}
	if hostConn != nil {
		defer hostConn.Close()
	}

	onMessage := func(payload []byte) {
		if hostClient == nil || len(payload) == 0 {
			return
		}
		var ack ipc.EmptyReply
		_ = hostClient.Call("Host.EmitMessage", ipc.EmitMessageArgs{Payload: payload}, &ack)
	}

	ctx := context.Background()
	rt, err := wasm.NewRuntime(ctx, onMessage)
	if err != nil {
		return fmt.Errorf("plugin-host: create runtime: %w", err)
	}

	var adapter *wasm.Adapter
	if wasmPath != "" {
		adapter, err = wasm.NewAdapter(ctx, rt, wasmPath, callTimeout, allowFS, dataDir)
		if err != nil {
			return err
		}
	} else {
		wasmPayload, readErr := fs.ReadFile(builtinPluginsFS, embeddedPath)
		if readErr != nil {
			return fmt.Errorf("plugin-host: read embedded %s: %w", embeddedPath, readErr)
		}
		adapter, err = wasm.NewEmbeddedAdapter(ctx, rt, embeddedPath, wasmPayload, callTimeout, allowFS, dataDir)
		if err != nil {
			return err
		}
	}
	defer adapter.Close()

	srv := rpc.NewServer()
	if err := srv.RegisterName("Plugin", &pluginRPC{adapter: adapter}); err != nil {
		return fmt.Errorf("plugin-host: register rpc service: %w", err)
	}

	conn := ipc.NewDuplexConn(os.Stdin, os.Stdout)
	defer conn.Close()

	srv.ServeConn(conn)
	return nil
}

func hostCallbackClientFromEnv() (*rpc.Client, *ipc.DuplexConn, error) {
	readFDStr := os.Getenv(ipc.EnvHostRPCReadFD)
	writeFDStr := os.Getenv(ipc.EnvHostRPCWriteFD)
	if readFDStr == "" || writeFDStr == "" {
		return nil, nil, nil
	}

	readFD, err := strconv.Atoi(readFDStr)
	if err != nil || readFD < 0 {
		return nil, nil, fmt.Errorf("plugin-host: invalid %s=%q", ipc.EnvHostRPCReadFD, readFDStr)
	}
	writeFD, err := strconv.Atoi(writeFDStr)
	if err != nil || writeFD < 0 {
		return nil, nil, fmt.Errorf("plugin-host: invalid %s=%q", ipc.EnvHostRPCWriteFD, writeFDStr)
	}

	readFile := os.NewFile(uintptr(readFD), "openlobster-host-rpc-read")
	writeFile := os.NewFile(uintptr(writeFD), "openlobster-host-rpc-write")
	if readFile == nil || writeFile == nil {
		return nil, nil, fmt.Errorf("plugin-host: failed to open callback fd handles")
	}

	conn := ipc.NewDuplexConn(readFile, writeFile)
	return rpc.NewClient(conn), conn, nil
}
