package ipc

import (
	"context"
	"fmt"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type hostRPC struct {
	onMessage func([]byte)
}

func (h *hostRPC) EmitMessage(args EmitMessageArgs, _ *EmptyReply) error {
	if h == nil || h.onMessage == nil || len(args.Payload) == 0 {
		return nil
	}
	h.onMessage(args.Payload)
	return nil
}

// Client owns one plugin helper process and the two RPC channels:
// - host->helper plugin calls over stdio
// - helper->host callbacks over extra pipe files
type Client struct {
	cmd        *exec.Cmd
	rpcClient  *rpc.Client
	hostConn   *DuplexConn
	pluginConn *DuplexConn

	closeOnce sync.Once
}

// StartClient starts a helper process and initializes RPC channels.
func StartClient(helperPath, processName string, args []string, onMessage func([]byte)) (*Client, error) {
	launchPath := helperPath
	if processName != "" {
		launchPath = prepareNamedExecutable(helperPath, processName)
	}

	cmd := exec.Command(launchPath, args...)
	if processName != "" {
		cmd.Args[0] = processName
	}
	env := append([]string{}, os.Environ()...)
	if os.Getenv("HOME") == "" {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			home = os.TempDir()
		}
		env = append(env, "HOME="+home)
	}
	cmd.Env = append(env,
		fmt.Sprintf("%s=%d", EnvHostRPCReadFD, 3),
		fmt.Sprintf("%s=%d", EnvHostRPCWriteFD, 4),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ipc: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ipc: stdout pipe: %w", err)
	}

	// Callback channel pair for helper->host RPC.
	childRead, hostWrite, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("ipc: host->helper callback pipe: %w", err)
	}
	hostRead, childWrite, err := os.Pipe()
	if err != nil {
		_ = childRead.Close()
		_ = hostWrite.Close()
		return nil, fmt.Errorf("ipc: helper->host callback pipe: %w", err)
	}

	cmd.ExtraFiles = []*os.File{childRead, childWrite}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = childRead.Close()
		_ = hostWrite.Close()
		_ = hostRead.Close()
		_ = childWrite.Close()
		return nil, fmt.Errorf("ipc: start helper %s: %w", helperPath, err)
	}

	// Parent keeps hostRead/hostWrite and closes child ends after start.
	_ = childRead.Close()
	_ = childWrite.Close()

	hostConn := NewDuplexConn(hostRead, hostWrite)
	hostServer := rpc.NewServer()
	if err := hostServer.RegisterName("Host", &hostRPC{onMessage: onMessage}); err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("ipc: register host callback service: %w", err)
	}
	go hostServer.ServeConn(hostConn)

	pluginConn := NewDuplexConn(stdout, stdin)
	rpcClient := rpc.NewClient(pluginConn)

	return &Client{
		cmd:        cmd,
		rpcClient:  rpcClient,
		hostConn:   hostConn,
		pluginConn: pluginConn,
	}, nil
}

func prepareNamedExecutable(helperPath, processName string) string {
	if runtime.GOOS != "darwin" {
		return helperPath
	}

	if processName == "" {
		return helperPath
	}

	safeName := sanitizeProcessName(processName)
	if safeName == "" {
		return helperPath
	}

	helperInfo, err := os.Stat(helperPath)
	if err != nil {
		return helperPath
	}

	baseDir := filepath.Dir(helperPath)
	namedDir := filepath.Join(baseDir, ".plugin-host-bin")
	if err := os.MkdirAll(namedDir, 0o755); err != nil {
		return helperPath
	}

	targetPath := filepath.Join(namedDir, safeName)
	if targetInfo, err := os.Stat(targetPath); err == nil {
		if os.SameFile(helperInfo, targetInfo) {
			return targetPath
		}
		_ = os.Remove(targetPath)
	} else if !os.IsNotExist(err) {
		return helperPath
	}

	tmpPath := fmt.Sprintf("%s.%d.tmp", targetPath, os.Getpid())
	_ = os.Remove(tmpPath)
	if err := os.Link(helperPath, tmpPath); err == nil {
		if renameErr := os.Rename(tmpPath, targetPath); renameErr == nil {
			return targetPath
		}
		_ = os.Remove(tmpPath)
	}

	if err := os.Link(helperPath, targetPath); err == nil {
		return targetPath
	}
	return helperPath
}

func sanitizeProcessName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

// Call executes one RPC method with context timeout/cancel handling.
func (c *Client) Call(ctx context.Context, method string, args any, reply any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan *rpc.Call, 1)
	c.rpcClient.Go(method, args, reply, done)

	select {
	case call := <-done:
		return call.Error
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close terminates RPC channels and helper process.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	var closeErr error
	c.closeOnce.Do(func() {
		if c.rpcClient != nil {
			_ = c.rpcClient.Close()
		}
		if c.pluginConn != nil {
			_ = c.pluginConn.Close()
		}
		if c.hostConn != nil {
			_ = c.hostConn.Close()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			if err := c.cmd.Wait(); err != nil {
				msg := strings.TrimSpace(err.Error())
				if msg != "" && !strings.Contains(msg, "signal: killed") {
					closeErr = err
				}
			}
		}
	})
	return closeErr
}
