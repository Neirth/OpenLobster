---
description: How OpenLobster works — understand the message processing flow, memory system, security, and task scheduling behind the scenes
icon: diagram-project
---

# How OpenLobster Works

This section explains the inner workings of OpenLobster — how messages flow from your users to the AI model, how the agent remembers conversations, how security is handled, and how tasks are scheduled.

You don't need to understand this to use OpenLobster, but it helps explain why things work the way they do.

## The big picture

When a user sends a message through Telegram, Discord, or any connected channel:

1. The message arrives and is validated (confirming the user is paired)
2. The agent builds context — pulling in the user's history, their tool permissions, and relevant memories
3. The message is sent to your configured AI model (OpenAI, Anthropic, etc.)
4. If the model needs to use a tool (search the web, run a command, etc.), those tools are executed
5. The response goes back to the user on the same channel
6. The conversation is saved, and new facts may be stored in the knowledge graph

This happens in seconds from the user's perspective. Behind the scenes, it involves multiple interconnected systems working together.

## Sections

- **[Message processing](processing-messages.md)** — How incoming messages flow through the system, from receipt to response delivery
- **[Memory graph system](memory-graph.md)** — How the agent builds and maintains long-term knowledge about users and topics
- **[Secrets protection](protecting-secrets.md)** — How API keys, tokens, and sensitive configuration data are kept secure
- **[Routing and scheduling](routing-scheduler.md)** — How tasks are scheduled, the scheduler works, and messages route to the correct channel

## Why this matters

Understanding this architecture helps you:
- **Troubleshoot issues** — If something isn't working, you'll understand where to look
- **Configure permissions correctly** — You'll see how per-user permissions are enforced throughout the pipeline
- **Optimize memory usage** — You'll know how the graph backend affects performance and search results
- **Plan for scale** — You'll understand which components need upgrades when handling many users
