---
description: Built-in capabilities are native features of the agent that can be enabled or disabled globally and controlled per user.
icon: bolt
---

# Built-in Capabilities

Built-in capabilities are feature groups implemented directly by the agent — as opposed to tools provided by external MCP servers. They include browser automation, terminal access, memory, filesystem, subagents, session interaction, and audio.

## Capability cards

Each capability appears as a card showing:

* **Name and description** — What the capability does.
* **Status badge** — Whether the capability is globally enabled or disabled in Settings.
* **Tools list** — The concrete tool names the model can call when this capability is active (accessible by clicking the card).

## Available capabilities

| Capability | What it allows the agent to do |
| ---------- | ------------------------------ |
| **Browser** | Fetch and browse web page content. |
| **Terminal** | Execute shell commands on the server. |
| **Subagents** | Launch and orchestrate parallel subagent instances. |
| **Memory** | Read and write to the long-term knowledge graph. |
| **Filesystem** | Read and write files directly on the server. |
| **Session Interaction** | Inspect and interact with other active agent sessions. |
| **MCP Gateway** | Call tools exposed by connected MCP servers. |
| **Audio** | Use voice, ASR (speech-to-text), and TTS (text-to-speech) features. |

## Status meanings

| Status | Meaning |
| ------ | ------- |
| **Active** | The capability is globally enabled and available to the model. |
| **Globally Disabled** | The capability is turned off in Settings. The model cannot use it regardless of user permissions. |
| **Denied for this user** | The capability is enabled globally but blocked for a specific user via the Permissions tab. |

## Enabling a disabled capability

If a capability shows **Globally Disabled** and you need the agent to use it, go to **Settings > Agent Capabilities**, toggle the relevant capability on, and save.

{% hint style="warning" %}
**Terminal** and **Filesystem** capabilities grant the agent direct access to the server. Enable them only in controlled environments with appropriate isolation.
{% endhint %}
