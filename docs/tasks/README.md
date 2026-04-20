---
description: Scheduled Tasks let you automate recurring or one-shot jobs that the agent executes automatically, with or without user input.
icon: clock
---

# Scheduled Tasks

The Scheduled Tasks view lets you create, manage, and monitor background jobs that the agent runs automatically. You can schedule a task to run once at a specific time, or set it to repeat on a cron schedule.

Tasks are useful for:

* Sending a daily summary report to a channel.
* Running a periodic check or data retrieval job.
* Triggering a one-off action at a specific datetime without manual intervention.

## How tasks work

When a task runs, the agent executes the task's **prompt** as if a user had sent it — the agent processes the prompt, optionally uses tools, and sends the result to the configured **output channel**.

### The task execution flow

Here's exactly what happens when your task's scheduled time arrives:

1. **Scheduler fires** — The scheduler sees it's time to run a task (e.g., 9 AM for a daily task)
2. **Create loopback message** — The scheduler creates a message from "System" containing your task prompt
3. **Message processing begins** — This message goes through the entire 11-step pipeline (see [Message Processing](../architecture/processing-messages.md))
   - Validation (step 3) — Skipped for system-generated tasks
   - Context building (step 4) — Same as user messages: history, memory, permissions
   - Tool execution (step 9) — If your task needs to fetch data, query an API, etc., tools run here
4. **Response generated** — The agent produces an output
5. **Deliver to output channel** — Result goes to Telegram, Discord, a file, or wherever you specified
6. **Dashboard update** — You see the task result in Recent Logs

**Key insight:** Tasks are not a separate execution path. They go through the exact same pipeline as user messages, which means:

- Tasks have access to ALL tools (browser, filesystem, terminal, MCPs, memory)
- Tasks respect the same permissions system
- Tasks can use memory and reference past conversations
- Task results are saved to the database like any other message

**This is powerful:** A task can do everything a user could ask the agent to do, but on a schedule.

### Example: Daily summary task

You create a task:
- **Prompt:** "Generate a summary of yesterday's conversations about Q4 budget"
- **Schedule:** Daily at 9 AM
- **Output channel:** Telegram (your personal chat)

When 9 AM hits:
1. Scheduler fires
2. Creates message: "Generate a summary of yesterday's conversations about Q4 budget"
3. Message goes through the pipeline:
   - Loads relevant memory nodes about Q4
   - Searches recent conversation history
   - Uses memory search tools if needed (tool execution)
4. Agent generates: "Yesterday, 5 conversations mentioned Q4. Budget discussions included..."
5. Message sent to your Telegram
6. You see it in Recent Logs

If your task needed to query a database or fetch data from an external system, an MCP tool would handle that in step 3.

{% hint style="info" %}
The scheduler must be enabled for tasks to run. Verify that **Scheduler Enabled** is set to `true` in **Settings > Scheduler Configuration**.
{% endhint %}

## Pages in this section

* [Create, Edit & Delete Tasks](create-edit-delete.md)
* [Scheduling Formats](scheduling.md)
