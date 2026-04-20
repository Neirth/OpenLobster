---
description: The Graph Visualization tab renders an interactive view of the knowledge graph, showing nodes and the relationships between them.
icon: diagram-project
---

# Graph Visualization

The Graph Visualization tab provides an interactive, visual map of the entire knowledge graph. Nodes appear as circles and relationships appear as labelled edges connecting them. Use this view to understand how the agent's knowledge is structured before making changes.

## Interacting with the graph

* **Click a node** — Selects it and loads its details in the right panel, the same as clicking a node in the sidebar.
* **Hover over an edge** — Shows the relationship name (e.g., `KNOWS`, `WORKS_AT`, `MENTIONED_IN`).
* **Click an edge** — Inspects the relationship between the two connected nodes.
* **Pan and zoom** — Drag to pan the canvas. Use the scroll wheel or pinch gesture to zoom in and out.

## Understanding graph structure

A healthy knowledge graph has these characteristics:

| Pattern | What it means | For your agent |
|---------|---------------|----------------|
| **Dense clusters** | Groups of highly connected nodes (e.g., Alice, Bob, Sales Team, all connected) | Good! The agent can reason about relationships in that area. |
| **Isolated nodes** | A node with 0-1 edges | Likely noise or one-time mention. Safe to delete. |
| **Hub nodes** | One node with many edges (e.g., "Acme Corp" connected to 10+ people) | Important entity. Deleting it breaks a lot of context. |
| **Long chains** | A→B→C→D path without cross-connections | Less useful. Agent needs cross-references to reason about connections. |
| **Duplicate nodes** | Two similar nodes ("Alice" and "Alice Smith") not connected | Problem! Merge them. Your search and context suffer. |

## When to use the graph view

The graph view helps you:

1. **Understand knowledge distribution** — See where the agent has deep vs shallow knowledge
2. **Find duplicates** — Isolated or oddly-named nodes often reveal extraction errors ("Alice" and "Alice Smith" should be one node)
3. **Plan deletions safely** — See how many edges a node has before removing it
4. **Optimize context** — Dense clusters = better agent reasoning; sparse graphs = weak context

{% hint style="warning" %}
Before deleting a node, check it in the graph view. A node with many edges is connected to a large part of the knowledge graph — deleting it may remove context the agent relies on for future conversations.
{% endhint %}

A node that appears isolated (few or no edges) is safer to delete without side effects.

## Performance note

Visualizing very large graphs (1M+ nodes) may be slow or freeze your browser. If that happens:
- Use the sidebar search to find specific nodes instead
- Consider Neo4j backend for better performance (File/GML becomes slow with large graphs)
- Periodic memory consolidation/cleanup can help (Settings > Scheduler Configuration)
