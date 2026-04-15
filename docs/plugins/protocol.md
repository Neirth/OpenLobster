# OpenLobster Plugin Wire Protocol v2

This document specifies the language-agnostic communication protocol between the OpenLobster host (Core) and its plugins.

## Transport: STDIO

Plugins are executed as native subprocesses. Communication happens over standard streams:
- **Host → Plugin**: Standard Input (`stdin`)
- **Plugin → Host**: Standard Output (`stdout`)
- **Logging/Errors**: Standard Error (`stderr`) — used for unplanned crashes and raw debugging.

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

## Core Protocol Methods (snake_case)

All core-level control methods use `snake_case`.

### 1. `get_info` (Handshake)
Called by the host immediately after starting the subprocess.

**Request:**
```json
{ "jsonrpc": "2.0", "id": "1", "method": "get_info", "params": {} }
```

**Response:**
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

### 2. `close` (Cleanup)
Called by the host to request a graceful shutdown.

**Request:**
```json
{ "jsonrpc": "2.0", "id": "2", "method": "close", "params": {} }
```

**Response:**
```json
{ "jsonrpc": "2.0", "id": "2", "result": {} }
```

---

## Function Dispatch (Flat Architecture)

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

## Plugin → Host Notifications

Plugins can send fire-and-forget notifications to the core. These methods also use `snake_case`.

### `emit_log`
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

### `emit_message` (Inbound)
Used by messaging plugins to inject messages into the core's pipeline.

```json
{
  "jsonrpc": "2.0",
  "method": "emit_message",
  "params": {
    "payload": { ... }
  }
}
```
