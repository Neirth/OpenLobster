---
description: The System Status panel shows the memory backend, secrets backend, and server uptime.
icon: server
---

# System Status Panel

The System Status panel confirms that the core infrastructure components are operating as configured. It is most useful for administrators verifying a deployment and for operators checking whether the system restarted unexpectedly.

## What it shows

| Field | Description | Why it matters | If it's wrong |
| ----- | ----------- | -------------- | ------------- |
| **Memory Backend** | The storage engine for the agent's long-term knowledge graph: `file` (local, single-writer) or `neo4j` (multi-writer database). See [Memory Graph System](../architecture/memory-graph.md). | Affects whether the agent can learn across sessions and how fast memory operations are. File = slower for large graphs, Neo4j = production-ready. | Check that it matches what you configured in Settings. If it says `file` but you configured Neo4j, the server may not have restarted. |
| **Secrets Backend** | Where encrypted credentials live: `file` (encrypted on disk) or `openbao` (HashiCorp Vault-compatible remote vault). | Affects security and multi-instance deployments. File = works for solo setups. OpenBao = enterprise-grade, sharable across instances. | If you configured OpenBao but it shows `file`, either the config didn't save or the server didn't restart. Channel tokens and API keys may not be accessible. |
| **Uptime** | How long the server has been running since its last start. | Quick check that the system is healthy and hasn't crash-restarted. | If uptime is very low (< 1 minute) repeatedly, the process is crashing. Check Recent Logs for what's killing it. |

## Understanding the backends

### Memory Backend

The memory backend is directly tied to **step 4** of the message processing pipeline: when a message arrives, the agent loads context from the memory graph.

- **File**: Works fine for personal deployments, testing, or if you have < 100k facts in your graph. If you try to run multiple instances writing to the same file, they'll corrupt each other.
- **Neo4j**: Recommended for production. Supports concurrent writes, complex queries, and scales to millions of facts. Requires a separate Neo4j server (Docker or managed).

**If this shows the wrong backend:**
- Configuration change didn't apply (server wasn't restarted)
- Restart is failing silently; check Recent Logs
- The file/Neo4j instance isn't accessible after reboot (credentials wrong, server down)

### Secrets Backend

The secrets backend stores sensitive data: channel tokens (Telegram, Discord), AI provider API keys, MCP authentication, database credentials.

- **File**: Encrypted on disk locally. Good enough for solo deployments. Single point of failure (if the server is compromised, secrets are compromised).
- **OpenBao**: Remote vault. Better for teams. Multiple instances can access the same secrets. If OpenBao is down, the agent can't start or authenticate to external services.

**If this shows the wrong backend:**
- Similar to memory: configuration didn't apply or server didn't restart
- If it says `openbao` but the URL is unreachable, the agent may have started but can't access tokens/keys. Check Recent Logs for auth failures.

## Common tasks

### Verify a successful restart

After restarting OpenLobster, check the **Uptime** field. It should show a low value (seconds or minutes). If it shows a large number, the process may not have actually restarted.

### Confirm configuration is applied

After changing the memory or secrets backend in Settings and restarting, confirm here that the values match what you configured. If they do not, the server may have failed to apply the new configuration.

{% hint style="info" %}
The values shown here are read-only. To change the memory or secrets backend, edit the configuration in **Settings > Configuration Editor** and restart the server.
{% endhint %}
