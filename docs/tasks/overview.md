---
description: The task list shows all scheduled jobs with their status, schedule, type, and next run time.
icon: table-list
---

# Tasks Overview

The task list shows every scheduled job in a table. Each row represents one task.

## Table columns

| Column | Description |
| ------ | ----------- |
| **Name** | The task name — typically a short description of what the task does. |
| **Schedule** | An ISO 8601 datetime for one-shot tasks, or a cron expression for cyclic tasks. |
| **Type** | `One-shot` (runs once) or `Cyclic` (repeats on schedule). |
| **Status** | Current runtime state: `pending`, `running`, `done`, or `failed`. |
| **Enabled** | Toggle to pause or resume the task without deleting it. |
| **Next Run** | The formatted datetime of the next scheduled execution (cyclic tasks only). |
| **Actions** | Edit or delete the task. |

## Task states

* **Pending** — The task is scheduled and waiting for its next run time.
* **Running** — The agent is currently executing the task.
* **Done** — The last execution completed successfully.
* **Failed** — The last execution encountered an error. Check the [Recent Logs](../dashboard/logs.md) panel for details.

## Pausing a task

Toggle the **Enabled** switch on any row to pause the task. A paused task remains in the list but will not run until re-enabled. This is useful for temporarily suspending a job without losing its configuration.
