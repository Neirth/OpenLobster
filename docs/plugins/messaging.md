---
description: "Specification for messaging gateway plugins in OpenLobster."
icon: message-dots
---

# Messaging Plugins

Messaging plugins connect OpenLobster to external communication platforms such as Telegram, Discord, Slack, and WhatsApp. They manage the bidirectional flow of text, media, and events between the user and the Core.

## Identification

*   **Type**: `messaging`
*   **Required Functions**: `send`, `resolve_channel_id`
*   **Recommended Functions**: `start`, `typing`

## Handshake Details

During the handshake phase, messaging plugins must specify their ingress strategy and capability matrix within the `properties` field.

### Inbound Mode (`inbound_mode`)
Identifies how the plugin receives messages from the upstream platform:
- `polling`: The plugin actively fetches updates (e.g., long-polling).
- `webhook`: The plugin expects the Host to route incoming HTTP requests to it via the `handle_webhook` function.
- `disabled`: The plugin only supports outbound messages.

### Capabilities Matrix
A structured object defining supported features:
- `supports_audio`: Whether the channel can handle voice messages.
- `supports_media`: Whether the channel can handle images or document attachments.

---

## Exported Functions

### `send`
Dispatches an outbound message from the Core to a specific platform user or group.

**Request Parameters:**
```json
{
  "message": {
    "channel_id": "platform-chat-id",
    "content": "Message text content",
    "audio": { "data": "base64_encoded_string", "format": "ogg" }
  },
  "config": { ... }
}
```

**Response:**
Returns `{"ok": true}` on success or an error object.

### `resolve_channel_id`
Translates incoming metadata or alternative identifiers into a canonical `channel_id` recognized by the platform.

---

## Inbound Flow (Plugin → Host)

Messaging plugins are usually autonomous. They maintain their own connection to the platform API and push new events to the Host.

### Callback: `emit_message`
The plugin invokes this notification on the Host whenever a new user message arrives.

**Payload Specification:**
```json
{
  "jsonrpc": "2.0",
  "method": "emit_message",
  "params": {
    "payload": {
      "channel_id": "123456789",
      "sender_id": "@username",
      "sender_name": "John Doe",
      "content": "Hello OpenLobster!",
      "metadata": { ... }
    }
  }
}
```

## Technical Considerations

1.  **Concurrency**: Messaging plugins must be able to handle multiple `send` requests while simultaneously processing inbound events.
2.  **Graceful Recovery**: Plugins are responsible for implementing reconnection logic if the upstream platform API becomes unavailable.
3.  **State Management**: If a plugin uses `webhook` mode, it must remain stateless as the Host may distribute ingress traffic across multiple plugin instances.
