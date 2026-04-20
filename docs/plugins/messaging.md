# Messaging Plugins

Messaging plugins connect OpenLobster to communication platforms like Telegram, Discord, Slack, or WhatsApp.

## Identification

- **Type**: `messaging`
- **Required Functions**: `send`, `resolve_channel_id`, `configure`
- **Recommended Functions**: `start`, `typing`, `inbound_mode`

## Handshake: `get_info`

Messaging plugins must return their `inbound_mode` and `capabilities` within the `properties` field.

- **`inbound_mode`**: `"polling"` or `"webhook"` (identifies how the plugin receives messages).
- **`capabilities`**: Object defining what the plugin can do (e.g., `{"supports_embedding": true}`).

---

## Exported Functions

### `send`
Sends an outbound message to a specific channel.

**Request (`params`):**
```json
{
  "channel_id": "12345",
  "content": "Hello, world!"
}
```

**Input Payload (`input` field in `Call` request)**:
```json
{
  "message": {
    "channel_id": "platform-chat-id",
    "content": "Message text",
    "audio": { "data": "base64", "format": "ogg" },
    "metadata": { ... }
  },
  "config": { ... }
}
```

**Output**: `{"ok": true}` or `{"error": "..."}`

### Function: `typing`
Signals that the bot is "typing" on the platform.

**Input Payload**: `{"message": { "channel_id": "..." }, "duration_ms": 5000}`

## Inbound Flow (Plugin -> Core)

Messaging plugins are typically "autonomous": they maintain a connection (long-polling or webhook) and push events to the core.

### Method: `emit_message` (Callback)
The plugin must call this core method via STDIO whenever a new message is received from the platform.

**Params (JSON-RPC Request to core)**:
```json
{
  "jsonrpc": "2.0",
  "method": "emit_message",
  "params": {
    "type": "emit_message",
    "payload": {
      "channel_id": "...",
      "sender_id": "...",
      "sender_name": "...",
      "content": "...",
      "attachments": [...],
      "metadata": { ... }
    }
  }
}
```

## Methods for Core Discovery

### `inbound_mode`
Returns how the plugin handles incoming messages: `"polling"`, `"webhook"`, or `"none"`.

### `capabilities`
Returns a boolean map of features:
- `HasVoiceMessage`: can send/receive audio.
- `HasTextStream`: supports real-time text tokens.
- `HasMediaSupport`: supports images/files.

### `resolve_channel_id`
Translates ambiguous IDs or metadata into a canonical platform `channel_id`.
