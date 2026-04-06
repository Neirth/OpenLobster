//go:build !tinygo

// Copyright (c) OpenLobster contributors. See LICENSE for details.

package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"

	pluginrpc "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/protocol"
	"github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/transport/socket"
	"github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/transport/stdio"
	"google.golang.org/grpc"
)

const (
	metadataExportName = "get_metadata"
)

type eventHub struct {
	mu      sync.RWMutex
	nextID  uint64
	subs    map[uint64]chan *pluginrpc.Event
	closed  bool
	closeCh chan struct{}
}

func newEventHub() *eventHub {
	return &eventHub{
		subs:    make(map[uint64]chan *pluginrpc.Event),
		closeCh: make(chan struct{}),
	}
}

func (h *eventHub) subscribe() (uint64, <-chan *pluginrpc.Event, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return 0, nil, net.ErrClosed
	}
	h.nextID++
	id := h.nextID
	ch := make(chan *pluginrpc.Event, 64)
	h.subs[id] = ch
	return id, ch, nil
}

func (h *eventHub) unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(ch)
	}
}

func (h *eventHub) close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	for id, ch := range h.subs {
		delete(h.subs, id)
		close(ch)
	}
	close(h.closeCh)
	h.mu.Unlock()
}

func (h *eventHub) publish(evt *pluginrpc.Event) {
	if evt == nil {
		return
	}

	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return
	}
	targets := make([]chan *pluginrpc.Event, 0, len(h.subs))
	for _, ch := range h.subs {
		targets = append(targets, ch)
	}
	h.mu.RUnlock()

	for _, ch := range targets {
		copyEvt := *evt
		copyEvt.Payload = append([]byte(nil), evt.Payload...)
		select {
		case ch <- &copyEvt:
		default:
		}
	}
}

type service struct {
	pluginrpc.UnimplementedPluginServiceServer

	plugin     Plugin
	hub        *eventHub
	grpcServer *grpc.Server
	stopOnce   sync.Once
}

func (s *service) emitMessage(payload []byte) {
	if len(payload) == 0 {
		return
	}
	s.hub.publish(&pluginrpc.Event{Type: pluginrpc.EventTypeEmitMessage, Payload: append([]byte(nil), payload...)})
}

func (s *service) emitLog(level, msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	s.hub.publish(&pluginrpc.Event{Type: pluginrpc.EventTypeLog, Level: strings.TrimSpace(level), Message: strings.TrimSpace(msg)})
}

func (s *service) callExport(name string, input []byte) ([]byte, error) {
	fn, ok := s.plugin.Exports[name]
	if !ok {
		return nil, fmt.Errorf("unknown export: %s", name)
	}
	return invokeWithScope(input, s.emitMessage, s.emitLog, fn)
}

func (s *service) resolveMetadata() Metadata {
	raw, err := s.callExport(metadataExportName, nil)
	if err == nil {
		if decoded, parseErr := parseMetadataJSON(raw); parseErr == nil {
			fallback := s.plugin.Metadata.normalize(strings.TrimSpace(s.plugin.ID))
			return coalesceMetadata(decoded, fallback).normalize(strings.TrimSpace(s.plugin.ID))
		}
	}
	return s.plugin.Metadata.normalize(strings.TrimSpace(s.plugin.ID))
}

func (s *service) GetInfo(context.Context, *pluginrpc.GetInfoRequest) (*pluginrpc.GetInfoResponse, error) {
	exports := make([]string, 0, len(s.plugin.Exports))
	for name := range s.plugin.Exports {
		exports = append(exports, name)
	}
	sort.Strings(exports)

	meta := s.resolveMetadata()

	return &pluginrpc.GetInfoResponse{
		ID:          meta.ID,
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Type:        meta.Type,
		Schema:      append([]byte(nil), meta.Schema...),
		Properties:  append([]byte(nil), meta.Properties...),
		Exports:     exports,
	}, nil
}

func (s *service) Call(_ context.Context, req *pluginrpc.CallRequest) (*pluginrpc.CallResponse, error) {
	if req == nil {
		return &pluginrpc.CallResponse{Error: "nil request"}, nil
	}
	fn := strings.TrimSpace(req.Function)
	if fn == "" {
		return &pluginrpc.CallResponse{Error: "function is required"}, nil
	}
	out, err := s.callExport(fn, req.Input)
	if err != nil {
		return &pluginrpc.CallResponse{Error: err.Error()}, nil
	}
	return &pluginrpc.CallResponse{Output: out}, nil
}

func (s *service) StreamEvents(_ *pluginrpc.StreamEventsRequest, stream pluginrpc.PluginService_StreamEventsServer) error {
	subID, ch, err := s.hub.subscribe()
	if err != nil {
		return err
	}
	defer s.hub.unsubscribe(subID)

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			if sendErr := stream.Send(evt); sendErr != nil {
				return sendErr
			}
		}
	}
}

func (s *service) Close(context.Context, *pluginrpc.CloseRequest) (*pluginrpc.CloseResponse, error) {
	s.stopOnce.Do(func() {
		s.hub.close()
		go s.grpcServer.Stop()
	})
	return &pluginrpc.CloseResponse{}, nil
}

// Run starts the native plugin server and blocks until the host closes it.
func Run(plugin Plugin) error {
	if strings.TrimSpace(plugin.ID) == "" {
		return fmt.Errorf("runtime: plugin ID is required")
	}
	if len(plugin.Exports) == 0 {
		return fmt.Errorf("runtime: plugin %s has no exports", plugin.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	frame, err := stdio.ReadFrame(reader)
	if err != nil {
		return fmt.Errorf("runtime: read handshake: %w", err)
	}

	ack := stdio.HandshakeFrame{Type: stdio.HandshakeTypeAck, Version: stdio.HandshakeVersion, OK: true}
	if frame.Type != stdio.HandshakeTypeRequest {
		ack.OK = false
		ack.Error = "invalid handshake type"
	}
	if frame.Version != stdio.HandshakeVersion {
		ack.OK = false
		ack.Error = fmt.Sprintf("unsupported handshake version: %d", frame.Version)
	}
	if strings.TrimSpace(frame.Transport) != "grpc_socketpair" {
		ack.OK = false
		ack.Error = fmt.Sprintf("unsupported transport: %q", frame.Transport)
	}
	if frame.FD <= 0 {
		ack.OK = false
		ack.Error = "missing socket fd"
	}
	if writeErr := stdio.WriteFrame(os.Stdout, ack); writeErr != nil {
		return fmt.Errorf("runtime: write handshake ack: %w", writeErr)
	}
	if !ack.OK {
		return errors.New(ack.Error)
	}

	socketFile := os.NewFile(uintptr(frame.FD), "plugin-socket")
	if socketFile == nil {
		return fmt.Errorf("runtime: cannot open socket fd %d", frame.FD)
	}
	conn, err := net.FileConn(socketFile)
	_ = socketFile.Close()
	if err != nil {
		return fmt.Errorf("runtime: socket conn: %w", err)
	}
	listener := socket.NewSingleConnListener(conn)
	defer listener.Close()

	grpcServer := grpc.NewServer()
	hub := newEventHub()
	svc := &service{plugin: plugin, hub: hub, grpcServer: grpcServer}
	pluginrpc.RegisterPluginServiceServer(grpcServer, svc)

	serveErr := grpcServer.Serve(listener)
	if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
		return fmt.Errorf("runtime: grpc serve: %w", serveErr)
	}
	return nil
}

// MustRun starts Run and exits the process on error.
func MustRun(plugin Plugin) {
	if err := Run(plugin); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
