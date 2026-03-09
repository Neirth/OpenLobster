---
description: The Skills view lists all installed capability packages and shows their name and description.
icon: list-check
---

# Skills Overview

The Skills view lists every skill currently installed in the agent's workspace. Each entry shows the skill's name and a short description of what it adds to the agent.

## Skills vs Capabilities vs MCP Servers

It's easy to confuse these three, so here's the breakdown:

| Type | What it is | Where it comes from | How it integrates | Used for |
|------|-----------|-------------------|-------------------|-----------|
| **Skills** | Reusable instruction/behavior packages (prompts, chains, templates) | Installed locally from files or imported | Merged into system prompt before conversations | Repeatable workflows, behavioral patterns |
| **Capabilities** | Built-in tool categories (browser, terminal, filesystem, memory, audio) | Part of OpenLobster | Toggled on/off globally in step 5 of pipeline | Core functionality groups |
| **MCP Servers** | External services exposing tools via HTTP | Remote servers you connect | Step 5-9 of pipeline (tool discovery and execution) | External integrations (APIs, databases, etc.) |

All three are tools the agent can use, but they're different things.

## Understanding skills in the pipeline

During **step 5-6** of the [message processing pipeline](../architecture/processing-messages.md):

1. System loads all enabled **Capabilities** and **Skills**
2. Both are injected into the system prompt
3. Agent sees them as available tools/instructions
4. Model considers them when deciding how to respond

**Example:** If you have a skill called "provide_disclaimers", the system prompt might say "Before giving legal advice, always include a disclaimer (see skill: provide_disclaimers)". The agent then follows that instruction.

## What to check before acting

| Check | Why it matters |
|-------|----------------|
| **Name and description** | Confirm the skill does what you expect. Misleading names = unintended agent behavior. |
| **Provenance** | Know where the skill came from. Untrusted sources could inject malicious instructions. If you don't recognize a skill, investigate or remove it. |
| **Behavior impact** | Some skills modify how the agent responds (tone, format, rules). Test in a sandboxed conversation first. |

## Capabilities section

Below the skills list, the **Capabilities** section shows which built-in capability groups are currently enabled:

- **Browser** — Fetch and parse web pages
- **Terminal** — Execute shell commands
- **Filesystem** — Read/write files
- **Memory** — Query and update knowledge graph
- **Audio** — Voice/TTS features
- **Subagents** — Spawn parallel agents
- **MCP** — Call external tool servers

This is a read-only view. To toggle capabilities, go to **Settings > Agent Capabilities**. Disabling a capability removes those tools from the agent's awareness in step 5, preventing their use.

{% hint style="danger" %}
Enabling **Terminal** or **Filesystem** capabilities gives the agent direct server access. Enable only in controlled environments. See [Agent Capabilities](../settings/configuration.md#agent-capabilities) for risk levels.
{% endhint %}
