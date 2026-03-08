---
description: Add, manage, and remove external MCP servers to extend the agent's tool set.
icon: server
---

# Managing Servers

The **Servers** tab lists every external MCP server connected to the agent. From here you can add new servers, inspect their tools and status, and remove them when they are no longer needed.

## Server status

| Status | Meaning |
| ------ | ------- |
| **Online** | Connected and tools are available. |
| **Degraded** | Reachable but responding with errors. |
| **Unauthorized** | Connected but requires OAuth authorization. See [OAuth Flow](oauth.md). |
| **Offline** | Cannot be reached. Check the URL and network connectivity. |

## Adding a server

{% stepper %}
{% step %}

## Open the Add Server modal

Click the **Add MCP Server** button. A modal will open with fields for the server name and URL.

{% endstep %}

{% step %}

## Enter the server details

* **Name** — A friendly label for the server (e.g., `My CRM`, `File Search`).
* **URL** — The full HTTPS endpoint of the MCP server (e.g., `https://example-mcp.com/mcp`).
* **API Key** — Optional. Enter an API key if the server requires one for authentication.

{% endstep %}

{% step %}

## Save and connect

Click **Add Server**. The agent will attempt to connect immediately. If the server requires OAuth authorization, the status will show **Unauthorized** — see [OAuth Flow](oauth.md) to complete authorization.

{% endstep %}
{% endstepper %}

{% hint style="info" %}
You can also browse ready-to-connect servers in the **Marketplace**. Click the **Marketplace** button in the Tools view to open it.
{% endhint %}

## Managing an existing server

Click **Manage** on any server row to open its management modal. From there you can:

* View the server's connection status.
* See the full list of tools it exposes.
* Initiate OAuth authorization if required.
* Remove the server entirely.

## Troubleshooting

If tools from a server do not appear as expected:

1. Check the status indicator — if it shows **Offline** or **Degraded**, the server is not reachable.
2. Open the [Recent Logs](../dashboard/logs.md) panel and look for errors mentioning the server name.
3. Verify that the server URL is correct and accessible from the network where OpenLobster is running.
