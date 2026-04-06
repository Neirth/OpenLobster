// Copyright (c) OpenLobster contributors. See LICENSE for details.

package stdio

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	HandshakeTypeRequest = "handshake_request"
	HandshakeTypeAck     = "handshake_ack"
	HandshakeVersion     = 1
)

// HandshakeFrame is exchanged over stdio before gRPC bootstraps over socketpair.
type HandshakeFrame struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	Transport string `json:"transport,omitempty"`
	FD        int    `json:"fd,omitempty"`
	OK        bool   `json:"ok,omitempty"`
	Error     string `json:"error,omitempty"`
}

func WriteFrame(w io.Writer, frame HandshakeFrame) error {
	if w == nil {
		return fmt.Errorf("stdio: write handshake: nil writer")
	}
	enc := json.NewEncoder(w)
	return enc.Encode(frame)
}

func ReadFrame(r *bufio.Reader) (HandshakeFrame, error) {
	if r == nil {
		return HandshakeFrame{}, fmt.Errorf("stdio: read handshake: nil reader")
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return HandshakeFrame{}, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return HandshakeFrame{}, fmt.Errorf("stdio: read handshake: empty frame")
	}
	var frame HandshakeFrame
	if err := json.Unmarshal([]byte(line), &frame); err != nil {
		return HandshakeFrame{}, fmt.Errorf("stdio: decode handshake: %w", err)
	}
	return frame, nil
}
