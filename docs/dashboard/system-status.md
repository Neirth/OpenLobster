---
description: The System Status panel shows the memory backend, secrets backend, and server uptime.
icon: server
---

# System Status Panel

The System Status panel confirms that the core infrastructure components are operating as configured. It is most useful for administrators verifying a deployment and for operators checking whether the system restarted unexpectedly.

## What it shows

| Field | Description |
| ----- | ----------- |
| **Memory Backend** | The storage engine used for the agent's long-term memory: `file` (local filesystem) or `neo4j` (graph database). |
| **Secrets Backend** | Where the application stores sensitive credentials: `file` (encrypted local file) or `openbao` (HashiCorp-compatible vault). |
| **Uptime** | How long the server has been running since its last start. |

## Common tasks

### Verify a successful restart

After restarting OpenLobster, check the **Uptime** field. It should show a low value (seconds or minutes). If it shows a large number, the process may not have actually restarted.

### Confirm configuration is applied

After changing the memory or secrets backend in Settings and restarting, confirm here that the values match what you configured. If they do not, the server may have failed to apply the new configuration.

{% hint style="info" %}
The values shown here are read-only. To change the memory or secrets backend, edit the configuration in **Settings > Configuration Editor** and restart the server.
{% endhint %}
