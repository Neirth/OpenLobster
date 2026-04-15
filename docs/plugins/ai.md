# AI Provider Plugins

AI plugins extend OpenLobster with new Large Language Models (LLMs) or providers.

## Identification

- **Type**: `ai`
- **Required Function**: `chat`
- **Recommended Function**: `configure`

## Core Methods

### `get_info`
Returns general plugin metadata plus AI-specific properties.
- **`properties`**:
    - `supports_audio_input` (bool): If true, `chat_with_audio` is available.
    - `supports_audio_output` (bool): If true, `chat_to_audio` is available.

---

## Exported Functions

### `chat`
Main entry point for text-based interactions.

**Request (`params`):**
```json
{
  "model": "gpt-4",
  "messages": [
    { "role": "user", "content": "Hello" }
  ]
}
```

**Input Payload:**
```json
{
  "model": "model-name",
  "messages": [
    { "role": "system", "content": "..." },
    { "role": "user", "content": "..." },
    { "role": "assistant", "content": "...", "tool_calls": [...] }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "tool_name",
        "description": "...",
        "parameters": { ... }
      }
    }
  ],
  "max_tokens": 4096,
  "config": { ... }
}
```

**Output Payload (`output` field in `CallResponse`)**:
```json
{
  "content": "Text response",
  "tool_calls": [
    {
      "id": "call_id",
      "type": "function",
      "function": {
        "name": "tool_name",
        "arguments": "{\"arg\": 1}"
      }
    }
  ],
  "stop_reason": "stop|tool_use|length|content_filter",
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50
  },
  "error": null
}
```

## Tool Calling Protocol

OpenLobster expects AI plugins to handle tool calling similarly to the OpenAI API:
1. Core sends available tools in the `chat` request.
2. Plugin responds with `tool_calls` if the model decides to use them.
3. Core executes tools and sends a new `chat` request with `role: "tool"` messages.
4. Plugin generates the final response.

## Capabilities (PluginInfo.properties)

- `supports_audio_input`: boolean (whether the model accepts audio binaries).
- `supports_audio_output`: boolean (whether the model can generate audio).
- `supports_images`: boolean.
