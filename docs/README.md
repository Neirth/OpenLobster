---
description: Welcome to the OpenLobster user guide. Learn how to operate your AI agent from the web interface.
icon: book-open
---

# OpenLobster User Guide

OpenLobster is an open-source AI agent platform that connects to your users through messaging channels like Telegram, Discord, WhatsApp, Slack, and SMS. This guide covers the web interface — the tool operators use to monitor activity, manage conversations, configure the agent, and extend its capabilities.

{% hint style="info" %}
This guide is for end users and operators who interact with the agent through the UI. For installation and developer documentation, refer to the project repository.
{% endhint %}

## What you can do with OpenLobster

<table data-view="cards">
  <thead>
    <tr>
      <th>Section</th>
      <th>Description</th>
      <th data-card-target data-type="content-ref">Link</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>Dashboard</strong></td>
      <td>Monitor system health, active channels, recent conversations, and live logs at a glance.</td>
      <td><a href="dashboard/README.md">Dashboard</a></td>
    </tr>
    <tr>
      <td><strong>Chat</strong></td>
      <td>Read and reply to user conversations, moderate users, and track message history across all channels.</td>
      <td><a href="chat/README.md">Chat</a></td>
    </tr>
    <tr>
      <td><strong>Memory</strong></td>
      <td>Browse, edit, and manage the agent's long-term knowledge graph.</td>
      <td><a href="memory/README.md">Memory</a></td>
    </tr>
    <tr>
      <td><strong>Tools (MCPs)</strong></td>
      <td>Connect external MCP servers, manage built-in capabilities, and control per-user permissions.</td>
      <td><a href="mcps/README.md">MCPs</a></td>
    </tr>
    <tr>
      <td><strong>Skills</strong></td>
      <td>Import and manage skill files that extend the agent's behaviour with reusable capabilities.</td>
      <td><a href="skills/README.md">Skills</a></td>
    </tr>
    <tr>
      <td><strong>Tasks</strong></td>
      <td>Schedule one-shot or recurring jobs that the agent runs automatically.</td>
      <td><a href="tasks/README.md">Tasks</a></td>
    </tr>
    <tr>
      <td><strong>Settings</strong></td>
      <td>Configure the agent, AI provider, database, memory backend, channels, and more.</td>
      <td><a href="settings/README.md">Settings</a></td>
    </tr>
  </tbody>
</table>

## How the agent connects to your users

OpenLobster acts as a backend that sits between your users and an AI model. Users send messages through their preferred platform — Telegram, Discord, WhatsApp, Slack, or SMS — and the agent processes those messages, optionally calling tools, and replies back through the same channel.

The web interface gives you visibility and control over all of this activity without requiring any coding.

## First time setup

If you are opening OpenLobster for the first time, the setup wizard will guide you through the essential configuration steps.

{% stepper %}
{% step %}
## Configure the agent

Enter a name for your agent and set the server URL so the frontend knows where to connect.
{% endstep %}

{% step %}
## Choose an AI provider

Select the model provider (OpenAI, Anthropic, Ollama, OpenRouter, or Docker Model Runner) and enter the required credentials.
{% endstep %}

{% step %}
## Enable communication channels

Turn on the messaging platforms your users will interact through. Each channel requires a bot token from the respective platform.
{% endstep %}

{% step %}
## Select agent capabilities

Enable the features the agent is allowed to use: browser automation, terminal commands, long-term memory, MCP integrations, and more.
{% endstep %}

{% step %}
## Connect MCP servers (optional)

Browse the MCP Marketplace to connect external tool servers that extend the agent's capabilities.
{% endstep %}
{% endstepper %}

After completing the wizard, you can adjust any setting at any time from the [Settings](settings/README.md) view.
