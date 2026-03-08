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

## Deleting a task

1. Click the **delete** icon on the task row.
2. Confirm the deletion in the modal.

Deletion is immediate and cannot be undone. If you only want to stop the task temporarily, use the **Enabled** toggle instead.
