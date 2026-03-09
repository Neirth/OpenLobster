---
description: The Chat view is where you monitor and participate in conversations between the agent and your users across all channels.
icon: message
---

# Chat

The Chat view is the operator's window into every conversation the agent is having. Messages arrive from Telegram, Discord, WhatsApp, Slack, SMS, and any other configured channel, and they all appear here in a unified interface.

The view has two main areas:

* **Conversations list** (left panel) — All current and past conversations, one row per user.
* **Message thread** (right panel) — The full message history for the selected conversation, plus the compose box for sending replies.

## What you can do in Chat

* Read the full message history for any user, including the agent's replies and tool calls.
* Send a message directly to a user on behalf of the agent.
* Attach a file reference to a message.
* Monitor conversations across multiple channels without switching platforms.
* Delete a user and all their data when required.

## What happens behind the scenes when a message arrives

Every message you see in Chat has gone through the [11-step message processing pipeline](../architecture/processing-messages.md). Here's the simplified flow:

```
User (Telegram, Discord, etc.)
         ↓
   Channel Adapter (translates platform format)
         ↓
   MessageHandler (validates user, loads context)
         ↓
   Memory Graph (retrieves relevant facts)
         ↓
   Tool Registry (prepares available tools)
         ↓
   AI Provider (sends to OpenAI, Anthropic, etc.)
         ↓
   Tool Execution (if AI model asked to use tools)
         ↓
   Response Generated (final message)
         ↓
   Router (sends back to original channel)
         ↓
   Chat View (you see it here)
```

**What you see in the Chat view:**

| Label | Meaning | Where It Comes From |
|-------|---------|-------------------|
| **USER** | Message from a person | Step 1 of the pipeline |
| **OPENLOBSTER** | Reply from the agent | Steps 3-8 of the pipeline |
| **TOOL** | A tool call and its result | Step 9 of the pipeline |

**Important:** Tool labels are for your visibility as an operator. When the agent sends the final response to the user on Telegram/Discord, they don't see the TOOL messages — only the final **OPENLOBSTER** response.

This is why the Chat view sometimes shows more messages than users see on their platform. The TOOL messages are your "behind the scenes" view of how the agent reached its conclusion.

## Channel and group conversations

OpenLobster supports both direct (one-to-one) and group conversations. When the agent is added to a group on Telegram or Discord, group messages appear in the conversation list alongside direct messages. The **Channel** badge on each conversation row tells you exactly which platform and conversation type it belongs to.

{% hint style="info" %}
Group conversations work the same way as direct conversations from the operator's perspective. Select the row to read the thread and compose a reply.
{% endhint %}

## Pages in this section

* [Conversations List](conversations-list.md)
* [Message Thread](message-thread.md)
* [Compose & Send](compose.md)
* [User Moderation](moderation.md)
