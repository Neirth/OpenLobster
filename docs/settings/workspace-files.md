---
description: The Workspace Files editor lets you edit the agent's persona and instruction files directly from the Settings view.
icon: file-pen
---

# Workspace Files Editor

The **System Files** tab within Settings provides an in-browser editor for the agent's workspace files. These are text files that the runtime reads to determine the agent's identity, persona, and behavioral instructions.

## Available files

| File | Purpose | What it affects | When it's used |
| ---- | ------- | --------------- | -------------- |
| `AGENTS.md` | Human-facing documentation describing available agents and their roles. For operator reference. | Operator understanding, not agent behavior. | When you check Settings; not sent to AI model |
| `SOUL.md` | The agent's personality, values, behavioral guidelines, and response style. Examples: "Be concise and direct", "Prioritize user privacy", "Explain technical concepts simply". | **Every response the agent generates.** Changes here immediately change how the agent talks and behaves. | **Step 6 of pipeline**: Injected into AI provider context as part of system prompt |
| `IDENTITY.md` | Core identity data: agent name, role, self-description, core mission. How the agent describes itself to users. | Agent's self-identification and how it presents itself in conversations. | **Step 6 of pipeline**: Included in system prompt so model knows "who" it is |

## How files feed into message processing

During **step 6** of the [message processing pipeline](../architecture/processing-messages.md), when the agent prepares to send a message to the AI provider:

1. System loads `IDENTITY.md` — "I am Claude, a helpful assistant"
2. System loads `SOUL.md` — "Be thoughtful, accurate, concise"
3. System includes conversation history, tools, and memory
4. All of this gets sent to OpenAI/Anthropic/Ollama as the system prompt
5. AI provider generates response using this context

**Result:** Your `IDENTITY.md` and `SOUL.md` files shape every single message the agent produces.

### Practical examples

**IDENTITY.md** (3-4 lines)
```markdown
# Identity

I am OpenLobster, a personal AI assistant.
I help with tasks across multiple channels (Telegram, Discord, etc.).
I maintain context across conversations and remember what users tell me.
```

**SOUL.md** (key behavioral instructions)
```markdown
# Behavioral Guidelines

- Be concise: answer briefly unless depth is requested
- Admit uncertainty: "I don't know" is better than guessing
- Respect privacy: never store or repeat sensitive data
- Ask before using tools: "Should I search the web for this?" (if in ask mode)
```

These directly influence every response. Different content = different agent personality.

## How to edit a file

1. Click the **System Files** tab in the Settings view.
2. Select the file tab you want to edit (`AGENTS.md`, `SOUL.md`, or `IDENTITY.md`).
3. Edit the content in the text area.
4. Click **Save**. A success or error indicator confirms whether the file was written correctly.

{% hint style="warning" %}
These files directly influence agent behavior. `SOUL.md` and `IDENTITY.md` are injected into the agent's system prompt context. Edits take effect on the next agent session. Make intentional changes and keep a backup of the original content when editing production instances.
{% endhint %}
