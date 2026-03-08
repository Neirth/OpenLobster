---
description: Edit a node's label, type, or properties, or permanently delete it from the knowledge graph.
icon: pen-to-square
---

# Edit & Delete Nodes

The edit and delete actions are available from the [Node Detail](node-detail.md) panel. Both actions are accessed through modals that require explicit confirmation before changes are applied.

## Editing a node

{% stepper %}
{% step %}

## Open the edit modal

In the node detail panel, click the **Edit** button (pencil icon). The modal opens with the current label, type, and properties prefilled.

{% endstep %}

{% step %}

## Update the fields

* **Label** — Change the human-readable name of the node.
* **Type** — Change the category the node belongs to.
* **Properties** — Edit existing key-value pairs inline, remove a property by clicking its remove button, or add a new property by clicking **Add property** and entering a key and value.

{% endstep %}

{% step %}

## Save

Click **Save**. A saving indicator confirms the update is being applied. Wait for the success message before closing the modal.

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
