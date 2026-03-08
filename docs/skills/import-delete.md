---
description: Import a new skill package or delete an existing one from the agent.
icon: arrow-up-from-bracket
---

# Import & Delete Skills

## Importing a skill

{% stepper %}
{% step %}

## Click Import Skill

In the Skills view, click the **Import Skill** button. A file picker will open.

{% endstep %}

{% step %}

## Select the skill file

Choose the `.skill` file from your computer. The UI will upload and validate the package automatically.

{% endstep %}

{% step %}

## Confirm success

If the file is valid, the skill appears in the list and a confirmation message is shown. If validation fails, an inline error explains the problem — check that the file is a valid `.skill` package and is compatible with your version of OpenLobster.

{% endstep %}
{% endstepper %}

{% hint style="warning" %}
Only import skills from trusted providers. A skill file can add new tools to the agent's context. In production environments, review the skill contents before importing.
{% endhint %}

## Deleting a skill

1. Find the skill in the list.
2. Click the **Delete** icon on its card.
3. Confirm the deletion in the prompt that appears.

The skill is removed from the runtime immediately. If you notice the agent stops working correctly after a deletion, check the [Recent Logs](../dashboard/logs.md) panel — removing a skill that other capabilities depend on may cause errors.
