# Memory Plugins
Memory plugins provide storage and retrieval for OpenLobster's knowledge graph and long-term memory.
## Identification
- **Type**: `memory`

## Core Methods

### `get_info`
Returns general plugin metadata.

---

## Exported Functions

### `store`
Saves information into the long-term memory.

**Request (`params`):**
```json
{
  "collection": "users",
  "id": "user123",
  "data": { "key": "value" }
}
```

## Dispatch Mode (`call` Method)
Memory plugins receive commands via the **`call`** method with one of the following `function` names:
- `get_entity`: Retrieve an entity or relationship.
- `set_entity`: Persist an entity or relationship.
- `query_graph`: Execute a graph query.
- `delete_entity`: Remove data.
- `configure_plugin`: Update settings.
