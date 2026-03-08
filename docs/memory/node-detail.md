---
description: The node detail panel shows the full information for a selected memory node, including its properties, connections, and available actions.
icon: circle-info
---

# Node Detail

When you select a node from the sidebar, its complete information loads in the right panel. This is where you read, edit, or delete individual memory entries.

## What is displayed

| Field | Description |
| ----- | ----------- |
| **ID** | The unique internal identifier of the node. |
| **Label** | The human-readable name of the node (e.g., a person's name or a topic title). |
| **Type** | The category of the node (e.g., `Person`, `Event`, `Organisation`). |
| **Created** | When the node was added to the graph. |
| **Properties** | A list of key-value pairs containing structured data about the node (e.g., `email: user@example.com`, `phone: +1 555-123`). |
| **Connections** | Outgoing and incoming edges linking this node to others in the graph. Each connection shows the relation name and the linked node's label. |

## Navigating connections

Click on a linked node in the Connections section to open that node's detail view directly. This lets you follow chains of related entities without going back to the sidebar.

## Available actions

* **Edit** — Opens the edit modal to update the label, type, or properties. See [Edit & Delete Nodes](edit-delete.md).
* **Delete** — Opens a confirmation modal. The deletion is irreversible. See [Edit & Delete Nodes](edit-delete.md).

{% hint style="info" %}
Edit properties to correct factual errors or add structured keys that make the node easier to find in future searches — for example, adding an `email` or `phone` property to a `Person` node.
{% endhint %}
