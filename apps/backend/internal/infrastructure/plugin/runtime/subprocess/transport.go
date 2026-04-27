// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// jrpcConn is a JSON-RPC 2.0 client over a plugin's stdin/stdout pipes.
// Requests are written to stdin; responses and notifications are read from
// stdout. Both sides may initiate requests at any time; all IDs are UUID v4
// strings. All methods are safe for concurrent use.
type jrpcConn struct {
	writer  *rpcWriter
	pending sync.Map // string (UUID) → chan *jrpcResult
}

type jrpcResult struct {
	result json.RawMessage
	err    *rpcError
}

func newJRPCConn(stdin io.Writer) *jrpcConn {
	return &jrpcConn{writer: newRPCWriter(stdin)}
}

func (c *jrpcConn) call(ctx context.Context, method string, params any, out any) error {
	id, err := c.writer.call(method, params)
	if err != nil {
		return fmt.Errorf("jsonrpc write %s: %w", method, err)
	}

	ch := make(chan *jrpcResult, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return errors.New(res.err.Message)
		}
		if out != nil && len(res.result) > 0 {
			return json.Unmarshal(res.result, out)
		}
		return nil
	}
}

// dispatch routes an incoming response to the call() future that is awaiting it.
func (c *jrpcConn) dispatch(msg *rpcMessage) {
	if msg.ID == nil {
		return
	}
	val, ok := c.pending.LoadAndDelete(*msg.ID)
	if !ok {
		return
	}
	ch := val.(chan *jrpcResult)
	ch <- &jrpcResult{result: msg.Result, err: msg.Error}
}

// respond writes a JSON-RPC 2.0 response for a plugin-initiated request back
// to the plugin's stdin.
func (c *jrpcConn) respond(id string, result json.RawMessage, rpcErr *rpcError) error {
	return c.writer.respond(id, result, rpcErr)
}

// ---------------------------------------------------------------------------
// readLoop
// ---------------------------------------------------------------------------

// readLoop reads stdout of the plugin process and classifies each message:
//
//   - Response (has id, no method): routed to the call() future via dispatch.
//   - Plugin-initiated request (has method + id): dispatched to onPluginRequest
//     in a new goroutine so the read loop is never blocked.
//   - Notification (has method, no id): dispatched to onNotification inline.
//
// onNotification and onPluginRequest may be nil (messages are silently dropped).
func readLoop(
	stdout io.Reader,
	conn *jrpcConn,
	onNotification func(method string, params json.RawMessage),
	onPluginRequest func(id string, method string, params json.RawMessage),
) {
	scanner := newRPCScanner(stdout)
	for {
		msg, err := scanner.scan()
		if err != nil {
			return
		}
		if msg == nil {
			return
		}

		if msg.Method != "" {
			if msg.ID != nil {
				// Plugin-initiated request — the plugin expects a response.
				if onPluginRequest != nil {
					id := *msg.ID
					params := append(json.RawMessage(nil), msg.Params...)
					go onPluginRequest(id, msg.Method, params)
				}
			} else {
				// Fire-and-forget notification.
				if onNotification != nil {
					onNotification(msg.Method, msg.Params)
				}
			}
			continue
		}

		// No method field: response to a host-initiated call.
		conn.dispatch(msg)
	}
}

// ---------------------------------------------------------------------------
// Schema normalisation helper
// ---------------------------------------------------------------------------

func normalizeMetadataSchema(raw json.RawMessage) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return []byte("{}")
	}
	if trimmed[0] == '"' {
		var schemaString string
		if err := json.Unmarshal(trimmed, &schemaString); err != nil {
			return []byte("{}")
		}
		trimmed = bytes.TrimSpace([]byte(schemaString))
	}
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return []byte("{}")
	}
	return append([]byte(nil), trimmed...)
}

// ---------------------------------------------------------------------------
// Adapter lifecycle
// ---------------------------------------------------------------------------

func (a *Adapter) startLocked(ctx context.Context) error {
	cmd := exec.Command(a.binaryPath)
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("plugin %s: stderr pipe: %w", a.id, err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("plugin %s: stdin pipe: %w", a.id, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("plugin %s: stdout pipe: %w", a.id, err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return fmt.Errorf("plugin %s: start %s: %w", a.id, a.binaryPath, err)
	}

	// Capture stderr in a separate goroutine to avoid blocking the plugin
	go func() {
		scanner := newRPCScanner(stderr)
		for {
			line, err := scanner.scanLine()
			if err != nil {
				break
			}
			if line = strings.TrimSpace(line); line != "" {
				log.Printf("plugin %s:stderr: %s", a.id, line)
			}
		}
	}()

	conn := newJRPCConn(stdin)

	// onNotification handles fire-and-forget messages from the plugin
	// (no id — no response expected).
	onNotification := func(method string, params json.RawMessage) {
		switch method {
		case methodEmitMessage:
			if a.onMessage == nil || len(params) == 0 {
				return
			}
			var p struct {
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(params, &p) == nil && len(p.Payload) > 0 {
				payload := append([]byte(nil), p.Payload...)
				go a.onMessage(payload)
			}

		case methodEmitLog:
			if len(params) == 0 {
				return
			}
			var p struct {
				Level   string `json:"level"`
				Message string `json:"message"`
			}
			if json.Unmarshal(params, &p) == nil {
				level := strings.TrimSpace(strings.ToLower(p.Level))
				if level == "" {
					level = "info"
				}
				log.Printf("plugin %s [%s]: %s", a.id, level, strings.TrimSpace(p.Message))
			}
		}
	}

	// onPluginRequest handles plugin-initiated RPC calls (has id — must respond).
	// No core handlers are registered yet; return method-not-found for all calls.
	// Future: wire up a service registry so plugins can call vault.get etc.
	onPluginRequest := func(id string, method string, _ json.RawMessage) {
		_ = conn.respond(id, nil, &rpcError{
			Code:    -32601,
			Message: fmt.Sprintf("method not found: %s", method),
		})
	}

	eventCtx, eventCancel := context.WithCancel(context.Background())
	go func() {
		readLoop(stdout, conn, onNotification, onPluginRequest)
		eventCancel()
	}()

	infoCtx, cancelInfo := context.WithTimeout(ctxOrBackground(ctx), handshakeTimeout)
	defer cancelInfo()

	var info getInfoResponse
	if err := conn.call(infoCtx, methodGetInfo, &getInfoRequest{}, &info); err != nil {
		eventCancel()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("plugin %s: get_info: %w", a.id, err)
	}

	id := strings.TrimSpace(info.ID)
	if id == "" {
		id = a.id
	}
	if id == "" {
		id = moduleStem(a.binaryPath)
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = id
	}

	a.cmd = cmd
	a.stdin = stdin
	a.stdout = stdout
	a.conn = conn
	a.eventCancel = eventCancel
	a.id = id
	a.name = name
	a.version = strings.TrimSpace(info.Version)
	a.description = strings.TrimSpace(info.Description)
	a.pluginType = strings.TrimSpace(info.Type)
	a.schemaJSON = normalizeMetadataSchema(json.RawMessage(info.Schema))
	a.properties = append(json.RawMessage(nil), info.Properties...)
	a.exports = make(map[string]struct{}, len(info.Exports))
	for _, fn := range info.Exports {
		if fn = strings.TrimSpace(fn); fn != "" {
			a.exports[fn] = struct{}{}
		}
	}

	a.stateMu.Lock()
	a.available = true
	a.lastError = nil
	a.stateMu.Unlock()

	go a.monitorProcess(cmd, eventCtx)

	return nil
}

func (a *Adapter) stopLocked() error {
	if a.eventCancel != nil {
		a.eventCancel()
		a.eventCancel = nil
	}

	var closeErr error
	if a.conn != nil {
		// Attempt clean RPC close only if the process is still known to be running
		isAlive := a.cmd != nil && a.cmd.Process != nil
		if isAlive && a.cmd.ProcessState == nil {
			closeCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			err := a.conn.call(closeCtx, methodClose, &closeRequest{}, nil)
			cancel()
			if err != nil && !isSafeCloseError(err) {
				closeErr = err
			}
		}
		a.conn = nil
	}

	if a.stdin != nil {
		if err := a.stdin.Close(); err != nil && closeErr == nil && !isSafeCloseError(err) {
			closeErr = err
		}
		a.stdin = nil
	}
	if a.stdout != nil {
		if err := a.stdout.Close(); err != nil && closeErr == nil && !isSafeCloseError(err) {
			closeErr = err
		}
		a.stdout = nil
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_, _ = a.cmd.Process.Wait()
	}
	a.cmd = nil

	return closeErr
}

func isSafeCloseError(err error) bool {
	if err == nil {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "closed pipe") ||
		strings.Contains(s, "file already closed") ||
		strings.Contains(s, "os: process already finished") ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func (a *Adapter) monitorProcess(cmd *exec.Cmd, eventCtx context.Context) {
	err := cmd.Wait()
	// Only update state if the event loop is still running (not a clean stop).
	select {
	case <-eventCtx.Done():
		return
	default:
	}
	a.stateMu.Lock()
	a.available = false
	if err != nil {
		a.lastError = fmt.Errorf("plugin %s exited: %w", a.id, err)
	}
	a.stateMu.Unlock()
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
