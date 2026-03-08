package terminal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/ports"
)

type HostAdapter struct {
	backgroundProcs map[string]*BackgroundProcessInfo
	mu              sync.Mutex
}

func NewHostAdapter() *HostAdapter {
	return &HostAdapter{
		backgroundProcs: make(map[string]*BackgroundProcessInfo),
	}
}

func (a *HostAdapter) Execute(ctx context.Context, cmd string, opts ...ports.TerminalOption) (ports.TerminalOutput, error) {
	options := &ports.TerminalOptions{}
	for _, opt := range opts {
		opt(options)
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ports.TerminalOutput{}, fmt.Errorf("empty command")
	}

	executable := parts[0]
	args := parts[1:]

	c := exec.CommandContext(ctx, executable, args...)
	c.Env = FilterOpenLobsterFromEnv(os.Environ())
	if options.WorkingDir != "" {
		c.Dir = options.WorkingDir
	}
	if len(options.Env) > 0 {
		// Filter user-provided env so OPENLOBSTER_* (e.g. OPENLOBSTER_SECRET_KEY)
		// can never be passed through even if the LLM requests it.
		c.Env = append(c.Env, FilterOpenLobsterFromEnv(options.Env)...)
	}

	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(options.Timeout)*time.Second)
		defer cancel()
	}

	out, err := c.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	return ports.TerminalOutput{
		Stdout:   string(out),
		Stderr:   "",
		ExitCode: exitCode,
	}, nil
}

func (a *HostAdapter) Spawn(ctx context.Context, cmd string) (ports.PtySession, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	proc := &BackgroundProcessInfo{
		id:        uuid.New().String(),
		output:    make(chan string, 100),
		done:      make(chan struct{}),
		startedAt: time.Now(),
	}

	proc.cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	proc.cmd.Env = FilterOpenLobsterFromEnv(os.Environ())

	err := proc.cmd.Start()
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.backgroundProcs[proc.id] = proc
	a.mu.Unlock()

	go func() {
		proc.cmd.Wait()
		proc.status = ports.ProcessStatusDone
		close(proc.done)
	}()

	return &PtySessionWrapper{proc: proc}, nil
}

func (a *HostAdapter) RunBackground(ctx context.Context, cmd string, opts ...ports.TerminalOption) (ports.BackgroundProcess, error) {
	options := &ports.TerminalOptions{}
	for _, opt := range opts {
		opt(options)
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	proc := &BackgroundProcessInfo{
		id:        uuid.New().String(),
		output:    make(chan string, 100),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		status:    ports.ProcessStatusRunning,
	}

	proc.cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	proc.cmd.Env = FilterOpenLobsterFromEnv(os.Environ())
	if options.WorkingDir != "" {
		proc.cmd.Dir = options.WorkingDir
	}

	err := proc.cmd.Start()
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.backgroundProcs[proc.id] = proc
	a.mu.Unlock()

	go func() {
		proc.cmd.Wait()
		proc.status = ports.ProcessStatusDone
		close(proc.done)
	}()

	return &BackgroundProcessWrapper{info: proc}, nil
}

func (a *HostAdapter) ListProcesses(ctx context.Context) ([]ports.BackgroundProcess, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make([]ports.BackgroundProcess, 0, len(a.backgroundProcs))
	for _, proc := range a.backgroundProcs {
		result = append(result, &BackgroundProcessWrapper{info: proc})
	}
	return result, nil
}

func (a *HostAdapter) KillProcess(ctx context.Context, pid int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, proc := range a.backgroundProcs {
		if proc.cmd.Process != nil && proc.cmd.Process.Pid == pid {
			return proc.cmd.Process.Kill()
		}
	}
	return nil
}

var _ ports.TerminalPort = (*HostAdapter)(nil)
