// Copyright (c) OpenLobster contributors. See LICENSE for details.

package terminal

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── NewHostAdapter ───────────────────────────────────────────────────────────

func TestNewHostAdapter(t *testing.T) {
	adapter := NewHostAdapter()
	require.NotNil(t, adapter)
	assert.NotNil(t, adapter.backgroundProcs)
}

// ─── BackgroundProcessInfo.appendOutput / getOutput ──────────────────────────

func TestBackgroundProcessInfo_AppendAndGetOutput(t *testing.T) {
	proc := &BackgroundProcessInfo{
		output: make(chan string, 16),
	}
	proc.appendOutput("line one")
	proc.appendOutput("line two")

	out := proc.getOutput()
	assert.Contains(t, out, "line one")
	assert.Contains(t, out, "line two")
}

func TestBackgroundProcessInfo_AppendOutput_ChannelFull(t *testing.T) {
	// When channel is full appendOutput must not block.
	proc := &BackgroundProcessInfo{
		output: make(chan string, 1),
	}
	// Fill the channel.
	proc.output <- "existing"
	// This must complete without blocking (select default branch).
	done := make(chan struct{})
	go func() {
		proc.appendOutput("overflow")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("appendOutput blocked on full channel")
	}
}

func TestBackgroundProcessInfo_GetOutput_Empty(t *testing.T) {
	proc := &BackgroundProcessInfo{
		output: make(chan string, 8),
	}
	assert.Equal(t, "", proc.getOutput())
}

// ─── BackgroundProcessWrapper ─────────────────────────────────────────────────

func TestBackgroundProcessWrapper_ID(t *testing.T) {
	proc := &BackgroundProcessInfo{id: "proc-1"}
	w := &BackgroundProcessWrapper{info: proc}
	assert.Equal(t, "proc-1", w.ID())
}

func TestBackgroundProcessWrapper_PID_NoProcess(t *testing.T) {
	proc := &BackgroundProcessInfo{}
	w := &BackgroundProcessWrapper{info: proc}
	assert.Equal(t, 0, w.PID())
}

func TestBackgroundProcessWrapper_Command_FromField(t *testing.T) {
	proc := &BackgroundProcessInfo{command: "echo hello"}
	w := &BackgroundProcessWrapper{info: proc}
	assert.Equal(t, "echo hello", w.Command())
}

func TestBackgroundProcessWrapper_Command_FallbackToNil(t *testing.T) {
	proc := &BackgroundProcessInfo{command: ""}
	w := &BackgroundProcessWrapper{info: proc}
	// cmd is nil so returns "".
	assert.Equal(t, "", w.Command())
}

func TestBackgroundProcessWrapper_Status_Running(t *testing.T) {
	proc := &BackgroundProcessInfo{
		done:   make(chan struct{}),
		status: ports.ProcessStatusRunning,
	}
	w := &BackgroundProcessWrapper{info: proc}
	// Channel is not closed so status should be Running.
	assert.Equal(t, ports.ProcessStatusRunning, w.Status())
}

func TestBackgroundProcessWrapper_Status_Done(t *testing.T) {
	proc := &BackgroundProcessInfo{
		done:   make(chan struct{}),
		status: ports.ProcessStatusDone,
	}
	close(proc.done)
	w := &BackgroundProcessWrapper{info: proc}
	assert.Equal(t, ports.ProcessStatusDone, w.Status())
}

func TestBackgroundProcessWrapper_Output(t *testing.T) {
	proc := &BackgroundProcessInfo{
		output: make(chan string, 8),
	}
	proc.output <- "test line"
	w := &BackgroundProcessWrapper{info: proc}
	ch := w.Output()
	require.NotNil(t, ch)
	line := <-ch
	assert.Equal(t, "test line", line)
}

func TestBackgroundProcessWrapper_CollectedOutput(t *testing.T) {
	proc := &BackgroundProcessInfo{
		output:      make(chan string, 8),
		outputLines: []string{"line1", "line2"},
	}
	w := &BackgroundProcessWrapper{info: proc}
	out := w.CollectedOutput()
	assert.Contains(t, out, "line1")
	assert.Contains(t, out, "line2")
}

func TestBackgroundProcessWrapper_Wait(t *testing.T) {
	proc := &BackgroundProcessInfo{
		done:        make(chan struct{}),
		outputLines: []string{"result"},
		exitCode:    0,
	}
	close(proc.done)
	w := &BackgroundProcessWrapper{info: proc}
	out, err := w.Wait()
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "result")
	assert.Equal(t, 0, out.ExitCode)
}

func TestBackgroundProcessWrapper_Kill_NilCmd(t *testing.T) {
	proc := &BackgroundProcessInfo{}
	w := &BackgroundProcessWrapper{info: proc}
	err := w.Kill()
	assert.NoError(t, err)
}

// ─── PtySessionWrapper ────────────────────────────────────────────────────────

func TestPtySessionWrapper_Write(t *testing.T) {
	proc := &BackgroundProcessInfo{}
	s := &PtySessionWrapper{proc: proc}
	err := s.Write([]byte("input"))
	assert.NoError(t, err)
}

func TestPtySessionWrapper_Read(t *testing.T) {
	proc := &BackgroundProcessInfo{}
	s := &PtySessionWrapper{proc: proc}
	data, err := s.Read()
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestPtySessionWrapper_Resize(t *testing.T) {
	proc := &BackgroundProcessInfo{}
	s := &PtySessionWrapper{proc: proc}
	err := s.Resize(80, 24)
	assert.NoError(t, err)
}

func TestPtySessionWrapper_Close_NilCmd(t *testing.T) {
	proc := &BackgroundProcessInfo{}
	s := &PtySessionWrapper{proc: proc}
	err := s.Close()
	assert.NoError(t, err)
}

// ─── HostAdapter.Execute ──────────────────────────────────────────────────────

func TestHostAdapter_Execute_EmptyCommand(t *testing.T) {
	adapter := NewHostAdapter()
	_, err := adapter.Execute(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty command")
}

func TestHostAdapter_Execute_SimpleCommand(t *testing.T) {
	adapter := NewHostAdapter()
	out, err := adapter.Execute(context.Background(), "echo hello")
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "hello")
	assert.Equal(t, 0, out.ExitCode)
}

func TestHostAdapter_Execute_WithTimeout(t *testing.T) {
	adapter := NewHostAdapter()
	// Use a timeout of 5 seconds; "echo" finishes immediately.
	out, err := adapter.Execute(context.Background(), "echo hi", func(o *ports.TerminalOptions) {
		o.Timeout = 5
	})
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "hi")
}

func TestHostAdapter_Execute_WithWorkingDir(t *testing.T) {
	adapter := NewHostAdapter()
	// Pass WorkingDir and verify the option is applied without error.
	out, err := adapter.Execute(context.Background(), "echo workdir_ok", func(o *ports.TerminalOptions) {
		o.WorkingDir = "/tmp"
	})
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "workdir_ok")
}

func TestHostAdapter_Execute_WithEnv(t *testing.T) {
	adapter := NewHostAdapter()
	// Provide extra env — the command itself just confirms execution succeeded.
	out, err := adapter.Execute(context.Background(), "echo env_ok", func(o *ports.TerminalOptions) {
		o.Env = []string{"MY_TEST_VAR=hello_from_env"}
	})
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "env_ok")
}

func TestHostAdapter_Execute_WithOpenLobsterEnvFiltered(t *testing.T) {
	adapter := NewHostAdapter()
	// Verify that OPENLOBSTER_ env supplied by user options does not propagate.
	// We just confirm the command executes cleanly; the filter is unit-tested in env_test.go.
	out, err := adapter.Execute(context.Background(), "echo filtered", func(o *ports.TerminalOptions) {
		o.Env = []string{"OPENLOBSTER_SECRET_KEY=should_not_appear"}
	})
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "filtered")
	assert.NotContains(t, out.Stdout, "should_not_appear")
}

func TestHostAdapter_Execute_FailingCommand(t *testing.T) {
	adapter := NewHostAdapter()
	// Use a command that is guaranteed to fail: reference a nonexistent path.
	out, err := adapter.Execute(context.Background(), "/bin/sh -c 'exit 1'")
	// Execute does not propagate non-zero exit as Go error.
	require.NoError(t, err)
	assert.NotEqual(t, 0, out.ExitCode)
}

// ─── HostAdapter.ListProcesses / GetProcess / KillProcess ────────────────────

func TestHostAdapter_ListProcesses_Empty(t *testing.T) {
	adapter := NewHostAdapter()
	list, err := adapter.ListProcesses(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestHostAdapter_GetProcess_NotFound(t *testing.T) {
	adapter := NewHostAdapter()
	_, err := adapter.GetProcess(context.Background(), "nonexistent-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHostAdapter_KillProcess_NotFound(t *testing.T) {
	adapter := NewHostAdapter()
	err := adapter.KillProcess(context.Background(), 9999999)
	assert.NoError(t, err)
}

// ─── HostAdapter.RunBackground ────────────────────────────────────────────────

func TestHostAdapter_RunBackground_EmptyCommand(t *testing.T) {
	adapter := NewHostAdapter()
	_, err := adapter.RunBackground(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty command")
}

func TestHostAdapter_RunBackground_SimpleCommand(t *testing.T) {
	adapter := NewHostAdapter()
	ctx := context.Background()
	proc, err := adapter.RunBackground(ctx, "/bin/sh -c 'echo bg_test'")
	require.NoError(t, err)
	require.NotNil(t, proc)
	assert.NotEmpty(t, proc.ID())

	// Wait for the process to finish and collect output.
	out, err := proc.Wait()
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "bg_test")
}

func TestHostAdapter_RunBackground_WithWorkingDir(t *testing.T) {
	adapter := NewHostAdapter()
	proc, err := adapter.RunBackground(context.Background(), "/bin/sh -c pwd", func(o *ports.TerminalOptions) {
		o.WorkingDir = "/tmp"
	})
	require.NoError(t, err)
	out, err := proc.Wait()
	require.NoError(t, err)
	assert.Contains(t, out.Stdout, "tmp")
}

func TestHostAdapter_ListProcesses_AfterRun(t *testing.T) {
	adapter := NewHostAdapter()
	ctx := context.Background()
	proc, err := adapter.RunBackground(ctx, "/bin/sh -c 'sleep 10'")
	require.NoError(t, err)

	list, err := adapter.ListProcesses(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)

	// Clean up.
	_ = proc.Kill()
}

func TestHostAdapter_GetProcess_Found(t *testing.T) {
	adapter := NewHostAdapter()
	ctx := context.Background()
	proc, err := adapter.RunBackground(ctx, "/bin/sh -c 'sleep 10'")
	require.NoError(t, err)

	found, err := adapter.GetProcess(ctx, proc.ID())
	require.NoError(t, err)
	assert.Equal(t, proc.ID(), found.ID())

	_ = proc.Kill()
}

// ─── HostAdapter.Spawn ────────────────────────────────────────────────────────

func TestHostAdapter_Spawn_EmptyCommand(t *testing.T) {
	adapter := NewHostAdapter()
	_, err := adapter.Spawn(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty command")
}

func TestHostAdapter_Spawn_SimpleCommand(t *testing.T) {
	adapter := NewHostAdapter()
	session, err := adapter.Spawn(context.Background(), "/bin/sh -c 'echo spawn_test'")
	require.NoError(t, err)
	require.NotNil(t, session)
	// Close the session.
	_ = session.Close()
}

// ─── Concurrent safety ────────────────────────────────────────────────────────

func TestHostAdapter_ConcurrentListAndRun(t *testing.T) {
	adapter := NewHostAdapter()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = adapter.ListProcesses(ctx)
		}()
	}
	wg.Wait()
}
