// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package protocol implements the JSON-RPC 2.0 / STDIO bidirectional client
// used to communicate with OpenLobster plugin binaries during validation.
package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PluginClient is the interface consumed by smoke test runners.
type PluginClient interface {
	HasFunction(function string) bool
	CallJSON(function string, payload any) (json.RawMessage, error)
	CallString(function string, payload any) (string, error)
	Info() PluginInfo
	Close() error
	SetVictoryCode(code string)
	WaitForVictory(timeout time.Duration) bool
}

// PluginInfo holds the data returned by get_info.
type PluginInfo struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Version     string          `json:"version,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	Exports     []string        `json:"exports,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *string         `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jsonRPCCallResponse struct {
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// runtimePluginClient
// ---------------------------------------------------------------------------

type runtimePluginClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	info    PluginInfo
	exports map[string]struct{}

	reqMu   sync.Mutex
	writeMu sync.Mutex

	notifMu       sync.Mutex
	notifications []struct {
		Method string
		Params json.RawMessage
	}

	victoryMu        sync.Mutex
	targetVCode      string
	foundVictoryCode bool
	victoryChan      chan bool
	pendingMu        sync.Mutex
	pending          map[string]chan *scanResult
	stopChan         chan struct{}
}

type scanResult struct {
	resp jsonRPCResponse
	err  error
}

// StartRuntimePlugin launches the plugin binary and performs the initial
// GetInfo handshake. Returns a PluginClient ready for use.
func StartRuntimePlugin(binaryPath string) (PluginClient, error) {
	cmd := exec.Command(binaryPath)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start plugin process: %w", err)
	}

	c := &runtimePluginClient{
		cmd:         cmd,
		stdin:       stdin,
		scanner:     bufio.NewScanner(stdout),
		exports:     make(map[string]struct{}),
		pending:     make(map[string]chan *scanResult),
		stopChan:    make(chan struct{}),
		victoryChan: make(chan bool, 1),
	}
	c.scanner.Buffer(make([]byte, 1<<20), 1<<20)

	// Start dispatcher
	go c.dispatchLoop()

	// Initial handshake to get info
	info, err := c.getInfo()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("get_info failed: %w", err)
	}
	c.info = info

	exports := map[string]struct{}{}
	for _, name := range info.Exports {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			exports[trimmed] = struct{}{}
		}
	}
	c.exports = exports

	return c, nil
}

func (c *runtimePluginClient) Info() PluginInfo { return c.info }

func (c *runtimePluginClient) HasFunction(function string) bool {
	_, ok := c.exports[strings.TrimSpace(function)]
	return ok
}

func (c *runtimePluginClient) SetVictoryCode(code string) {
	c.victoryMu.Lock()
	defer c.victoryMu.Unlock()
	c.targetVCode = code
	c.foundVictoryCode = false
}

func (c *runtimePluginClient) WaitForVictory(timeout time.Duration) bool {
	select {
	case <-c.victoryChan:
		return true
	case <-time.After(timeout):
		c.victoryMu.Lock()
		defer c.victoryMu.Unlock()
		return c.foundVictoryCode
	}
}

func (c *runtimePluginClient) dispatchLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
			if !c.scanner.Scan() {
				scanErr := c.scanner.Err()
				if scanErr == nil {
					scanErr = io.EOF
				}
				// Notify all pending of the error
				c.pendingMu.Lock()
				for id, ch := range c.pending {
					ch <- &scanResult{err: fmt.Errorf("read response: %w", scanErr)}
					delete(c.pending, id)
				}
				c.pendingMu.Unlock()
				return
			}
			line := strings.TrimSpace(c.scanner.Text())
			if line == "" {
				continue
			}
			var resp jsonRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				continue
			}

			// Plugin-initiated request: reply with method-not-found.
			if resp.Method != "" && resp.ID != nil {
				errResp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      resp.ID,
					Error:   &jsonRPCError{Code: -32601, Message: "method not found: " + resp.Method},
				}
				data, _ := json.Marshal(errResp)
				data = append(data, '\n')
				c.writeMu.Lock()
				_, _ = c.stdin.Write(data)
				c.writeMu.Unlock()
				continue
			}

			// Notification (emit_log, emit_message, etc.)
			if resp.ID == nil {
				c.handleNotification(resp)
				continue
			}

			// Match pending request
			id := *resp.ID
			c.pendingMu.Lock()
			if ch, ok := c.pending[id]; ok {
				ch <- &scanResult{resp: resp}
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
		}
	}
}

func (c *runtimePluginClient) handleNotification(resp jsonRPCResponse) {
	if resp.Method == "emit_log" {
		var logParams struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(resp.Params, &logParams)
		fmt.Printf("[%s] %s\n", strings.ToUpper(logParams.Level), logParams.Message)
	}

	if resp.Method == "emit_message" {
		var p struct {
			Payload struct {
				Content string `json:"content"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(resp.Params, &p)
		if p.Payload.Content != "" {
			fmt.Printf("[MESSAGE] %s\n", p.Payload.Content)

			c.victoryMu.Lock()
			if c.targetVCode != "" && strings.Contains(strings.ToUpper(p.Payload.Content), strings.ToUpper(c.targetVCode)) {
				if !c.foundVictoryCode {
					c.foundVictoryCode = true
					select {
					case c.victoryChan <- true:
					default:
					}
				}
			}
			c.victoryMu.Unlock()
		}
	}

	c.notifMu.Lock()
	c.notifications = append(c.notifications, struct {
		Method string
		Params json.RawMessage
	}{Method: resp.Method, Params: append(json.RawMessage(nil), resp.Params...)})
	c.notifMu.Unlock()
}

func (c *runtimePluginClient) sendRequest(method string, params interface{}, timeout time.Duration) (json.RawMessage, error) {
	reqID := uuid.NewString()
	req := jsonRPCRequest{JSONRPC: "2.0", ID: reqID, Method: method, Params: params}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	ch := make(chan *scanResult, 1)
	c.pendingMu.Lock()
	c.pending[reqID] = ch
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	_, writeErr := c.stdin.Write(reqBytes)
	c.writeMu.Unlock()

	if writeErr != nil {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write request: %w", writeErr)
	}

	select {
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		if result.resp.Error != nil {
			return nil, fmt.Errorf("rpc error [%d]: %s", result.resp.Error.Code, result.resp.Error.Message)
		}
		return result.resp.Result, nil
	case <-time.After(timeout):
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response to %s (id=%s)", method, reqID)
	}
}

func (c *runtimePluginClient) getInfo() (PluginInfo, error) {
	raw, err := c.sendRequest("get_info", nil, 10*time.Second)
	if err != nil {
		return PluginInfo{}, err
	}
	var info PluginInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return PluginInfo{}, fmt.Errorf("unmarshal get_info response: %w", err)
	}
	return info, nil
}

func (c *runtimePluginClient) CallJSON(function string, payload any) (json.RawMessage, error) {
	var input json.RawMessage
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		input = raw
	}

	callTimeout := 12 * time.Second
	switch function {
	case "chat", "tts", "stt", "send":
		callTimeout = 10 * time.Minute
	}

	raw, err := c.sendRequest(function, input, callTimeout)
	if err != nil {
		return nil, err
	}

	// In the flat protocol, the result IS the output, or a structure containing output/error
	// We'll try to unmarshal as jsonRPCCallResponse for backward compatibility with plugins
	// that still return the legacy structure, otherwise return raw result.
	var resp jsonRPCCallResponse
	if err := json.Unmarshal(raw, &resp); err == nil && (len(resp.Output) > 0 || resp.Error != "") {
		if strings.TrimSpace(resp.Error) != "" {
			return nil, fmt.Errorf("%s", strings.TrimSpace(resp.Error))
		}
		return append([]byte(nil), resp.Output...), nil
	}

	return raw, nil
}

func (c *runtimePluginClient) CallString(function string, payload any) (string, error) {
	out, err := c.CallJSON(function, payload)
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(out, &s); err == nil {
		return strings.TrimSpace(s), nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *runtimePluginClient) Close() error {
	if c == nil {
		return nil
	}
	close(c.stopChan)
	if c.stdin != nil {
		_, _ = c.sendRequest("close", nil, 3*time.Second)
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}
