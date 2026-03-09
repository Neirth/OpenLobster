---
description: Create a new scheduled task, edit an existing one, or delete it from the agent.
icon: pen-to-square
---

# Create, Edit & Delete Tasks

## Creating a task

{% stepper %}
{% step %}

## Click New Task

In the Scheduled Tasks view, click the **New Task** button. The task creation modal will open.

{% endstep %}

{% step %}

## Fill in the task details

* **Task Name** — A short, descriptive label (e.g., `Daily sales summary`).
* **Task Type** — Choose between **One-shot** (runs once) or **Cyclic** (repeats on schedule).
* **Schedule** — Enter the schedule for the task:
  * For **One-shot**: an ISO 8601 datetime (e.g., `2026-04-01T09:00:00Z`). Leave empty to run immediately.
  * For **Cyclic**: a cron expression (e.g., `0 8 * * *` for every day at 08:00).
* **Channel** — Select the output channel where the agent will send results.
* **Prompt** — The instruction the agent will execute (e.g., `Summarise the last 24 hours of sales data and post a report`).

{% endstep %}

{% step %}

## Save

Click **Create Task**. The task appears in the list with a `pending` status and the next scheduled run time.

{% endstep %}
{% endstepper %}

{% hint style="warning" %}
If you are not familiar with cron syntax, validate your expression with an external cron tester before saving. An incorrect expression can schedule jobs at unexpected times or prevent them from running at all.
{% endhint %}

## Editing a task

1. Click the **edit** icon on the task row.
2. The edit modal opens with all fields prefilled.
3. Change the name, schedule, channel, or prompt as needed.
4. Click **Save**.

## Task status and monitoring

After creating a task, it appears in the list with status information:

| Status | Meaning | What happens next |
|--------|---------|------------------|
| **pending** | Task created but hasn't run yet (waiting for scheduled time) | Scheduler will execute it at the scheduled time |
| **running** | Task is currently executing (in the 11-step pipeline) | Result will appear when complete |
| **completed** | Task ran successfully | You can see output in Recent Logs and Memory updates reflect any facts extracted |
| **failed** | Task encountered an error during execution | Check Recent Logs for error details; fix prompt or dependencies, then retry |
| **disabled** | Task is toggled off | Won't run until you re-enable it; useful for temporarily pausing recurring tasks |

## How to check task results

Task results appear in several places:

1. **Dashboard > Stat Cards** — "Tasks Done" counter increases when successful
2. **Dashboard > Recent Logs** — Look for entries like `Task 'daily_summary' completed successfully`
3. **Output channel** — The actual task result (report, summary, etc.) is sent there
4. **Memory** — Any facts extracted from the task output are added to the knowledge graph

## Why tasks fail

Tasks execute through the full message processing pipeline (steps 1-11), so they can fail for the same reasons messages fail:

| Failure point | Cause | How to debug |
|---------------|-------|-------------|
| **Step 1-2** | Output channel offline (Telegram bot disconnected, etc.) | Check Dashboard > Channels panel |
| **Step 4-6** | Invalid prompt or missing context | Check if prompt makes sense; verify memory has needed context |
| **Step 7-8** | AI provider error (rate limit, token limit, provider down) | Check Recent Logs for provider errors |
| **Step 9** | Tool error (MCP server down, permission denied, etc.) | Check if MCP servers are online; verify permissions |
| **Step 11** | Database or routing error (output channel can't deliver) | Check Recent Logs for routing errors |

## Deleting a task

1. Click the **delete** icon on the task row.
2. Confirm the deletion in the modal.

Deletion is immediate and cannot be undone. If you only want to stop the task temporarily, use the **Enabled** toggle instead.
