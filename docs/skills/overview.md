---
description: The Skills view lists all installed capability packages and shows their name and description.
icon: list-check
---

# Skills Overview

The Skills view lists every skill currently installed in the agent's workspace. Each entry shows the skill's name and a short description of what it adds to the agent.

## What to check before acting

* **Name and description** — Confirm the skill does what you expect before enabling or keeping it.
* **Provenance** — Recall where the skill came from. If you do not recognise a skill, investigate before leaving it installed.

## Capabilities section

Below the skills list, the **Capabilities** section shows which built-in capability groups are currently enabled and injected into the agent's LLM prompt context. This is a read-only view — to change which capabilities are active, go to **Settings > Agent Capabilities**.

{% hint style="info" %}
Enabled capabilities are injected as tools into the agent's LLM prompt context. Disabling a capability removes those tools from the model's awareness, preventing it from using them.
{% endhint %}
