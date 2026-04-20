---
description: Use the compose box to write and send messages to users directly from the Chat view.
icon: paper-plane
---

# Compose & Send

The compose box sits at the bottom of the message thread. Use it to write a reply and send it to the user through the same channel they used to contact the agent.

## Sending a message

Type your message in the text field. Press `Enter` to add a new line. When your message is ready, send it using either:

* The **Send** button.
* The keyboard shortcut `Ctrl+Enter` on Windows and Linux, or `Cmd+Enter` on macOS.

The message appears in the thread immediately and is delivered to the user on their platform.

### What happens when you send a message

When you (an operator) send a message through the compose box, it goes through similar processing to user messages:

1. **Message created** — Your text is stored with metadata (sent by operator, timestamp)
2. **Routed to channel** — OpenLobster uses the conversation's channel info to send via Telegram, Discord, etc.
3. **Delivered** — User receives your message on their platform
4. **Recorded** — Appears in Chat thread with **OPERATOR** label (so you know it came from a human, not the agent)
5. **Memory updated** — Your message is stored in the conversation history (like all messages, per step 11 of the pipeline)

**Important distinction:** Your message is treated as **human-written**, not agent-generated. It doesn't count as "agent activity" — the agent won't learn from it or use it to refine behavior. It's a direct human intervention in the conversation.

### Who sees what

- **You (in Chat view):** See your message with an OPERATOR label
- **User (on their platform):** Sees your message as if it came from the agent/support team (exact labeling depends on platform)
- **Agent (in future messages):** Can see your message in the conversation history but knows it's from a human (metadata tells it apart)

This is useful for operator interventions: manually answering a complex question, correcting something the agent said, or providing information the agent can't access.

## Attaching a file reference

Click the **paperclip icon** to attach a file from your device. This inserts a note into your message with the file's name and size.

{% hint style="warning" %}
Attaching a file does not upload or transfer the file to the user. It adds a text reference to the message so the user knows a file is being referenced. Actual file transfer depends on the capabilities of the destination channel.
{% endhint %}

## Inserting emojis

Click the **smiley face icon** to open the emoji picker and insert an emoji into your message at the cursor position.

## Sending to groups

When the open conversation is a group chat, your message is sent to the entire group — not to a single user. The compose box works identically for direct and group conversations.
