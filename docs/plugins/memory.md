---
description: "Specification for memory and knowledge graph plugins in OpenLobster."
icon: database
---

# Memory Plugins

Memory plugins provide storage and retrieval for OpenLobster's knowledge graph and long-term memory. They are responsible for persisting entities, relationships, and embeddings.

## Identification

- **Type**: `memory`
- **Required Functions**: `store`, `retrieve`, `query`

---

## Exported Functions

### `store`
Saves information into the long-term memory or knowledge graph. This function handles creating nodes, updating properties, and establishing relationships.

**Request Parameters:**
```json
{
  "config": { ... },
  "op": "add_relation|delete_relation|edit_node|delete_node|...",
  "user_id": "user-123",
  "content": "Node content",
  "label": "EntityLabel",
  "relation": "RELATION_TYPE"
}
```

### `retrieve`
Performs semantic search or direct lookup of memory entries.

**Request Parameters:**
```json
{
  "config": { ... },
  "query": "search term or vector",
  "limit": 10
}
```

### `query`
Executes advanced queries against the knowledge graph, such as retrieving a full user subgraph or executing Cypher-like queries.

**Request Parameters:**
```json
{
  "config": { ... },
  "user_id": "user-123",
  "op": "user_graph|cypher",
  "cypher": "MATCH (n) RETURN n LIMIT 10"
}
```

---

## Technical Considerations

1.  **Atomicity**: Memory plugins should ensure that graph operations (like creating a node and its relationships) are atomic where possible.
2.  **Embeddings**: For vector-capable memory plugins, the `store` function may receive an `embedding` array of floats to facilitate semantic retrieval.
3.  **Isolation**: Data should be isolated by `user_id` to ensure privacy and prevent knowledge leakage between users.
