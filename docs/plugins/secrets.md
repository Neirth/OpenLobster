---
description: "Specification for secret storage and vault plugins in OpenLobster."
icon: key
---

# Secrets Plugins

Secrets plugins provide secure storage for API keys, tokens, and sensitive configuration. They act as a bridge between the OpenLobster Core and external vaults or local encrypted storage.

## Identification

- **Type**: `secrets`
- **Required Functions**: `get`, `set`, `delete`, `list`

---

## Exported Functions

### `get`
Retrieves a secret value by its key.

**Request Parameters:**
```json
{
  "config": { ... },
  "key": "anthropic_api_key"
}
```

**Response:**
```json
{
  "value": "sk-ant-...",
  "found": true,
  "error": null
}
```

### `set`
Persists a secret value.

**Request Parameters:**
```json
{
  "config": { ... },
  "key": "anthropic_api_key",
  "value": "sk-ant-..."
}
```

### `delete`
Removes a secret from storage.

**Request Parameters:**
```json
{
  "config": { ... },
  "key": "anthropic_api_key"
}
```

### `list`
Lists all keys currently stored in the vault or secret backend.

**Request Parameters:**
```json
{
  "config": { ... },
  "prefix": "optional-prefix"
}
```

**Response:**
```json
{
  "keys": ["key1", "key2", ...],
  "error": null
}
```
