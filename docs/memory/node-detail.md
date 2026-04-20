---
description: The node detail panel shows the full information for a selected memory node, including its properties, connections, and available actions.
icon: circle-info
---

# Node Detail

When you select a node from the sidebar, its complete information loads in the right panel. This is where you read, edit, or delete individual memory entries.

## What is displayed

| Field | Description | Why it matters |
| ----- | ----------- | --------------- |
| **ID** | Unique internal identifier (usually a UUID). Used internally for referencing. | You rarely need this unless troubleshooting. |
| **Label** | Human-readable name (e.g., "Alice", "Q4 Planning", "Acme Corp"). | This is the name the agent uses when remembering the entity. Clear, accurate labels = better agent understanding. |
| **Type** | Category: Person, Organization, Topic, Event, Location, Document, etc. | Helps organize and search memory. The agent uses type to understand context. |
| **Created** | Timestamp when the node was first extracted from a conversation. | Helps you understand knowledge age. Very old timestamps might be outdated. |
| **Properties** | Key-value pairs with structured data (e.g., `email: alice@company.com`, `title: VP Sales`, `department: Revenue`). | These are searched during step 4 of the pipeline when building context. More properties = better agent understanding. |
| **Connections** | Edges (relationships) linking this node to others: "Alice **works_at** Acme Corp", "Acme Corp **located_in** New York". | Shows how this entity relates to the rest of the graph. Broken or incorrect connections degrade agent reasoning. |

### Real example

Selecting the node "Alice" might show:

```
Label: Alice
Type: Person
Created: 2024-01-15 10:30 UTC
Properties:
  - title: Vice President of Sales
  - email: alice@acme.com
  - department: Revenue
  - location: New York

Connections (outgoing):
  → works_at: Acme Corp
  → leads: Sales Team
  → manages: 5 people

Connections (incoming):
  ← reports_to: CEO Bob
  ← knows: Jane (colleague)
```

This tells the agent: "Alice is a VP, works at Acme, is in NYC, and knows Jane." Next time someone asks about Alice, the agent pulls this entire context.

## Navigating connections

Click on a linked node in the Connections section to open that node's detail view directly. This lets you follow chains of related entities without going back to the sidebar.

## Available actions

* **Edit** — Opens the edit modal to update the label, type, or properties. See [Edit & Delete Nodes](edit-delete.md).
* **Delete** — Opens a confirmation modal. The deletion is irreversible. See [Edit & Delete Nodes](edit-delete.md).

{% hint style="info" %}
Edit properties to correct factual errors or add structured keys that make the node easier to find in future searches — for example, adding an `email` or `phone` property to a `Person` node.
{% endhint %}
