---
description: How the agent builds and maintains knowledge — the memory graph explained for users
icon: database
---

# Memory graph system

## How the agent learns and remembers

OpenLobster doesn't just process messages — it builds a **knowledge graph** that grows with every conversation. This is how the agent remembers facts about users, recognizes patterns, and makes connections across different conversations.

Think of it like a web of connected notes. When you talk to the agent about "my customer Apple and my employee Bob," the agent:

1. **Extracts facts** — "Apple is a customer", "Bob works for me"
2. **Creates nodes** — A node for "Apple", a node for "Bob", a node for "customer"
3. **Connects them** — Links "Apple" → "customer", "Bob" → "my employee"
4. **Reuses knowledge** — The next time you mention Apple, it already knows what Apple is

This happens automatically. You don't need to tell the agent to save things — it's part of step 11 in the [message processing pipeline](processing-messages.md).

## What gets stored in the graph

When conversations happen, the agent extracts:

- **Entities**: People, organizations, locations, topics ("Alice", "Acme Corp", "New York", "Q4 planning")
- **Relationships**: How things connect ("Alice works at Acme", "Acme is in New York", "Q4 planning involves budgets")
- **Attributes**: Details about entities ("Alice is VP of Sales", "Acme has 500 employees")
- **Context**: What prompted this knowledge ("Alice told me in conversation #423 about...")

Each piece of knowledge is a **node** with connections (called **edges**) to other nodes. When you browse the Memory view, you're looking at this graph — the nodes appear on the left (indexed by type), and when you click one, you see its connections.

## Two different backends: choose what fits your needs

### Neo4j (Production)

Neo4j is a real graph database. It's excellent for:

- **Multi-instance deployments** — If you're running multiple OpenLobster instances, they can all safely write to the same Neo4j server
- **Complex queries** — You can ask "show me everyone Alice knows" or "find all decisions made in Q4"
- **Scale** — Handles millions of nodes and relationships
- **Consistency** — ACID transactions mean concurrent writes don't corrupt the graph

Trade-off: Requires running a Neo4j server (Docker, or managed service).

### File/GML (Local)

GML is a file-based graph format. It's great for:

- **Local development** — No external dependencies, just a file
- **Testing** — Simple to set up and tear down
- **Small deployments** — If you have one instance and moderate conversation volume

Limitations: Only one OpenLobster instance can write to it at a time. If you try to run 2 instances writing to the same file, you'll lose data.

## How to choose

| Scenario | Recommendation |
|----------|-----------------|
| Solo personal setup, few conversations per day | File/GML |
| Local development & testing | File/GML |
| Multiple users, production use | Neo4j |
| Multiple OpenLobster instances (high availability) | Neo4j required |
| Very large knowledge base (1M+ nodes) | Neo4j |

## What happens when you look at Memory

In the Memory view, you see:

1. **Left panel** — All nodes, indexed by type (Person, Place, Thing, Story, Fact, etc.)
2. **Right panel** — When you click a node, its details and connected nodes appear
3. **Graph visualization** — A visual representation showing how nodes connect

When you search for something, OpenLobster searches the graph for matching nodes and their relationships.

## Memory growth and space

Each conversation automatically adds new nodes and edges. Over time, your graph grows. This is normal and good — it means the agent is learning more.

**Signs your graph is healthy:**
- You see relevant nodes when you search
- Related facts appear connected
- New conversations reference old knowledge

**Signs something might be off:**
- Duplicate nodes ("Alice" appears multiple times)
- Broken connections (something seems disconnected that should be related)

If you see duplicates, it might mean the agent extracted the same entity differently in different conversations. You can edit/merge nodes in the Memory view.

## Performance notes

**With File/GML:**
- Snapshots happen asynchronously, so you might not see the latest data immediately if you refresh quickly
- Search is fast for small graphs (< 100k nodes), but slows down significantly beyond that

**With Neo4j:**
- Search is fast even with millions of nodes (thanks to indexes)
- Multiple instances can read and write simultaneously
- Backups are straightforward (use Neo4j tools)

If memory operations feel slow, check your backend choice. If you've outgrown file-based storage, switching to Neo4j will make things snappier.

## Memory consolidation pipeline

Beyond real-time extraction (step 11 of the message pipeline), OpenLobster runs a separate **consolidation pass** on a configurable schedule. This is a three-phase Map-Reduce pipeline that processes batches of up to 500 unvalidated messages at a time:

### Phase 1 — Extraction (Map)

Each user's messages are broken into chunks that fit within the AI provider's token budget. An LLM pass extracts candidate persistent facts from each chunk: preferences, habits, personal details, significant events. Greetings, questions, and transient content are skipped. Each chunk produces a short bulleted list of candidate facts, or nothing if there's nothing worth remembering.

### Phase 2 — Reduction (Filter)

The candidate facts from all chunks are combined and sent to a second LLM call that has access to the `search_memory` tool. The model cross-checks every candidate against what is already stored in the graph. Only facts that are **genuinely new or meaningfully different** survive. If every candidate already exists, the phase emits `NO_REPLY` and the pipeline stops — no write happens.

### Phase 3 — Synchronization (Reduce)

The surviving facts are sent to a third LLM call with access to `add_memory` and `set_user_property`. The model writes each fact to the graph with the correct entity type and relationship, then marks the source messages as validated so they won't be processed again.

This pipeline runs as an agentic loop (up to five rounds per phase) so the model can call tools multiple times if needed.

**Configuration** — enable and tune it in Settings under [Scheduler Configuration](../settings/configuration.md#scheduler-configuration):

```yaml
scheduler:
  memory_enabled: true
  memory_interval: 4h
```

### Privacy and logging

User names and identifiers are hashed (SHA-256, first 12 hex chars) in all log lines to avoid PII leakage. By default only a truncated snippet of the prompt is logged at DEBUG level.

Full prompt logging can be enabled for troubleshooting:

```bash
OPENLOBSTER_MEMORY_VERBOSE=1 openlobster serve
```

Use this only in controlled environments — it will write raw conversation content to the log.

## Typed memories

Facts stored in the graph carry a semantic **entity type**. The `add_memory` tool accepts an `entity_type` parameter, and Neo4j creates correspondingly-typed nodes:

| Type | When to use | Neo4j node label |
|------|-------------|-----------------|
| `person` | A specific person: colleague, friend, family member | `Person` |
| `place` | A location: city, country, address, venue | `Place` |
| `thing` | An object, topic, organization, or abstract concept | `Thing` |
| `story` | A narrative event or diary-style entry | `Story` |
| `fact` | Generic facts that don't fit the other categories | `Fact` |

The model chooses the type automatically during the Synchronization phase. In the Memory view, the **left panel** groups nodes by these types, making it easier to navigate large knowledge bases. Deduplication is keyed on `(userID, label, relation)` so facts from different users never overwrite each other.

{% hint style="info" %}
Typed nodes are only created by the consolidation pipeline or via the GraphQL `addMemoryNode` mutation (and updated via `updateMemoryNode`). The File/GML backend stores the type as a node attribute; Neo4j stores it as the node label.
{% endhint %}

## How memory affects agent behavior

This graph feeds directly into the message processing pipeline:

When you send a message, **step 4** of the pipeline pulls relevant facts from the graph and includes them in the context sent to the AI model. This is why the agent can reference things from previous conversations — they're in the graph.

The more connected your graph (good relationships between nodes), the better the context the agent receives, and the smarter it behaves.
