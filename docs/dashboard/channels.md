---
description: The Channels panel shows the connection status of every messaging platform configured in OpenLobster.
icon: plug
---

# Channels Panel

The Channels panel lists every messaging platform you have connected — Telegram, Discord, WhatsApp, Slack, Twilio SMS, and any others you have configured. Each entry shows whether that channel is currently able to send and receive messages.

## Status indicators

| Status | Color | Meaning |
| ------ | ----- | ------- |
| **Online** | Green | The channel is connected and working normally. |
| **Degraded** | Amber | The channel is reachable but experiencing intermittent failures or delays. |
| **Offline** | Red | The channel is disconnected. The agent cannot send or receive messages through it. |

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
