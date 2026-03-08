---
description: A complete reference for every configuration field in the Settings view, with examples and security guidance.
icon: sliders
---

# Configuration Editor

The Configuration Editor is the main settings panel. Fields are organized into groups. Edit the values and click **Save Changes** to apply. Some fields (provider, memory backend) control the visibility of dependent fields — changing them will show or hide related options dynamically.

{% hint style="info" %}
Only save when you are confident in the values. Changes apply to the running server. For changes to the database driver, memory backend, or GraphQL host/port, a server restart is required.
{% endhint %}

## General Configuration

| Field | Description |
| ----- | ----------- |
| **Agent Name** | Display name for this agent instance. Use letters, numbers, and dashes. Example: `support-agent`. |
| **AI Provider** | The model provider: `openai`, `openrouter`, `ollama`, `anthropic`, `docker-model-runner`. Changing this reveals provider-specific fields. |
| **Default Model** | The model to use for agent tasks. Example: `gpt-4o-mini`, `llama3.2:latest`. |

### Provider-specific fields

{% tabs %}
{% tab title="OpenAI" %}
* **API Key** — Copy from [platform.openai.com](https://platform.openai.com). Treat as a secret.
* **Base URL** — Leave empty unless you are using an enterprise proxy or private endpoint.
{% endtab %}

{% tab title="Anthropic" %}
* **Anthropic API Key** — Copy from [console.anthropic.com](https://console.anthropic.com).
{% endtab %}

{% tab title="Ollama" %}
* **Ollama Host** — URL of the Ollama server, e.g. `http://localhost:11434`.
* **Ollama API Key** — Optional. Required only for protected or remote Ollama instances.
{% endtab %}

{% tab title="OpenRouter" %}
* **API Key** — Copy from your OpenRouter dashboard.
* **Base URL** — Leave empty to use the default OpenRouter endpoint.
{% endtab %}

{% tab title="Docker Model Runner" %}
* **Endpoint** — Docker Model Runner API endpoint, e.g. `http://localhost:12434/engines/v1`.
* **Model** — Model name, e.g. `ai/smollm2`.
{% endtab %}
{% endtabs %}

## Agent Capabilities

Each capability can be toggled on or off. Disabling a capability removes its tools from the agent's context.

| Capability | What it enables | Risk level |
| ---------- | --------------- | ---------- |
| **Browser** | The agent can fetch and browse web pages. | Medium |
| **Terminal** | The agent can execute shell commands on the server. | High |
| **Subagents** | The agent can spawn parallel subagent instances. | Medium |
| **Memory** | The agent reads and writes to the knowledge graph. | Low |
| **MCP** | The agent calls tools from connected MCP servers. | Medium |
| **Audio** | Voice, ASR, and TTS features. | Low |
| **Filesystem** | The agent reads and writes files on the server. | High |
| **Session Interaction** | The agent can inspect and interact with other sessions. | Medium |

{% hint style="warning" %}
**Terminal** and **Filesystem** grant the agent direct access to the server filesystem and shell. Enable these only in isolated, controlled environments.
{% endhint %}

## Database Configuration

| Field | Description |
| ----- | ----------- |
| **Database Driver** | `sqlite` (default for local), `postgres`, or `mysql`. |
| **Database DSN** | Connection string. See examples below. |
| **Max Open Connections** | Maximum simultaneous database connections. `0` = unlimited. Typical production value: 10–50. |
| **Max Idle Connections** | Maximum idle connections kept in the pool. |

<details>
<summary>DSN examples</summary>

**SQLite (local/development):**

```
./data/openlobster.db
```

**PostgreSQL (production):**

```
postgres://user:password@db-host:5432/openlobster?sslmode=require
```

**MySQL:**

```
user:password@tcp(db-host:3306)/openlobster
```

</details>

## Memory Configuration

| Field | Description |
| ----- | ----------- |
| **Memory Backend** | `file` (local filesystem) or `neo4j` (graph database). |
| **File Path** | Path for file-based storage, e.g. `./data/memory`. Ensure the service user has read/write access. |
| **Neo4j URI** | Bolt URI, e.g. `bolt://neo4j-host:7687`. |
| **Neo4j User / Password** | Credentials for the Neo4j instance. |

## Subagents Configuration

| Field | Description |
| ----- | ----------- |
| **Max Concurrent Subagents** | Maximum number of subagents running in parallel. Default: `3`. Reduce on resource-constrained systems. |
| **Default Timeout** | Maximum time a subagent may run. Format: `<number><unit>` — e.g., `5m`, `300s`, `1h`. |

## GraphQL Configuration

| Field | Description |
| ----- | ----------- |
| **GraphQL Enabled** | Set to `false` to disable the GraphQL API entirely. |
| **Port** | Port the server listens on. Default: `8080`. |
| **Host** | Bind address. Default: `127.0.0.1` (localhost only). Set to `0.0.0.0` to accept external connections. |
| **Server Base URL** | Public URL of this OpenLobster instance, e.g. `https://openlobster.example.com`. Required for OAuth callbacks and MCP redirects. |

## Logging Configuration

| Field | Description |
| ----- | ----------- |
| **Log Level** | `debug`, `info`, `warn`, or `error`. Use `debug` only for troubleshooting — it produces very verbose output. |
| **Log Path** | Directory for log files. Default: `./logs`. Ensure the service user has write access. |

## Secrets Configuration

| Field | Description |
| ----- | ----------- |
| **Secrets Backend** | `file` (encrypted local file) or `openbao` (HashiCorp Vault-compatible). |
| **Secrets File Path** | Path to the secrets file, e.g. `./data/secrets.json`. Restrict permissions: `chmod 600`. |
| **OpenBao URL / Token** | URL and authentication token for the OpenBao server. |

## Scheduler Configuration

| Field | Description |
| ----- | ----------- |
| **Scheduler Enabled** | Set to `true` to enable the scheduled task loop. Required for tasks to run. |
| **Memory Consolidation** | When enabled, the agent periodically consolidates and compacts its memory graph. |
| **Consolidation Interval** | How often memory consolidation runs. Format: `<number><unit>` — e.g., `4h`, `30m`. |

## Channel Configuration

Enable and configure the messaging channels the agent listens on.

{% tabs %}
{% tab title="Telegram" %}
1. Open Telegram and search for `@BotFather`.
2. Send `/newbot` and follow the prompts to create a bot.
3. Copy the bot token and paste it into **Telegram Bot Token**.
4. Set **Enable Telegram** to `true` and save.
5. Add the bot to your chats or groups and grant the required permissions.
{% endtab %}

{% tab title="Discord" %}
1. Visit [discord.com/developers/applications](https://discord.com/developers/applications).
2. Create a new Application, then navigate to **Bot** and click **Add Bot**.
3. Copy the bot token and paste it into **Discord Bot Token**.
4. Set **Enable Discord** to `true` and save.
5. Use the OAuth2 URL Generator to invite the bot to your server with the required permissions.
{% endtab %}

{% tab title="Slack" %}
1. Create a Slack App at [api.slack.com/apps](https://api.slack.com/apps).
2. Enable **Socket Mode** and generate an App-Level Token (`xapp-...`).
3. Add the `chat:write` and `channels:history` OAuth scopes, then install the app to your workspace to get a Bot OAuth Token (`xoxb-...`).
4. Enter both tokens in the corresponding fields and set **Enable Slack** to `true`.
{% endtab %}

{% tab title="WhatsApp" %}
1. Set up a WhatsApp Business account via [Meta Business Suite](https://business.facebook.com).
2. Provision a phone number and obtain the **Phone Number ID** and an **API Token**.
3. Enter both values in the corresponding fields and set **Enable WhatsApp** to `true`.
{% endtab %}

{% tab title="Twilio (SMS)" %}
1. Log in to [twilio.com/console](https://www.twilio.com/console).
2. Copy your **Account SID** and **Auth Token**.
3. Enter a Twilio phone number in E.164 format (e.g., `+15551234567`) as the **From Number**.
4. Set **Enable Twilio** to `true` and save.
{% endtab %}
{% endtabs %}

## Security and best practices

{% hint style="danger" %}
Never commit API keys, channel tokens, or secrets to source control. Use environment variables or the secrets backend to manage sensitive credentials.
{% endhint %}

* Restrict permissions on secrets files: `chmod 600 secrets.json`.
* In production, prefer PostgreSQL or MySQL over SQLite and run the database as a separate service.
* Set `OPENLOBSTER_SECRET_KEY` (32-byte Base64 or passphrase) as an environment variable to encrypt config and secrets on disk. If not set, a default key derived from "OpenLobster" is used — acceptable for local development only.
* All `OPENLOBSTER_*` environment variables are never exposed to terminal tools.

## Troubleshooting

**Test the GraphQL API:**

```bash
curl -X POST -H "Content-Type: application/json" \
  --data '{"query":"{health{status}}"}' http://127.0.0.1:8080
```

**Test Neo4j connectivity:**

```bash
nc -zv neo4j-host 7687
```

**Follow live logs:**

```bash
tail -f ./logs/openlobster.log
```
