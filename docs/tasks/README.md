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

{% hint style="info" %}
The scheduler must be enabled for tasks to run. Verify that **Scheduler Enabled** is set to `true` in **Settings > Scheduler Configuration**.
{% endhint %}

## Pages in this section

* [Create, Edit & Delete Tasks](create-edit-delete.md)
* [Scheduling Formats](scheduling.md)
