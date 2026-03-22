---
description: Browse, inspect, edit, and manage the agent's knowledge graph — everything it knows and learns
icon: brain
---

# Memory

## What is the Memory view?

The Memory view is your window into **everything the agent knows**. As conversations happen, the agent automatically extracts facts and builds a knowledge graph. The Memory view lets you explore, audit, correct, and manage this knowledge.

Think of it as the agent's external brain. When you talk to the agent, it learns things. Memory is where those learned things live, organized and searchable.

## The two-panel interface

**Left sidebar** — A searchable catalog of everything the agent knows:
- Organized by node type (Person, Place, Thing, Story, Fact, etc.)
- Search box to find specific entities
- Counts showing how many nodes of each type

**Right panel** — Deep dive into a selected node:
- Node properties (what the agent knows about it)
- Related nodes (what it connects to)
- Conversation references (where this knowledge came from)
- Actions: edit, delete, or add relationships

**Graph visualization tab** — See how nodes are connected visually. Useful for understanding relationship networks at a glance.

## How does memory actually work?

Here's the flow: User sends message → Agent processes (step 4 of the [message processing pipeline](../architecture/processing-messages.md)) → Agent extracts facts → Facts become nodes in the graph.

For example:

**Conversation:**
> User: "Hi, I'm Alice. I work at Acme Corp in New York, handling Q4 planning."

**What the agent extracts:**
- Node: Person "Alice"
- Node: Thing "Acme Corp"
- Node: Place "New York"
- Node: Thing "Q4 planning"
- Relationships: Alice works_at Acme Corp, Acme Corp located_in New York, Alice handles Q4 planning

**In Memory view:** You search for "Alice" and see all of this automatically mapped out.

This is why the agent can reference things from old conversations — they're stored here. When a new message arrives, the pipeline pulls relevant nodes from the graph to provide context to the AI model.

## When to use Memory

| Task | Reason |
|------|--------|
| **Audit what the agent knows** | See if the agent has accurate information about key customers/users |
| **Find duplicate nodes** | Sometimes "Alice" and "Alice Smith" are the same person; merge them here |
| **Update outdated info** | If a customer changed addresses, update the Place node |
| **Remove stale knowledge** | If a project is cancelled, delete the Thing node |
| **Explore relationships** | Understand how entities are connected |
| **Fix bad extractions** | If the agent misunderstood something, correct it here |

{% hint style="info" %}
The agent builds its memory automatically. You don't need to add entries manually under normal operation — memory grows from conversations.
{% endhint %}

## The automatic knowledge-building process

Here's what happens behind the scenes:

**Real-time extraction** (happens during every message):

1. **Conversation happens** — User talks to agent
2. **Agent processes** — Runs through all 11 steps of message processing
3. **Fact extraction** — In step 11, facts are automatically extracted from the conversation
4. **Graph update** — Nodes and edges are created or updated
5. **Memory updated** — You see new entries in the Memory view

**Consolidation pipeline** (runs on a schedule, e.g. every 4 hours):

1. **Batch collection** — Unvalidated messages are gathered (up to 500 at a time)
2. **Extraction phase** — An LLM extracts candidate facts per user, skipping greetings and transient content
3. **Reduction phase** — Candidate facts are cross-checked against the existing graph; duplicates are discarded
4. **Synchronization phase** — Genuinely new facts are written to the graph with typed nodes (`person`, `place`, `thing`, `story`, `fact`)
5. **Messages marked** — Processed messages are marked validated so they won't be re-processed

This two-track approach means the graph updates incrementally during conversation and is periodically cleaned up and deduplicated by the consolidation pass.

See [Memory Graph System](../architecture/memory-graph.md) for the full technical details of the consolidation pipeline.

## Backend matters (Neo4j vs File)

Your configured memory backend affects how Memory works:

**Neo4j** (production):
- Search is fast even with large knowledge bases
- Supports complex queries
- Safe for multiple instances writing simultaneously

**File/GML** (local development):
- Search is fast for small graphs (< 100k nodes)
- Single-instance only
- Simpler setup

See [Memory Graph System](../architecture/memory-graph.md) for details on choosing a backend.

## Common tasks

* [Browsing the Knowledge Graph](browsing.md) — Find and explore nodes
* [Node Detail](node-detail.md) — Understand properties and relationships
* [Graph Visualization](graph-visualization.md) — See connections visually
* [Edit & Delete Nodes](edit-delete.md) — Correct or remove information

## Troubleshooting

**Memory view is slow?**
- If using File/GML backend: graphs with 100k+ nodes slow down. Switch to Neo4j.
- If using Neo4j: check if indexes are created on frequently searched fields.

**I see duplicate nodes?**
- This happens sometimes when the agent extracts the same entity differently ("Alice" vs "Alice Smith")
- Edit one to point to the other, or delete the duplicate
- See [Edit & Delete Nodes](edit-delete.md)

**Memory isn't growing?**
- Check that the agent is actually having conversations (check Chat view)
- Verify memory backend is configured and accessible (Settings → Memory Backend)
- Check Dashboard logs for extraction errors
