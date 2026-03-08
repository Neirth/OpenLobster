---
description: "OpenLobster supports two scheduling formats for tasks: cron expressions for recurring jobs and ISO 8601 datetimes for one-shot jobs."
icon: calendar
---

# Scheduling Formats

OpenLobster supports two formats for scheduling tasks. The format you use depends on whether the task should run once or repeat.

## Cron expressions (cyclic tasks)

Cron expressions define a recurring schedule using five space-separated fields.

```
┌───────── minute (0–59)
│ ┌───────── hour (0–23)
│ │ ┌───────── day of month (1–31)
│ │ │ ┌───────── month (1–12)
│ │ │ │ ┌───────── day of week (0–7, Sunday = 0 or 7)
│ │ │ │ │
* * * * *
```

### Common examples

| Expression | Schedule |
| ---------- | -------- |
| `0 8 * * *` | Every day at 08:00 |
| `0 9 * * 1` | Every Monday at 09:00 |
| `*/30 * * * *` | Every 30 minutes |
| `0 0 1 * *` | First day of every month at midnight |
| `0 12 * * 1-5` | Weekdays at noon |

{% hint style="info" %}
If you are not familiar with cron syntax, use an online cron expression tester to validate before saving. Mistakes can cause tasks to run far more frequently than intended, or not at all.
{% endhint %}

## ISO 8601 datetimes (one-shot tasks)

One-shot tasks run at a specific point in time. Enter the datetime in ISO 8601 format with a UTC timezone suffix (`Z`).

```
2026-04-01T09:00:00Z
```

* **Date**: `YYYY-MM-DD`
* **Time**: `THH:MM:SS`
* **Timezone**: Always use `Z` (UTC) to avoid timezone ambiguity on the server.

Leave the schedule field empty to run the task immediately upon creation.

{% hint style="warning" %}
Always use UTC (`Z`) for one-shot datetimes. The server processes all schedules in UTC. If you enter a local time without a timezone offset, it may execute at an unexpected time.
{% endhint %}
