---
description: The Dashboard gives you an at-a-glance view of system health, active channels, recent conversations, and live logs.
icon: gauge-high
---

# Dashboard

The Dashboard is the first screen you see after logging in. It acts as a command center: you can check key metrics, confirm channels are online, see who is talking to the agent, and read live logs — all without leaving a single page.

## Panels at a glance

The dashboard is divided into six panels. Each is described in its own page in this section.

* **[Statistic Cards](stat-cards.md)** — Key numbers: active sessions, messages sent and received, pending and completed tasks.
* **[Channels Panel](channels.md)** — Online/offline status for each configured messaging channel (Telegram, Discord, etc.).
* **[System Status Panel](system-status.md)** — Memory backend, secrets backend, and server uptime.
* **[Recent Conversations Panel](recent-conversations.md)** — Latest user sessions with a shortcut to open them in the Chat view.
* **[MCP Servers Panel](mcp-servers.md)** — Connection status and tool count for each connected MCP server.
* **[Recent Logs Panel](logs.md)** — Live feed of INFO, WARN, and ERROR messages from the backend.

## When to use the Dashboard

{% hint style="info" %}
Make the Dashboard your first stop whenever something seems off. Most problems produce a visible signal here: a channel going offline, an error in the logs, or metrics that stop updating.
{% endhint %}

Use the Dashboard to:

* Confirm the system started correctly after a restart (check **Uptime** and **Health**).
* Verify that all channels are online before expecting users to interact.
* Monitor activity during a high-traffic period.
* Find the first clue about an incident before diving into logs or settings.
