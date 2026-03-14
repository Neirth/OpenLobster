package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
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
		command:   cmd,
		output:    make(chan string, 256),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		status:    ports.ProcessStatusRunning,
	}

	proc.cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	proc.cmd.Env = FilterOpenLobsterFromEnv(os.Environ())

	stdoutPipe, err := proc.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := proc.cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := proc.cmd.Start(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.backgroundProcs[proc.id] = proc
	a.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go streamToProc(stdoutPipe, proc, &wg)
	go streamToProc(stderrPipe, proc, &wg)

	go func() {
		wg.Wait()
		waitErr := proc.cmd.Wait()
		proc.mu.Lock()
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				proc.exitCode = exitErr.ExitCode()
				proc.status = ports.ProcessStatusFailed
			} else {
				proc.status = ports.ProcessStatusFailed
			}
		} else {
			proc.status = ports.ProcessStatusDone
		}
		proc.mu.Unlock()
		close(proc.done)
	}()

	return &PtySessionWrapper{proc: proc}, nil
}

// streamToProc reads lines from r and appends them to proc's output buffer.
func streamToProc(r io.Reader, proc *BackgroundProcessInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		proc.appendOutput(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("terminal: streamToProc scanner error (cmd=%q): %v", proc.command, err)
		proc.appendOutput("[stream error: " + err.Error() + "]")
	}
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
		command:   cmd,
		output:    make(chan string, 256),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		status:    ports.ProcessStatusRunning,
	}

	proc.cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	proc.cmd.Env = FilterOpenLobsterFromEnv(os.Environ())
	if options.WorkingDir != "" {
		proc.cmd.Dir = options.WorkingDir
	}

	stdoutPipe, err := proc.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := proc.cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := proc.cmd.Start(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.backgroundProcs[proc.id] = proc
	a.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go streamToProc(stdoutPipe, proc, &wg)
	go streamToProc(stderrPipe, proc, &wg)

	go func() {
		wg.Wait()
		waitErr := proc.cmd.Wait()
		proc.mu.Lock()
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				proc.exitCode = exitErr.ExitCode()
				proc.status = ports.ProcessStatusFailed
			} else {
				proc.status = ports.ProcessStatusFailed
			}
		} else {
			proc.status = ports.ProcessStatusDone
		}
		proc.mu.Unlock()
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

func (a *HostAdapter) GetProcess(ctx context.Context, id string) (ports.BackgroundProcess, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	proc, ok := a.backgroundProcs[id]
	if !ok {
		return nil, fmt.Errorf("process %q not found", id)
	}
	return &BackgroundProcessWrapper{info: proc}, nil
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
