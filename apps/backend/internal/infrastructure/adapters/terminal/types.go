package terminal

import (
	"os/exec"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
)

type BackgroundProcessInfo struct {
	id        string
	cmd       *exec.Cmd
	startedAt time.Time
	output    chan string
	done      chan struct{}
	status    ports.ProcessStatus
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
	if p.info.cmd != nil {
		return p.info.cmd.Args[0]
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

func (p *BackgroundProcessWrapper) Wait() (ports.TerminalOutput, error) {
	<-p.info.done
	return ports.TerminalOutput{
		ExitCode: 0,
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
