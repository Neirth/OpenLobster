---
description: Edit a node's label, type, or properties, or permanently delete it from the knowledge graph.
icon: pen-to-square
---

# Edit & Delete Nodes

The edit and delete actions are available from the [Node Detail](node-detail.md) panel. Both actions are accessed through modals that require explicit confirmation before changes are applied.

## Why editing nodes matters

When you edit a node, you're directly influencing what the agent knows. During step 4 of the [message processing pipeline](../architecture/processing-messages.md), when context is built, the agent pulls your edited node information. Fixing inaccurate nodes = smarter agent responses.

**Examples of useful edits:**
- **Label**: Change "Acme Inc." to "Acme Corp" (fix company name typo)
- **Type**: Change "Person" to "Organization" (discovered it's actually a team, not a person)
- **Properties**: Add `email: alice@company.com` to a Person node (gives agent more specific contact info to reference)
- **Remove property**: Delete outdated `phone: 555-1234` (replace with updated number)

## Editing a node

{% stepper %}
{% step %}

## Open the edit modal

In the node detail panel, click the **Edit** button (pencil icon). The modal opens with the current label, type, and properties prefilled.

{% endstep %}

{% step %}

## Update the fields

* **Label** — The name the agent uses to reference this entity. Keep it clear and unambiguous (e.g., "Alice Smith" instead of just "Alice").
* **Type** — The category. Change if the entity was miscategorized (discovered a "Person" is actually a "Team").
* **Properties** — Key-value metadata. Add helpful info like `email`, `department`, `location`, `role`. These are searchable and pulled into context during step 4 of the pipeline.

**Property examples:**
- Person: `email`, `phone`, `title`, `department`, `location`
- Organization: `industry`, `size`, `location`, `website`, `headquarters`
- Topic: `status`, `owner`, `deadline`, `priority`

{% endstep %}

{% step %}

## Save

Click **Save**. The update is applied immediately. The next time the agent builds context (step 4), it will see your updated information.

{% endstep %}
{% endstepper %}

## Deleting a node

{% hint style="danger" %}
Node deletion is irreversible. Once confirmed, the node and all its edges are permanently removed from the graph. There is no undo in the UI.
{% endhint %}

{% stepper %}
{% step %}

## Check the graph first

Before deleting, open the [Graph Visualization](graph-visualization.md) tab and inspect the node's connections. If it has many edges, consider whether removing it will affect the agent's ability to recall related information.

{% endstep %}

{% step %}

## Click Delete

In the node detail panel, click the **Delete** button (trash icon). A confirmation modal will appear with a clear warning.

{% endstep %}

{% step %}

## Confirm

Read the warning and click **Confirm delete** to proceed. The node and its edges are removed immediately.

{% endstep %}
{% endstepper %}

{% hint style="info" %}
If you need to remove a large number of nodes, consider exporting or snapshotting the graph data first (using the database backup tools available to your administrator). Bulk operations in the UI should be performed carefully, one node at a time.
{% endhint %}
