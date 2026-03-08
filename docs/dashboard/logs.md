---
description: The Recent Logs panel streams live log output from the backend, making it easy to spot errors and monitor activity in real time.
icon: terminal
---

# Recent Logs Panel

The Recent Logs panel shows a live feed of messages from the OpenLobster backend. It updates in real time as the agent processes conversations, runs tasks, and connects to external services.

## Log levels

| Level | Meaning |
| ----- | ------- |
| **INFO** | Normal operational messages. The agent is working as expected. |
| **WARN** | Something unexpected happened, but the system is still running. Worth investigating if it repeats. |
| **ERROR** | A failure occurred. Action is likely required. |

## Diagnosing problems

When something is not working — a channel is offline, a task failed, a tool call returned an error — this panel is the fastest place to find the reason.

{% stepper %}
{% step %}

## Identify the symptom

Note which feature is not working: a specific channel, a tool call, a scheduled task.

{% endstep %}

{% step %}

## Scan the logs

Look for `ERROR` or `WARN` entries. The message text usually names the component that failed (e.g., `discord`, `mcp`, `scheduler`).

{% endstep %}

{% step %}

## Act on the error

Common fixes include updating a bot token in **Settings > Communication Channels**, correcting an MCP server URL in the **MCPs** view, or checking network connectivity to an external service.

{% endstep %}
{% endstepper %}

{% hint style="info" %}
This panel only shows the most recent log entries. For the complete log history, access the log files directly on the server at the path configured in **Settings > Logging Path** (default: `./logs`).
{% endhint %}
