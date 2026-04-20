---
description: The Settings view lets you configure every aspect of the agent — AI provider, channels, capabilities, database, memory, and more.
icon: gear
---

# Settings

The Settings view is where you configure the OpenLobster agent. Changes made here affect how the agent connects to AI providers, which channels it listens on, what capabilities it has, where it stores data, and how it logs activity.

{% hint style="warning" %}
Configuration changes apply to the running server and take effect on save. For most backend changes (database driver, memory backend, GraphQL host), a server restart is required for the new values to take effect.
{% endhint %}

## Configuration sections

The settings page is organized into groups. Each affects different aspects of the system:

| Section | What it controls | Takes effect | Why it matters |
|---------|------------------|-------------|----------------|
| **General** | Agent name, AI provider (OpenAI/Anthropic/Ollama/etc.), default model | Immediately on save | Determines step 7-8 of pipeline (which AI generates responses) |
| **Capabilities** | Which built-in tools are available (browser, terminal, memory, MCP, etc.) | Immediately on save | Affects step 5 of pipeline (which tools agent can see) |
| **Database** | Where conversations, users, and tasks are stored (SQLite/Postgres/MySQL) | **Requires restart** | Persistence layer for entire system; wrong choice = data loss or poor performance |
| **Memory** | Knowledge graph backend (File/GML or Neo4j) | **Requires restart** | Affects step 4 of pipeline (context loading) and memory search speed |
| **Subagents** | Max concurrent subagents, timeout limits | Immediately on save | Resource management; affects scalability |
| **GraphQL** | API host, port, public URL | **Requires restart** | Affects how dashboard and external clients connect to the agent |
| **Logging** | Log verbosity, output location | Immediately on save | Affects what you see in Recent Logs for debugging |
| **Secrets** | Where to store API keys and tokens (File encrypted or OpenBao vault) | **Requires restart** | Security-critical; affects which credentials the agent can access |
| **Scheduler** | Enable/disable task scheduling, memory consolidation frequency | Immediately on save | Affects whether scheduled tasks run at all |
| **Channels** | Enable and configure Telegram, Discord, Slack, WhatsApp, SMS | Immediately on save | Affects step 1 (which adapters listen for messages) |

### Quick reference: Which settings need restart?

These require a server restart to take effect:
- Database driver or connection string
- Memory backend selection or connection details
- Secrets backend selection or connection details
- GraphQL host, port, or base URL

These take effect immediately:
- Agent name, AI provider, model
- Capabilities (on/off toggles)
- Scheduler enable/disable
- Channel configuration
- Logging level
- Subagent limits

If you change a setting and nothing happens, check if it's in the "requires restart" list.

## System Files

The **System Files** tab in Settings provides an editor for workspace files — `AGENTS.md`, `SOUL.md`, and `IDENTITY.md`. These files influence the agent's persona and instructions at runtime.

## Pages in this section

* [Configuration Editor](configuration.md)
* [Workspace Files Editor](workspace-files.md)
