package terminal

import (
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

type BackgroundProcessInfo struct {
	id          string
	command     string // full original command string
	cmd         *exec.Cmd
	startedAt   time.Time
	output      chan string
	done        chan struct{}
	status      ports.ProcessStatus
	exitCode    int
	mu          sync.Mutex
	outputLines []string // accumulated stdout+stderr lines
}

func (p *BackgroundProcessInfo) appendOutput(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.outputLines = append(p.outputLines, line)
	select {
	case p.output <- line:
	default:
	}
}

func (p *BackgroundProcessInfo) getOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return strings.Join(p.outputLines, "\n")
}

type BackgroundProcessWrapper struct {
	info *BackgroundProcessInfo
}

func (p *BackgroundProcessWrapper) PID() int {
	if p.info.cmd != nil && p.info.cmd.Process != nil {
		return p.info.cmd.Process.Pid
	}
	return 0
}

func (p *BackgroundProcessWrapper) ID() string {
	return p.info.id
}

func (p *BackgroundProcessWrapper) Command() string {
	if p.info.command != "" {
		return p.info.command
	}
	if p.info.cmd != nil {
		return strings.Join(p.info.cmd.Args, " ")
	}
	return ""
}

func (p *BackgroundProcessWrapper) Status() ports.ProcessStatus {
	select {
	case <-p.info.done:
		return ports.ProcessStatusDone
	default:
		return ports.ProcessStatusRunning
	}
}

func (p *BackgroundProcessWrapper) Output() <-chan string {
	return p.info.output
}

func (p *BackgroundProcessWrapper) CollectedOutput() string {
	return p.info.getOutput()
}

func (p *BackgroundProcessWrapper) Wait() (ports.TerminalOutput, error) {
	<-p.info.done
	return ports.TerminalOutput{
		Stdout:   p.info.getOutput(),
		ExitCode: p.info.exitCode,
	}, nil
}

func (p *BackgroundProcessWrapper) Kill() error {
	if p.info.cmd != nil && p.info.cmd.Process != nil {
		return p.info.cmd.Process.Kill()
	}
	return nil
}

type PtySessionWrapper struct {
	proc *BackgroundProcessInfo
}

func (p *PtySessionWrapper) Write(data []byte) error {
	return nil
}

func (p *PtySessionWrapper) Read() ([]byte, error) {
	return nil, nil
}

func (p *PtySessionWrapper) Resize(cols, rows int) error {
	return nil
}

func (p *PtySessionWrapper) Close() error {
	if p.proc.cmd != nil && p.proc.cmd.Process != nil {
		return p.proc.cmd.Process.Kill()
	}
	return nil
}
