// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subprocess

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Wire message
// ---------------------------------------------------------------------------

// rpcMessage is a JSON-RPC 2.0 envelope that covers both requests and
// responses. Fields are populated selectively depending on direction.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *string         `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Writer — sends JSON-RPC 2.0 messages over an io.Writer (plugin stdin)
// ---------------------------------------------------------------------------

// rpcWriter serialises JSON-RPC 2.0 lines and writes them to an underlying
// io.Writer. All methods are safe for concurrent use.
type rpcWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func newRPCWriter(w io.Writer) *rpcWriter {
	return &rpcWriter{w: w}
}

// call serialises a JSON-RPC 2.0 request, writes it as a single newline-
// terminated JSON line, and returns the UUID string id for response correlation.
func (rw *rpcWriter) call(method string, params any) (string, error) {
	id := uuid.NewString()

	raw, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("jsonrpc marshal params for %s: %w", method, err)
	}

	msg := rpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  raw,
	}
	line, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("jsonrpc marshal request for %s: %w", method, err)
	}
	line = append(line, '\n')

	rw.mu.Lock()
	defer rw.mu.Unlock()
	if _, err := rw.w.Write(line); err != nil {
		return "", fmt.Errorf("jsonrpc write %s: %w", method, err)
	}
	return id, nil
}

// respond writes a JSON-RPC 2.0 response for a plugin-initiated request back
// to the plugin's stdin. Exactly one of result or rpcErr should be non-nil.
func (rw *rpcWriter) respond(id string, result json.RawMessage, rpcErr *rpcError) error {
	msg := rpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  result,
		Error:   rpcErr,
	}
	line, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("jsonrpc marshal response: %w", err)
	}
	line = append(line, '\n')

	rw.mu.Lock()
	defer rw.mu.Unlock()
	_, err = rw.w.Write(line)
	return err
}

// ---------------------------------------------------------------------------
// Scanner — reads JSON-RPC 2.0 lines from an io.Reader (plugin stdout)
// ---------------------------------------------------------------------------

// rpcScanner reads newline-delimited JSON-RPC 2.0 messages from an
// io.Reader. It is NOT safe for concurrent use; run it in a single goroutine.
type rpcScanner struct {
	s *bufio.Scanner
}

func newRPCScanner(r io.Reader) *rpcScanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // 4 MiB max line
	return &rpcScanner{s: sc}
}

// scan reads the next JSON-RPC message. It returns (nil, nil) when the
// underlying reader is closed cleanly, and a non-nil error on I/O failure.
func (rs *rpcScanner) scan() (*rpcMessage, error) {
	for rs.s.Scan() {
		line := rs.s.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip unparseable lines — the plugin may emit non-JSON debug output.
			continue
		}
		return &msg, nil
	}
	if err := rs.s.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

// scanLine reads the next raw line from the scanner.
func (rs *rpcScanner) scanLine() (string, error) {
	if rs.s.Scan() {
		return rs.s.Text(), nil
	}
	if err := rs.s.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}
