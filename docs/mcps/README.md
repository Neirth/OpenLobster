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

{% hint style="info" %}
Only Streamable HTTP transport is supported. If you want to use a stdio-based MCP server, connect it through a bridge such as `mcp-proxy` first.
{% endhint %}

## MCP Marketplace

The **Marketplace** button in the Tools view lets you browse and connect pre-configured MCP servers with a single click. This is the fastest way to add common integrations.

## Pages in this section

* [Managing Servers](servers.md)
* [Built-in Capabilities](builtin-capabilities.md)
* [User Permissions](permissions.md)
* [OAuth Flow](oauth.md)
