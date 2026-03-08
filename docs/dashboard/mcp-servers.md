---
description: The MCP Servers panel shows the connection status and tool count for each external tool server connected to the agent.
icon: puzzle-piece
---

# MCP Servers Panel

The MCP Servers panel gives you a quick overview of the external Model Context Protocol (MCP) servers currently connected to your agent. Each server extends the agent's capabilities by exposing additional tools.

## What each row shows

* **Server name** — The friendly name assigned to the server when it was added.
* **Transport badge** — The protocol used to communicate with the server (typically `http`).
* **Tool count** — The number of tools this server makes available to the agent.
* **Status dot** — Connection health: green (Online), amber (Degraded), red (Offline).

## Common tasks

### Verify all servers are connected

After starting or restarting the application, confirm that every expected server shows a green status. An amber or red status means the agent cannot use that server's tools.

### Check available tools

The tool count gives you a rough idea of how many capabilities each server adds. Click **MCPs** in the sidebar for a detailed breakdown of tools per server and their descriptions.

{% hint style="info" %}
To add, remove, or reconfigure MCP servers, go to the [MCPs](../mcps/README.md) view from the sidebar.
{% endhint %}
