// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subprocess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	pluginrpc "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/protocol"
	"github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/transport/stdio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type pluginMetadataPayload struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Version     string          `json:"version,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

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

func (a *Adapter) startLocked(ctx context.Context) error {
	hostConn, childFile, err := openSocketpair()
	if err != nil {
		return err
	}

	cmd := exec.Command(a.binaryPath)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = hostConn.Close()
		_ = childFile.Close()
		return fmt.Errorf("plugin %s: stdin pipe: %w", a.id, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = hostConn.Close()
		_ = childFile.Close()
		_ = stdin.Close()
		return fmt.Errorf("plugin %s: stdout pipe: %w", a.id, err)
	}
	cmd.ExtraFiles = []*os.File{childFile}

	if err := cmd.Start(); err != nil {
		_ = hostConn.Close()
		_ = childFile.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("plugin %s: start %s: %w", a.id, a.binaryPath, err)
	}
	_ = childFile.Close()

	if err := stdio.WriteFrame(stdin, stdio.HandshakeFrame{
		Type:      stdio.HandshakeTypeRequest,
		Version:   stdio.HandshakeVersion,
		Transport: "grpc_socketpair",
		FD:        3,
	}); err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("plugin %s: write handshake: %w", a.id, err)
	}

	ack, err := readHandshakeAck(stdout, handshakeTimeout)
	if err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("plugin %s: handshake ack: %w", a.id, err)
	}
	if ack.Type != stdio.HandshakeTypeAck || !ack.OK {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if strings.TrimSpace(ack.Error) != "" {
			return fmt.Errorf("plugin %s: handshake rejected: %s", a.id, strings.TrimSpace(ack.Error))
		}
		return fmt.Errorf("plugin %s: invalid handshake ack", a.id)
	}

	dialer := newSingleConnDialer(hostConn)

	grpcConn, err := grpc.NewClient(
		"passthrough:///openlobster-plugin",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer.DialContext),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(pluginrpc.JSONCodec{})),
	)
	if err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("plugin %s: grpc dial: %w", a.id, err)
	}

	client := pluginrpc.NewPluginServiceClient(grpcConn)
	infoCtx, cancelInfo := context.WithTimeout(ctxOrBackground(ctx), handshakeTimeout)
	defer cancelInfo()
	info, err := client.GetInfo(infoCtx, &pluginrpc.GetInfoRequest{})
	if err != nil {
		_ = grpcConn.Close()
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return fmt.Errorf("plugin %s: info: %w", a.id, err)
	}

	resolved := pluginMetadataPayload{
		ID:          strings.TrimSpace(info.ID),
		Name:        strings.TrimSpace(info.Name),
		Version:     strings.TrimSpace(info.Version),
		Description: strings.TrimSpace(info.Description),
		Type:        strings.TrimSpace(info.Type),
		Schema:      append(json.RawMessage(nil), info.Schema...),
		Properties:  append(json.RawMessage(nil), info.Properties...),
	}
	if resolved.ID == "" {
		resolved.ID = a.id
	}
	if resolved.ID == "" {
		resolved.ID = moduleStem(a.binaryPath)
	}
	if resolved.Name == "" {
		resolved.Name = resolved.ID
	}

	a.cmd = cmd
	a.stdin = stdin
	a.stdout = stdout
	a.grpcConn = grpcConn
	a.client = client
	a.id = resolved.ID
	a.name = resolved.Name
	a.version = resolved.Version
	a.description = resolved.Description
	a.pluginType = resolved.Type
	a.schemaJSON = normalizeMetadataSchema(resolved.Schema)
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

	eventCtx, eventCancel := context.WithCancel(context.Background())
	a.eventCancel = eventCancel
	go a.consumeEvents(eventCtx, client)
	go a.monitorProcess(cmd)

	return nil
}

func (a *Adapter) stopLocked() error {
	if a.eventCancel != nil {
		a.eventCancel()
		a.eventCancel = nil
	}

	var closeErr error
	if a.client != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := a.client.Close(closeCtx, &pluginrpc.CloseRequest{})
		cancel()
		if err != nil {
			closeErr = err
		}
	}
	if a.grpcConn != nil {
		if err := a.grpcConn.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	if a.stdin != nil {
		if err := a.stdin.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	if a.stdout != nil {
		if err := a.stdout.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}

	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}

	a.cmd = nil
	a.stdin = nil
	a.stdout = nil
	a.grpcConn = nil
	a.client = nil

	return closeErr
}

func (a *Adapter) consumeEvents(ctx context.Context, client pluginrpc.PluginServiceClient) {
	stream, err := client.StreamEvents(ctx, &pluginrpc.StreamEventsRequest{})
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("plugin %s: stream events init failed: %v", a.id, err)
		}
		return
	}

	for {
		event, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("plugin %s: stream events closed: %v", a.id, err)
			}
			return
		}
		switch event.Type {
		case pluginrpc.EventTypeEmitMessage:
			if a.onMessage != nil && len(event.Payload) > 0 {
				payload := append([]byte(nil), event.Payload...)
				go a.onMessage(payload)
			}
		case pluginrpc.EventTypeLog:
			level := strings.TrimSpace(strings.ToLower(event.Level))
			if level == "" {
				level = "info"
			}
			log.Printf("plugin %s [%s]: %s", a.id, level, strings.TrimSpace(event.Message))
		}
	}
}

func (a *Adapter) monitorProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	a.stateMu.Lock()
	a.available = false
	if err != nil {
		a.lastError = fmt.Errorf("plugin %s exited: %w", a.id, err)
	}
	a.stateMu.Unlock()
}

func readHandshakeAck(stdout io.Reader, timeout time.Duration) (stdio.HandshakeFrame, error) {
	reader := bufio.NewReader(stdout)
	type result struct {
		frame stdio.HandshakeFrame
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		frame, err := stdio.ReadFrame(reader)
		ch <- result{frame: frame, err: err}
	}()

	select {
	case got := <-ch:
		return got.frame, got.err
	case <-time.After(timeout):
		return stdio.HandshakeFrame{}, fmt.Errorf("timeout waiting handshake ack")
	}
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
