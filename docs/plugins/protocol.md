---
description: "Technical specification of the OpenLobster Plugin Wire Protocol v2."
icon: network-wired
---

# Plugin Protocol

This document specifies the language-agnostic communication protocol between the OpenLobster host (Core) and its plugins.

## Conceptual Model

At its heart, the OpenLobster plugin system is based on **Process Isolation**. Instead of loading shared libraries (which is complex and varies by OS), OpenLobster runs each plugin as a separate **subprocess**.

Think of a plugin as a specialized worker that speaks a specific dialect. The Core acts as the manager, sending tasks via standard input and receiving results via standard output.

### Why STDIO and JSON-RPC?
- **Universal Support**: Every programming language can read from stdin and write to stdout.
- **Robustness**: If a plugin crashes, the Core remains unaffected.
- **Simplicity**: JSON-RPC 2.0 is a mature, human-readable standard for remote procedure calls.

---

## Transport: STDIO

Communication happens over standard streams:
- **Host → Plugin**: Standard Input (`stdin`)
- **Plugin → Host**: Standard Output (`stdout`)
- **Logging/Errors**: Standard Error (`stderr`) — used for unplanned crashes and raw debugging.

---

## Format: JSON-RPC 2.0

Every message is a single-line JSON object following the [JSON-RPC 2.0](https://www.jsonrpc.org/specification) specification. The pipe must stay open for the duration of the plugin's lifecycle.

### Message Envelope

```json
{
  "jsonrpc": "2.0",
  "id": "uuid-v4-string",
  "method": "method_name",
  "params": { ... }
}
```

---

## The Handshake (Hand-legible explanation)

When OpenLobster starts a plugin, the very first thing it does is ask: *"Who are you and what can you do?"*. This is the `get_info` call.

### 1. `get_info`
Called by the host immediately after starting the subprocess.

**Request:**
```json
{ "jsonrpc": "2.0", "id": "1", "method": "get_info", "params": {} }
```

**Response:**
The plugin returns its identity and its "contract" (schema and exports).
```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "result": {
    "id": "plugin-slug",
    "name": "Human Readable Name",
    "version": "1.0.0",
    "description": "Short description",
    "type": "ai|messaging|memory|audio|secrets",
    "schema": { ... },
    "properties": { ... },
    "exports": ["chat", "configure"]
  }
}
```

> [!NOTE]
> `properties` contains type-specific technical metadata (e.g., `supports_audio_input` for AI plugins).

---

## Lifecycle Management

### 2. `close` (Cleanup)
Called by the host to request a graceful shutdown. This allows the plugin to save state or close network connections properly.

**Request:**
```json
{ "jsonrpc": "2.0", "id": "2", "method": "close", "params": {} }
```

**Response:**
```json
{ "jsonrpc": "2.0", "id": "2", "result": {} }
```

---

## Function Dispatch

Plugins export specific capabilities (e.g., `chat`, `send`, `store`). These are invoked by the host as first-class JSON-RPC methods.

**Example: Calling `chat` on an AI plugin**

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": "3",
  "method": "chat",
  "params": {
    "model": "gpt-4",
    "messages": [...]
  }
}
```

**Response:**
Plugins MUST return an object containing `output` or `error`.

```json
{
  "jsonrpc": "2.0",
  "id": "3",
  "result": {
    "output": { "content": "Hello!" },
    "error": null
  }
}
```

---

## Bidirectional Communication

One of the most powerful features of the OpenLobster protocol is that it is **Bidirectional**. The plugin isn't just a passive responder; it can call the Core to ask for secrets, log information, or inject messages.

### Plugin → Host Notifications

Plugins can send fire-and-forget notifications to the core.

#### `emit_log`
Used for structured logging that appears in the host's terminal.

```json
{
  "jsonrpc": "2.0",
  "method": "emit_log",
  "params": {
    "level": "info|warn|error|debug",
    "message": "Connected to API"
  }
}
```

#### `emit_message` (Inbound)
Used by messaging plugins to inject messages into the core's pipeline when a new message is received from a platform (e.g., Telegram).

```json
{
  "jsonrpc": "2.0",
  "method": "emit_message",
  "params": {
    "payload": { ... }
  }
}
```
