---
description: Plugins extend OpenLobster with new AI providers, messaging channels, memory backends, and more.
icon: puzzle-piece
---

# Plugins

Plugins are binary extensions that add new capabilities to OpenLobster. Each plugin runs as a separate subprocess and communicates with the core through a bidirectional JSON-RPC 2.0 protocol over standard input/output streams. Plugins can provide AI model access, messaging integrations, memory storage, audio synthesis, and secret management backends.

## Plugin types

OpenLobster supports five categories of plugins, each identified by a type field:

| Type | Description | Typical exports |
|------|------------|----------------|
| `ai` | AI model providers (Anthropic, OpenAI, Ollama) | `chat`, `embed` |
| `messaging` | User chat channels (Telegram, Discord, Slack, WhatsApp, Twilio) | `send`, `start` |
| `memory` | Knowledge graph backends (GML, Neo4j) | `get`, `set`, `query` |
| `audio` | Text-to-speech services (ElevenLabs) | `synthesize` |
| `secrets` | Secret storage backends (JSON file, OpenBao) | `get`, `set`, `delete` |

## How plugins work

Plugins communicate with the OpenLobster host through a bidirectional channel:

1. **Initialization**: On startup, the plugin receives a metadata request and responds with its identifying information.
2. **Configuration**: The host sends configuration parameters that the plugin stores for subsequent operations.
3. **Execution**: The plugin receives function calls with input data and returns results asynchronously.
4. **Callbacks**: The plugin can call the host at any time to send messages, request secrets, or query data.

This design keeps the plugin isolated from the core, so crashes or bugs in a plugin do not affect the main application. Plugins are always asynchronous and can initiate requests to the host at any time.

## Using plugins

Most users install pre-built plugins from the OpenLobster plugin registry. To add a new plugin:

1. Build or download the plugin binary for your platform.
2. Place it in the configured plugins directory.
3. Configure the plugin through the Settings panel.
4. Enable the plugin in the desired channels.

## Creating custom plugins

If you need a provider or integration not available in the registry, you can create your own plugin. Plugins are built using the Rust SDK, which handles the protocol implementation so you can focus on your plugin's specific functionality.

{% hint style="info" %}
The SDK provides the JSON-RPC communication layer, schema validation, hot-configuration management, and the ability to call back to the host, so you only need to implement your plugin's business logic.
{% endhint %}

## Pages in this section

* [Plugin Protocol](protocol.md) — Technical details of the STDIO/JSON-RPC transport
* [AI Providers](ai.md) — Specification for LLM and provider plugins
* [Messaging Gateways](messaging.md) — Specification for chat platform integrations
* [Memory Backends](memory.md) — Specification for knowledge and storage plugins
* [Audio & TTS](audio.md) — Specification for audio processing plugins
* [Secrets & Vaults](secrets.md) — Specification for sensitive data management
* [Creating Custom Plugins](create.md) — Build your own plugin from scratch