---
description: The MCP Servers panel shows the connection status and tool count for each external tool server connected to the agent.
icon: puzzle-piece
---

# MCP Servers Panel

The MCP Servers panel gives you a quick overview of the external Model Context Protocol (MCP) servers currently connected to your agent. Each server extends the agent's capabilities by exposing additional tools.

## What each row shows

| Field | What it means | Why it matters |
|-------|---------------|----------------|
| **Server name** | The friendly name assigned to the server when it was added (e.g., "database", "slack-tools", "calendar"). | Helps you identify which server is which at a glance. |
| **Transport badge** | The protocol used to communicate: typically `http` or `https` for Streamable servers. | Shows whether the connection is secure (https) or not. |
| **Tool count** | The number of tools this server exposes (e.g., "5 tools"). | Higher count = more capabilities. If count is 0, the server connected but has no tools. |
| **Status dot** | Green (Online), amber (Degraded), or red (Offline). | Directly affects what the agent can do during step 5-9 of message processing. |

## Status meanings

### Online (Green)
- MCP server is reachable and responding
- Agent can see and use all its tools
- Tools will execute when the AI model requests them

### Degraded (Amber)
- Server is reachable but experiencing intermittent issues
- **Some** tool requests may fail
- Agent can still "see" the tools (they appear in context), but execution is unreliable
- Check server logs or network connectivity; this often resolves automatically

### Offline (Red)
- Server is unreachable (network down, wrong URL, auth failed, server crashed)
- Agent cannot use this server's tools
- If the AI model tries to use these tools anyway, the request will fail (but not crash the agent)

## How MCP servers affect message processing

During **step 5-9** of the [message processing pipeline](../architecture/processing-messages.md):

1. **Step 5** (Find tools): The agent loads available tools, including those from all Online/Degraded MCP servers
2. **Step 6** (Check permissions): User permissions are applied (some tools may be denied)
3. **Step 7-8** (AI decides): The model sees available tools in its context and decides whether to use them
4. **Step 9** (Execute): If a tool is from an MCP server and it's Online, the HTTP call is made. If Degraded or Offline, it fails.

**Result:** If all your MCP servers are Offline, the agent loses a lot of functionality. High-value tools (database queries, API integrations) become unavailable.

## Common tasks

### Verify all servers are connected

After starting or restarting the application, confirm that every expected server shows a green status. An amber or red status means the agent cannot use that server's tools.

### Check available tools

The tool count gives you a rough idea of how many capabilities each server adds. Click **MCPs** in the sidebar for a detailed breakdown of tools per server and their descriptions.

{% hint style="info" %}
To add, remove, or reconfigure MCP servers, go to the [MCPs](../mcps/README.md) view from the sidebar.
{% endhint %}
