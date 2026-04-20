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

## Which component generates which logs

Understanding which part of the system is logging helps you diagnose faster:

| Component | Typical Log | Level | What It Means |
|-----------|------------|-------|--------------|
| **Adapter** (Telegram, Discord, etc.) | `Telegram adapter connected` | INFO | Channel is online and listening |
| **Adapter** | `Discord adapter: auth failed, token invalid` | ERROR | Bot token expired or wrong — fix in Settings |
| **MessageHandler** | `User pairing validated: user_123` | INFO | User went through pairing flow (first message) |
| **MessageHandler** | `Session not found: user_456` | WARN | User sent message but pairing incomplete |
| **ToolRegistry** | `Tool 'fetch_page' executed in 1.2s` | INFO | A built-in tool ran successfully |
| **ToolRegistry** | `MCP server 'database' request failed: connection timeout` | ERROR | External MCP server is unreachable |
| **AIProvider** | `OpenAI request took 2.3s (200 tokens)` | INFO | API call to model provider completed |
| **AIProvider** | `Rate limit approaching on OpenAI` | WARN | API quotas running low — monitor usage |
| **Scheduler** | `Task 'daily_summary' executed successfully` | INFO | Scheduled task ran and completed |
| **Scheduler** | `Task 'daily_summary' failed: prompt execution timeout` | ERROR | Scheduled task exceeded time limit or errored |
| **Memory** | `Graph updated: 3 nodes added, 2 edges created` | INFO | Knowledge graph being built from conversation |
| **Memory** | `Neo4j connection failed: host unreachable` | ERROR | Memory backend is offline |

## Quick troubleshooting by symptom

| Symptom | What to look for in logs |
|---------|-------------------------|
| "Agent not responding on Telegram" | ERROR from `telegram` adapter component |
| "Tool not executing" | ERROR or WARN from `ToolRegistry` or the specific tool name |
| "Scheduled task didn't run" | ERROR or WARN from `Scheduler` component |
| "Memory not updating" | ERROR from `Memory` or `Neo4j` components |
| "Responses are very slow" | Long response times from `AIProvider` component |
| "MCP server not connecting" | ERROR from the MCP server name in `ToolRegistry` |

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
