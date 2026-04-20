---
description: Skills are capability packages you can install in the agent to give it new behaviours and tools.
icon: graduation-cap
---

# Skills

Skills are packaged capability files (`.skill`) that extend the agent's behaviour. Each skill typically adds one or more tools to the agent's LLM context, enabling it to perform new types of tasks without modifying the core application.

The Skills view shows every skill currently installed in the agent's workspace, with its name and description.

## When to use Skills

* You have a `.skill` file from a trusted provider and want to add a new capability to your agent.
* You want to remove a capability that is no longer needed.
* You want to audit what custom capabilities are currently active.

{% hint style="warning" %}
Only install skills from sources you trust. A skill injects tools into the agent's prompt context, which gives it the ability to act in new ways. Review the skill's contents before installing it in a production environment.
{% endhint %}

## Pages in this section

* [Import & Delete Skills](import-delete.md)
