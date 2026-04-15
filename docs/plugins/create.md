---
description: Build a custom plugin for OpenLobster from scratch.
icon: code
---

# Creating Custom Plugins

This guide explains how to create a custom plugin for OpenLobster. Plugins are standalone binaries that communicate with the host through JSON-RPC 2.0 over standard streams, allowing you to extend the platform with new providers and integrations.

## How the protocol works

Plugins communicate with the OpenLobster host through a bidirectional channel using standard input and output streams. All messages are line-delimited JSON objects following the JSON-RPC 2.0 specification.

### Bidirectional communication

Unlike traditional request-response patterns, the protocol allows both sides to initiate calls:

```
host  ──GetInfo(uuid)──────────► plugin
plugin ◄──result(uuid)─────────────────

host  ──Call(uuid, fn, input)──────► plugin
plugin ◄──result(uuid)─────────────────

plugin ──Call(uuid, method, params)──► host
      ◄──result(uuid)────────────────

host  ──Close(uuid)──────────────► plugin
plugin ◄──result(uuid, {})────────────
```

This bidirectional design lets plugins request data from the host (e.g. secrets, configuration) at any time, rather than waiting for the host to ask.

### Request IDs

All request IDs are UUID v4 strings, not numbers. Either side generates a UUID when initiating a request and the responder echoes it in the response.

### Asynchronous execution

Plugins are always asynchronous. The runner spawns a background task that reads stdin, while the main task dispatches requests. This allows the plugin to initiate calls to the host while also responding to incoming requests.

## Required metadata

Every plugin must identify itself on startup by returning metadata with information about its capabilities and configuration schema.

### PluginInfo structure

| Field | Description |
|-------|-------------|
| `id` | Unique identifier string (e.g. `"openlobster-ai-anthropic"`) |
| `name` | Human-readable display name |
| `version` | Semantic version string |
| `description` | One-line description of purpose |
| `type` | Plugin category: `"ai"`, `"messaging"`, `"memory"`, `"audio"`, or `"secrets"` |
| `schema` | JSON Schema object describing required configuration fields |
| `properties` | JSON object with capability flags |
| `exports` | List of function names the plugin exposes |

The schema follows JSON Schema conventions, defining what configuration keys the host must provide and which are required versus optional.

## Exported functions

Plugins expose functions that the host can call. The exact functions depend on the plugin type:

| Plugin type | Required functions |
|-------------|-------------------|
| `ai` | `chat`, optionally `embed` |
| `messaging` | `send`, optionally `start` (for webhook listeners) |
| `memory` | `get`, `set`, optionally `query` |
| `audio` | `synthesize` |
| `secrets` | `get`, `set`, `delete` |

Every plugin must implement a `configure` function that accepts runtime configuration. This allows the host to update settings without restarting the plugin.

## Configuration handling

Plugins receive configuration as JSON data through the `configure` function. The configuration is stored in memory and merged with per-call overrides when functions are invoked.

### Configuration merge rules

1. If the host provides per-call values, those take precedence.
2. Otherwise, stored configuration values are used.
3. If neither is present, default values from the schema are applied.

This pattern allows flexible configuration where some parameters can be set globally while others are overridden per-request.

## Calling the host from a plugin

Plugins can initiate JSON-RPC calls to the host at any time using `call_core()`. This is useful for:

- Retrieving secrets (API keys, tokens)
- Fetching dynamic configuration
- Querying the agent state

The call is asynchronous: the plugin writes the request to stdout and suspends until the host responds.

## Sending messages to the host

Since the protocol is bidirectional, the plugin simply calls the host using `call_core()` whenever it needs to send a message. No separate event mechanism is needed.

## Building a plugin

To create a custom plugin, you need to:

1. Create a new binary that implements the request-response loop.
2. Read line-delimited JSON requests from stdin.
3. Return line-delimited JSON responses to stdout.
4. Handle the three request types: metadata query, function calls, and shutdown.

The official OpenLobster SDK provides reusable implementations of this protocol. Refer to the SDK documentation for implementation details.

## Protocol examples

Here are the actual JSON messages exchanged between host and plugin. All request IDs are UUID v4 strings.

### Initialization: GetInfo request

The host requests plugin metadata:

```json
{"jsonrpc": "2.0", "method": "GetInfo", "params": null, "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}
```

The plugin responds with its metadata:

```json
{
  "jsonrpc": "2.0",
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "result": {
    "id": "openlobster-ai-anthropic",
    "name": "Anthropic Claude",
    "version": "0.1.0",
    "description": "Anthropic Claude AI provider plugin",
    "type": "ai",
    "schema": {
      "type": "object",
      "properties": {
        "api_key": {"type": "string"},
        "model": {"type": "string", "default": "claude-3-sonnet-20250219"}
      },
      "required": ["api_key"]
    },
    "properties": {},
    "exports": ["configure", "chat"]
  }
}
```

### Configuration

The host sends configuration:

```json
{
  "jsonrpc": "2.0",
  "method": "Call",
  "params": {
    "function": "configure",
    "input": {
      "config": {
        "api_key": "sk-ant-...",
        "model": "claude-3-opus-20240229"
      }
    }
  },
  "id": "b2c3d4e5-f6a7-8901-bcde-f23456789012"}
```

The plugin confirms:

```json
{"jsonrpc": "2.0", "id": "b2c3d4e5-f6a7-8901-bcde-f23456789012", "result": {"output": {"ok": true}}}
```

### Function call

The host invokes a plugin function:

```json
{
  "jsonrpc": "2.0",
  "method": "Call",
  "params": {
    "function": "chat",
    "input": {
      "model": "claude-3-opus-20240229",
      "messages": [
        {"role": "user", "content": "Hello"}
      ],
      "max_tokens": 1024
    }
  },
  "id": "c3d4e5f6-a7b8-9012-cdef-345678901234"}
}
```

The plugin returns the result:

```json
{
  "jsonrpc": "2.0",
  "id": "c3d4e5f6-a7b8-9012-cdef-345678901234",
  "result": {
    "output": {
      "content": "Hello! How can I help you?",
      "stop_reason": "end_turn",
      "usage": {"input_tokens": 10, "output_tokens": 12}
    }
  }
}
```

### Plugin calls the host (bidirectional)

The plugin can initiate calls to the host at any time, for example to fetch a secret:

```json
{
  "jsonrpc": "2.0",
  "method": "vault.get",
  "params": {"key": "anthropic_api_key"},
  "id": "d4e5f6a7-b8c9-0123-def0-456789012345"}
}
```

The host responds:

```json
{
  "jsonrpc": "2.0",
  "id": "d4e5f6a7-b8c9-0123-def0-456789012345",
  "result": "sk-ant-api03-..."
}
```

### Error response

When something goes wrong:

```json
{
  "jsonrpc": "2.0",
  "method": "Call",
  "params": {"function": "chat", "input": {}},
  "id": "e5f6a7b8-c9d0-1234-ef01-567890123456"}
}
```

The plugin returns an error:

```json
{
  "jsonrpc": "2.0",
  "id": "e5f6a7b8-c9d0-1234-ef01-567890123456",
  "error": {"code": -32602, "message": "invalid input: messages is required"}
}
```



### Graceful shutdown

The host requests the plugin to exit:

```json
{"jsonrpc": "2.0", "method": "Close", "params": null, "id": "f6a7b8c9-d0e1-2345-f012-678901234567"}
```

The plugin should clean up and exit:

```json
{"jsonrpc": "2.0", "id": "f6a7b8c9-d0e1-2345-f012-678901234567", "result": {"output": {}}}
```

## Testing your plugin

After building your plugin binary:

1. Place it in the plugins directory.
2. Configure it through the Settings panel.
3. Enable it in your channels.
4. Monitor logs for any errors during startup.

Common issues include invalid schema format, missing required exports, and protocol framing errors (each message must be on its own line).