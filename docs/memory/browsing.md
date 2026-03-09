---
description: Use the sidebar to browse memory nodes by type and search across the entire knowledge graph.
icon: magnifying-glass
---

# Browsing the Knowledge Graph

The left sidebar of the Memory view is your primary navigation tool. It organises all memory nodes by their type and lets you search across the entire graph.

## How nodes are organised

Memory nodes are grouped by their `type` field — automatically extracted by the agent. Common types include:

| Type | What it represents | Example |
|------|-------------------|---------|
| **Person** | Individual people mentioned in conversations | "Alice", "Bob", "John Smith" |
| **Organization** | Companies, teams, groups | "Acme Corp", "Sales Team", "Engineering" |
| **Topic** | Subjects, concepts, project names | "Q4 Planning", "Budget", "API Integration" |
| **Location** | Places, cities, regions | "New York", "San Francisco", "EU" |
| **Event** | Scheduled or mentioned events | "Board Meeting", "Product Launch" |
| **Document** | Files, articles, contracts | "Q4 Report", "Budget Proposal" |

Types are discovered dynamically as the agent extracts knowledge from conversations and displayed in alphabetical order.

To navigate:

1. Expand a type group by clicking its header to reveal nodes it contains
2. Look at the count next to each type (e.g., "Person (42)") to understand your knowledge distribution
3. Click a node name to open its full details in the right panel

**Performance note:** If you have 100k+ nodes, opening large type groups might be slow. This is a sign your memory backend should be Neo4j instead of File. See [Memory Graph System](../architecture/memory-graph.md).

## Searching for a node

The search box at the top of the sidebar filters in real time. You can search by:

* **Node label** (name) — Type "Alice" to find all nodes named Alice
* **Node type** — Type "Person" to filter and show only person nodes
* **Property content** — Type "sales" to find nodes that mention "sales" in their properties

### Search patterns

| What you want | How to search | Example |
|---------------|---------------|---------|
| Find a specific person | Full or partial name | "Alice" or "Bob S" |
| Find all organizations | Type name | "Organization" |
| Find topics related to budgeting | Keyword | "budget" |
| Find Q4-related information | Keyword with wildcards | "Q4" or "quarter" |
| Find recent mentions | Context or date-related info | "January" or "2024" |

{% hint style="info" %}
If many nodes match, include part of the type name to narrow results — e.g., searching "project Marketing" surfaces "Project" nodes containing "Marketing" before less relevant matches.
{% endhint %}

## Why searching memory is important

When a user sends a message (step 4 of the [message processing pipeline](../architecture/processing-messages.md)), the system searches memory for relevant context. The better organized your memory is, the better results the agent gets. If you fix duplicate nodes or update stale information here, future agent responses will be smarter.

## After selecting a node

Once you click a node, its details load in the right panel. From there you can read its properties and connections, edit its data, or delete it. See [Node Detail](node-detail.md) for more.
