---
description: The Tools view lets you connect external MCP servers, inspect built-in capabilities, and control which tools each user can access.
icon: wrench
---

# Tools (MCP Servers)

The Tools view is where you extend and control the agent's capabilities. It has three tabs:

* **Servers** — External Model Context Protocol (MCP) servers that provide the agent with additional tools.
* **Built-in** — Native capabilities built into the agent: browser, terminal, memory, filesystem, subagents, sessions, and audio.
* **Permissions** — Per-user control over which tools the agent may invoke during a conversation.

## What is MCP?

Model Context Protocol (MCP) is an open standard that lets external services expose tools that AI models can call. By connecting an MCP server, you give the agent access to a new set of capabilities — searching a database, calling an API, reading files from a remote system, and more — without modifying the agent itself.

Think of MCP servers as **plugins** that let the agent do new things. Want the agent to check your calendar? Add a calendar MCP server. Want it to query your SQL database? Add a database MCP server.

{% hint style="info" %}
Only Streamable HTTP transport is supported. If you want to use a stdio-based MCP server, connect it through a bridge such as `mcp-proxy` first.
{% endhint %}

## How tools fit into the message processing pipeline

When you enable an MCP server, here's what happens:

1. **Server connects** — You enter the server URL in Tools → Servers
2. **Tools registered** — OpenLobster contacts the server and loads available tools (e.g., "fetch_calendar", "query_database")
3. **Tools appear in built-in list** — You see them under Tools → Built-in, namespaced (e.g., "calendar:fetch_calendar")
4. **Permissions layer** — You can allow, deny, or ask-before-using each tool per user
5. **Message arrives** — When a user sends a message, step 5 of the message processing pipeline checks permissions
6. **Tool execution** — If the AI model chooses to use a tool and permissions allow it, step 9 executes the tool
7. **Result returned** — The tool output feeds back into the message pipeline

**Key insight:** Tools are available to the agent immediately after step 5 in the message pipeline. The AI model "knows" about them (they're in the context), but won't actually use them unless permissions allow it.

## Built-in vs External tools

| Type | Source | When It Runs | Examples |
|------|--------|--------------|----------|
| **Built-in** | Part of OpenLobster | Locally (fast) | Browser navigation, file read/write, terminal commands, memory search |
| **External (MCP)** | Connected servers | Remote HTTP calls | Calendar access, database queries, CRM lookups, Slack messaging |

## MCP Marketplace

The **Marketplace** button in the Tools view lets you browse and connect pre-configured MCP servers with a single click. This is the fastest way to add common integrations.

## Why permissions matter for tools

Even if a tool is available (server is connected), permissions control whether the agent actually uses it:

**Scenario:** You connect a calendar MCP server, but user Alice is a customer support agent who shouldn't see internal meetings.

- **Allow** — Agent can use this tool without asking
- **Deny** — Agent won't use this tool even if asked
- **Ask** — Agent will ask for permission in the chat before using this tool

This is the three-level permission system in action. See [User Permissions](permissions.md) for details.

## Pages in this section

* [Managing Servers](servers.md)
* [Built-in Capabilities](builtin-capabilities.md)
* [User Permissions](permissions.md)
* [OAuth Flow](oauth.md)
