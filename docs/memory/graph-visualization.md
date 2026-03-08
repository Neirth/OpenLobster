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

## When to use the graph view

The graph view is most useful when you are planning to make structural changes to the knowledge base, such as deleting a node or merging duplicates.

{% hint style="warning" %}
Before deleting a node, check it in the graph view. A node with many edges is connected to a large part of the knowledge graph — deleting it may remove context the agent relies on for future conversations.
{% endhint %}

A node that appears isolated (few or no edges) is safer to delete without side effects.
