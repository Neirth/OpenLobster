// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package subprocess contains unexported protocol types that mirror the
// JSON-RPC 2.0 plugin wire format. Keeping them here ensures the backend
// core has no dependency on any plugin package.
package subprocess

import "encoding/json"

// ---------------------------------------------------------------------------
// Plugin methods
// ---------------------------------------------------------------------------

const (
	// methodGetInfo is called by the host during startup to identify the plugin.
	methodGetInfo = "get_info"

	// methodClose is called by the host to gracefully stop the plugin.
	methodClose = "close"
)

// getInfoRequest is the params payload for the "get_info" RPC method.
type getInfoRequest struct{}

// getInfoResponse is the result returned by a plugin in response to "get_info".
type getInfoResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Schema      json.RawMessage `json:"schema"`
	Properties  json.RawMessage `json:"properties,omitempty"`
	Exports     []string        `json:"exports"`
}

// closeRequest is the params payload for the "close" RPC method.
type closeRequest struct{}

// ---------------------------------------------------------------------------
// Function dispatch
// ---------------------------------------------------------------------------

// callResponse is the result returned by any plugin function invocation.
// Since the protocol is now flat, we expect the result to be the raw output.
type callResponse struct {
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Notification method names (plugin → host, no id field)
// ---------------------------------------------------------------------------

const (
	// methodEmitMessage is sent by the plugin when it wants to inject an
	// inbound message into the host's message pipeline.
	// Params: { "type": "emit_message", "payload": <object> }
	methodEmitMessage = "emit_message"

	// methodEmitLog is sent by the plugin for structured log output.
	// Params: { "level": "info|warn|error|debug", "message": "..." }
	methodEmitLog = "emit_log"
)
