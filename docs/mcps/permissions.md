---
description: The Permissions tab gives you per-user control over which tools the agent may call on behalf of each user.
icon: lock
---

# User Permissions

The **Permissions** tab lets you decide exactly which tools each user can access. This is important for security — you may want some users to have access to powerful tools like browser or terminal, while restricting others to basic conversation only.

## How the permission system works

The default policy is **allow**: the agent can use any enabled tool on behalf of any user unless you explicitly deny it. You create explicit **Deny** entries to restrict access for a specific user.

{% hint style="info" %}
Permission changes take effect immediately and are persisted. There is no need to restart the agent.
{% endhint %}

## Setting permissions for a user

{% stepper %}
{% step %}

## Select a user

In the left panel, click the user whose permissions you want to manage. Users appear here once they have been paired with the agent.

{% endstep %}

{% step %}

## Review the tool list

The right panel shows all available tools, grouped by capability (built-in capabilities first, then tools from each MCP server). Each tool shows its name, server origin, description, and current status.

{% endstep %}

{% step %}

## Toggle individual tools

Click **Deny** next to a tool to block it for this user. The status changes to **Denied**. Click the toggle again to remove the explicit entry and revert to the default (allow) policy.

{% endstep %}
{% endstepper %}

## Bulk operations

Use the **Allow All** or **Deny All** buttons to apply a permission change across all tools at once for the selected user.

{% hint style="warning" %}
Use **Deny All** carefully. It will block the user from accessing every tool, including basic built-in capabilities. You can restore access by clicking **Allow All** or by enabling tools individually.
{% endhint %}

## Service accounts and bot users

Users marked with a `bot` badge are service accounts or agent accounts. Review their permissions separately — they may legitimately need broader tool access than regular users.
