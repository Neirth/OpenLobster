---
description: The Recent Conversations panel shows the latest user sessions and lets you jump directly into any conversation.
icon: comments
---

# Recent Conversations Panel

The Recent Conversations panel lists the most recent user sessions, giving you a quick view of who has been talking to the agent and whether those conversations are still active.

## What each row shows

* **Avatar** — A colored circle showing the first letter of the user's name. Helps you identify users at a glance.
* **User name** — The display name of the user. This is their identity in OpenLobster (mapped during the [pairing flow](../architecture/processing-messages.md)).
* **Channel badge** — The platform: Telegram, Discord, WhatsApp, Slack, or SMS. The agent can connect users from different channels but treats them as one user across all channels.
* **Status badge** — Either **Active** (user sent a message recently) or **Idle** (no messages for a while). This is real-time, updated as messages arrive.

## Active vs Idle

**Active** means the user sent a message within the last activity window (typically 15-30 minutes, configurable). This badge updates immediately when:

1. User sends message (step 1 of message processing pipeline)
2. Adapter receives it
3. Session is marked "active" in the database

**Idle** means no recent messages. The conversation still exists in the database, but the user hasn't messaged recently.

**Why this matters:** If you're monitoring for important customers or support issues, the status badge tells you whether they're currently engaged. An "Idle" status doesn't mean they're unresponsive — just that they haven't messaged recently.

## Opening a conversation

Click any row to open the full conversation history in the [Chat](../chat/README.md) view. This is the fastest way to get from the Dashboard to a specific conversation.

{% hint style="info" %}
Only the most recent sessions are shown here. To see all conversations, navigate to the full [Chat](../chat/README.md) view using the sidebar.
{% endhint %}
