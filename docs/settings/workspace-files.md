---
description: The Workspace Files editor lets you edit the agent's persona and instruction files directly from the Settings view.
icon: file-pen
---

# Workspace Files Editor

The **System Files** tab within Settings provides an in-browser editor for the agent's workspace files. These are text files that the runtime reads to determine the agent's identity, persona, and behavioral instructions.

## Available files

| File | Purpose |
| ---- | ------- |
| `AGENTS.md` | Human-facing documentation describing the configured agents and their roles. |
| `SOUL.md` | The agent's personality, values, and behavioral style. Read by the runtime as part of the system prompt. |
| `IDENTITY.md` | Core identity data for the agent — name, role, and self-description used during conversations. |

## How to edit a file

1. Click the **System Files** tab in the Settings view.
2. Select the file tab you want to edit (`AGENTS.md`, `SOUL.md`, or `IDENTITY.md`).
3. Edit the content in the text area.
4. Click **Save**. A success or error indicator confirms whether the file was written correctly.

{% hint style="warning" %}
These files directly influence agent behavior. `SOUL.md` and `IDENTITY.md` are injected into the agent's system prompt context. Edits take effect on the next agent session. Make intentional changes and keep a backup of the original content when editing production instances.
{% endhint %}
