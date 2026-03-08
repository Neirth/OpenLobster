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

The settings page is organized into groups:

* **General Configuration** — Agent name, AI provider, and default model.
* **Agent Capabilities** — Enable or disable browser, terminal, memory, MCP, subagents, filesystem, sessions, and audio.
* **Database Configuration** — Driver (SQLite, Postgres, MySQL) and connection string.
* **Memory Configuration** — File-based or Neo4j memory backend.
* **Subagents Configuration** — Maximum concurrent subagents and default timeout.
* **GraphQL Configuration** — Host, port, and public base URL for the API.
* **Logging Configuration** — Log level and log file path.
* **Secrets Configuration** — File or OpenBao secrets backend.
* **Scheduler Configuration** — Enable/disable the scheduler and memory consolidation.
* **Channel Configuration** — Enable and configure Telegram, Discord, WhatsApp, Slack, and Twilio SMS.

## System Files

The **System Files** tab in Settings provides an editor for workspace files — `AGENTS.md`, `SOUL.md`, and `IDENTITY.md`. These files influence the agent's persona and instructions at runtime.

## Pages in this section

* [Configuration Editor](configuration.md)
* [Workspace Files Editor](workspace-files.md)
