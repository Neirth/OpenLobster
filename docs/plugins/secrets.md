# Secrets Plugins
Secrets plugins provide secure storage for API keys, tokens, and sensitive configuration.
## Identification
- **Type**: `secrets`
## Dispatch Mode (`Call`)

### `get_info`
Returns general plugin metadata.

---

## Exported Functions

### `get`
Retrieves a secret.

**Request (`params`):**
```json
{
  "key": "api_key"
}
```
- `set`: Store a secret value.
- `delete`: Remove a secret.
- `list`: List available secret keys.
- `configure`: Update settings.
