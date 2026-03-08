---
description: The statistic cards at the top of the Dashboard show the most important numbers at a glance.
icon: chart-bar
---

# Statistic Cards

The row of cards at the top of the Dashboard gives you an instant read on system health and activity. No need to dig into logs or settings for routine status checks — the answer is usually here.

## System health cards

| Card | What it means |
| ---- | ------------- |
| **Health** | `OK` when the system is running normally. If it shows `ERROR` or `KO`, investigate the [Recent Logs](logs.md) panel immediately. |
| **Active Sessions** | Number of conversations that are currently in progress. |
| **MCP Servers** | Number of external tool servers currently connected and available to the agent. |
| **Agent Version** | The version of OpenLobster running on the server. |

## Activity metrics

| Card | What it means |
| ---- | ------------- |
| **Tasks Pending** | Scheduled tasks that are queued and waiting to run. |
| **Tasks Done** | Tasks that have completed successfully. |
| **Messages Received** | Total messages received from users across all channels. |
| **Messages Sent** | Total messages sent by the agent back to users. |

## Diagnosing problems with these cards

{% hint style="warning" %}
If **Health** is not `OK`, check the [Recent Logs](logs.md) panel immediately. Most system-level errors surface there first.
{% endhint %}

* **Messages stuck at zero** — If you expect activity but the message counts are not increasing, verify that your channels are online in the [Channels Panel](channels.md).
* **Tasks Pending keeps growing** — The scheduler may not be running. Check **Settings > Scheduler** to confirm it is enabled.
* **Errors increasing** — Open the logs panel and look for repeating `ERROR` entries to identify the root cause.
