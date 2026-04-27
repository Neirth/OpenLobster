---
description: "Specification for AI provider plugins in OpenLobster."
icon: microchip
---

# AI Provider Plugins

AI plugins extend OpenLobster with new Large Language Models (LLMs) or providers. They translate the Core's requests into the specific API calls required by the provider (e.g., Anthropic, OpenAI, Ollama).

## Identification

- **Type**: `ai`
- **Required Function**: `chat`
- **Recommended Function**: `configure`

## Handshake Details

### `get_info`
Returns general plugin metadata plus AI-specific capabilities.

- **`properties`**:
    - `supports_audio_input` (bool): If true, the model can process audio data.
    - `supports_audio_output` (bool): If true, the model can generate audio responses directly.
    - `supports_images` (bool): If true, the model accepts image inputs.

---

## Exported Functions

### `chat`
Main entry point for text and tool-based interactions.

**Request Parameters:**

```json
{
  "model": "gpt-4",
  "messages": [
    { "role": "user", "content": "Hello" }
  ],
  "tools": [...],
  "max_tokens": 4096,
  "config": { ... }
}
```

**Expected Result:**
The plugin MUST return a standard chat completion object.

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

## Tool Calling Flow

OpenLobster expects AI plugins to handle tool calling following the standard iterative pattern:
1. Core sends available tools in the `chat` request.
2. Plugin responds with `tool_calls` if the model decides to use them.
3. Core executes tools and sends a new `chat` request with `role: "tool"` messages containing the results.
4. Plugin generates the final user-facing response.
