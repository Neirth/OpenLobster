---
description: The Channels panel shows the connection status of every messaging platform configured in OpenLobster.
icon: plug
---

# Channels Panel

The Channels panel lists every messaging platform you have connected — Telegram, Discord, WhatsApp, Slack, Twilio SMS, and any others you have configured. Each entry shows whether that channel is currently able to send and receive messages.

## Status indicators

| Status | Color | Meaning | What it means for users |
| ------ | ----- | ------- | ---------------------- |
| **Online** | Green | The channel adapter is connected and working normally. Messages flow freely in both directions. | Messages are received and replied to normally |
| **Degraded** | Amber | The channel is reachable but experiencing intermittent failures, rate limiting, or delays. Some messages may fail or take longer. | Users may see delayed responses or occasional errors; reconnections happening |
| **Offline** | Red | The channel adapter is disconnected. Unable to send or receive messages. | Users send messages but agent never responds; looks like the agent is broken/missing |

### What happens when a channel is Offline

When a channel goes **Offline**, the message processing pipeline is broken at step 1 (Channel Adapter reception):

1. User sends message on Telegram/Discord/etc.
2. Channel Adapter cannot receive it (not connected)
3. Step 1 fails → entire pipeline stops
4. User message never reaches OpenLobster
5. Agent never generates a response
6. User waits indefinitely with no reply

**Result:** Users think the agent is down, even if the rest of the system is healthy.

### What's the difference between Degraded and Offline?

| Degraded | Offline |
|----------|---------|
| Adapter can reach the platform but is experiencing issues | Adapter cannot reach the platform at all |
| Some messages get through, some don't | No messages get through |
| Temporary network hiccup, rate limiting, or auth expiring | Bot token revoked, network down, or platform unavailable |
| Usually resolves itself within minutes | Requires manual intervention (token update, network fix, etc.) |
| User might see occasional "Message not sent" | User sees nothing (silence) |

## What each row shows

* **Icon and name** — The platform icon and the channel name help you identify it at a glance.
* **Status badge** — A colored dot and label reflect the current connection state.

## Troubleshooting a channel

{% hint style="danger" %}
If a channel shows **Offline**, users on that platform will not receive any responses from the agent. Fix the issue before it affects your users.
{% endhint %}

{% stepper %}
{% step %}

## Check the status here

If a user reports the agent is not responding on a specific platform, look at this panel first. An **Offline** status confirms the problem is on the channel level.

{% endstep %}

{% step %}

## Review the logs

Open the [Recent Logs](logs.md) panel and look for `ERROR` entries related to the channel name. Common causes include an expired or revoked bot token, a network timeout, or a misconfigured webhook.

{% endstep %}

{% step %}

## Update the configuration

Go to **Settings > Communication Channels**, correct the token or configuration, and save. The channel should reconnect automatically within a few seconds.

{% endstep %}
{% endstepper %}

{% hint style="info" %}
A **Degraded** status often indicates a temporary network issue or rate limiting from the platform. If it persists for more than a few minutes, treat it the same as an **Offline** status and investigate.
{% endhint %}

To add or remove channels, or to update tokens and credentials, go to [Settings > Configuration](../settings/configuration.md#channels).
