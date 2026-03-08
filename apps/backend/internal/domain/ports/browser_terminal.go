package ports

import (
	"context"
)

type BrowserPort interface {
	NewPage(ctx context.Context) (BrowserPage, error)
	Close() error
}

type BrowserPage interface {
	Navigate(ctx context.Context, url string) error
	Screenshot(ctx context.Context) ([]byte, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	Eval(ctx context.Context, script string) (interface{}, error)
	WaitForSelector(ctx context.Context, selector string) error
	Close() error
}

type TerminalPort interface {
	Execute(ctx context.Context, cmd string, opts ...TerminalOption) (TerminalOutput, error)
	Spawn(ctx context.Context, cmd string) (PtySession, error)
	RunBackground(ctx context.Context, cmd string, opts ...TerminalOption) (BackgroundProcess, error)
	ListProcesses(ctx context.Context) ([]BackgroundProcess, error)
	KillProcess(ctx context.Context, pid int) error
}

type TerminalOption func(*TerminalOptions)

type TerminalOptions struct {
	Env        []string
	WorkingDir string
	Timeout    int
}

type TerminalOutput struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type PtySession interface {
	Write(data []byte) error
	Read() ([]byte, error)
	Resize(cols, rows int) error
	Close() error
}

type BackgroundProcess interface {
	PID() int
	ID() string
	Command() string
	Status() ProcessStatus
	Output() <-chan string
	Wait() (TerminalOutput, error)
	Kill() error
}

type ProcessStatus string

const (
	ProcessStatusRunning ProcessStatus = "running"
	ProcessStatusDone    ProcessStatus = "done"
	ProcessStatusFailed  ProcessStatus = "failed"
	ProcessStatusKilled  ProcessStatus = "killed"
)
