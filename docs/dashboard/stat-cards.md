---
description: The statistic cards at the top of the Dashboard show the most important numbers at a glance.
icon: chart-bar
---

# Statistic Cards

The row of cards at the top of the Dashboard gives you an instant read on system health and activity. No need to dig into logs or settings for routine status checks — the answer is usually here.

## System health cards

| Card | What it means | Healthy value | Red flag |
| ---- | ------------- | ------------- | -------- |
| **Health** | Overall system status. `OK` when the system is running normally. If it shows `ERROR` or `KO`, investigate the [Recent Logs](logs.md) panel immediately. | `OK` | `ERROR` or `KO` — check logs immediately |
| **Active Sessions** | Number of conversations that are currently in progress. This is a real-time count of users who have sent at least one message in the last session window. | Depends on use case (1-100+) | Stuck at 0 while users are messaging = channels offline |
| **MCP Servers** | Number of external tool servers currently connected and available to the agent. Shows only servers with active connections. | All configured servers connected | Lower than expected = server down or auth failed |
| **Agent Version** | The version of OpenLobster running on the server. Useful for confirming you're running the expected build. | Should match your deployment | Older than expected = server wasn't restarted after upgrade |

### Understanding Active Sessions

Active Sessions is a real-time metric tied directly to the message processing pipeline:

- When a user sends a message (step 1-2 of the pipeline), a session is created/updated
- The session is "active" while the user has recent message activity
- Sessions go inactive after a period of inactivity (typically configurable)
- Helps you understand current traffic without opening Chat view

**Interpreting the number:**
- If you expect 20 users but see 5: Some channels may be offline, or users haven't messaged recently
- If the number jumps suddenly: Either marketing campaign drove traffic, or there's a flood/DDoS (check logs)
- If it's constantly 0: No messages are arriving; verify channels are online

## Activity metrics

| Card | What it means | Healthy pattern | What it indicates |
| ---- | ------------- | --------------- | ------------------|
| **Tasks Pending** | Scheduled tasks that are queued and waiting to run. This is step 1 of the task scheduler. | Should fluctuate (pending tasks run and clear) | Growing indefinitely = scheduler disabled or stuck |
| **Tasks Done** | Tasks that have completed successfully. Cumulative count since the agent started. | Increasing over time if you have scheduled tasks | Stuck (not increasing) = scheduler disabled or no tasks configured |
| **Messages Received** | Total messages received from users across all channels. Cumulative since agent start. | Should increase as users message | Stuck at 0 = channels aren't receiving messages; stuck = users quiet |
| **Messages Sent** | Total messages sent by the agent back to users. Cumulative since agent start. | Should roughly match or slightly exceed Received (replies + proactive messages) | Stuck = agent can't reach users (routing issue) |

### Understanding message flow metrics

These metrics directly reflect the message processing pipeline:

1. **Messages Received** increments at step 1 (Channel Adapter receives)
2. **Messages Sent** increments at step 11 (Router delivers response)

**What healthy patterns look like:**
- Received = Sent: Most messages get a reply
- Received > Sent: Some messages errored before generating response (check logs for why)
- Sent > Received: Agent initiated messages (proactive tasks, or reconnections)

**Performance insights:**
- If Received is high but Sent is low: Messages are getting stuck in steps 3-10 (pairing validation, AI provider, tool execution). Check Recent Logs for errors.
- If both are low: Check Channels Panel to confirm at least one channel is online.

## Diagnosing problems with these cards

{% hint style="warning" %}
If **Health** is not `OK`, check the [Recent Logs](logs.md) panel immediately. Most system-level errors surface there first.
{% endhint %}

* **Messages stuck at zero** — If you expect activity but the message counts are not increasing, verify that your channels are online in the [Channels Panel](channels.md).
* **Tasks Pending keeps growing** — The scheduler may not be running. Check **Settings > Scheduler** to confirm it is enabled.
* **Errors increasing** — Open the logs panel and look for repeating `ERROR` entries to identify the root cause.
