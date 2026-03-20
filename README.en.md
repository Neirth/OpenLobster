# OpenLobster — Personal AI Assistant

<p align="center">
    <picture>
        <source media="(prefers-color-scheme: light)" srcset="https://placehold.co/1600x200/ffffff/000000?text=OpenLobster&font=raleway">
         <img src="https://placehold.co/800x200/0b6e4f/ffffff?text=OpenLobster&font=raleway" alt="OpenLobster" width="800">
    </picture>
</p>

<p align="center">
  <strong>Personal, self-hosted AI assistant — runs where you want, connects to the channels you use.</strong>
</p>

<p align="center">
  <a href="README.en.md">English</a> •
  <a href="README.es.md">Español</a> •
  <a href="README.zh.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/Neirth/OpenLobster/actions/workflows/release.docker-images.yaml?branch=main"><img src="https://img.shields.io/github/actions/workflow/status/Neirth/OpenLobster/release.docker-images.yaml?branch=master&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/Neirth/OpenLobster/releases"><img src="https://img.shields.io/github/v/release/Neirth/OpenLobster?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="https://neirth.gitbook.io/openlobster"><img src="https://img.shields.io/badge/Docs-GitBook-blue?style=for-the-badge" alt="Docs"></a>
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/License-GPLv3-blue.svg?style=for-the-badge" alt="GPLv3 License"></a>
  <a href="https://discord.gg/Qx9eJcZH5v"><img src="https://img.shields.io/badge/Discord-%235865F2.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Discord"></a>
  <a href="https://github.com/Neirth/OpenLobster/discussions"><img src="https://img.shields.io/badge/Q%26A-1E232A?style=for-the-badge&logo=github&logoColor=white" alt="GitHub Discussions"></a>
</p>

> [!NOTE]
> **Migrating from OpenClaw?** A step-by-step migration guide is available in [Discussions #44](https://github.com/Neirth/OpenLobster/discussions/44).


An opinionated fork of OpenClaw that actually addresses the things people have been complaining about since the project blew up.

OpenClaw had a moment — self-hosted AI agent, lots of hype, fast growth. Then the security community took a look and it got ugly fast: a CVE batch that filled a whole page on RedPacket, and a skills marketplace (ClawHub) where 26% of skills had at least one vulnerability. The memory system was a MEMORY.md file that blew up with concurrent sessions. The "scheduler" was a heartbeat daemon that woke up every 30 minutes to read a HEARTBEAT.md checklist. Multi-user support was basically non-existent — the docs literally said "only the main session writes to MEMORY.md, preventing conflicts from parallel sessions" as if that was a feature.

This fork started as a personal fix for all of that and grew from there.

---

## What changed (and why)

* **Memory** — MEMORY.md and a folder of markdown files is not a memory system, it's a wiki. OpenLobster uses a proper graph database (Neo4j) where the agent builds nodes, edges, and typed relationships as it talks to people. You can browse and edit it from the UI. There's also a file backend for local use that doesn't require running Neo4j.

* **Multi-user** — In OpenClaw, curated memory was only loaded in the "main, private session" and never in group contexts. There was no concept of separate users with separate histories. Here, each user across each channel is a proper first-class entity with their own conversation history, their own tool permissions, and their own pairing flow. A Telegram user and a Discord user can talk to the same agent without stepping on each other.

* **Scheduler** — The heartbeat loop reading a HEARTBEAT.md file every 30 minutes is gone. There's a real task scheduler with cron expressions for recurring jobs and ISO 8601 datetimes for one-shot tasks. Status, next-run times, and logs are all visible in the dashboard.

* **MCP** — OpenClaw's MCP integration was essentially a demo. OpenLobster connects to any Streamable HTTP MCP server, handles the full OAuth 2.1 flow, lets you browse tools per server, and gives you a per-user permission matrix so you control exactly what each user can do. There's also a marketplace for one-click integrations.

* **Security** — This is the big one. OpenClaw shipped with authentication disabled by default, which is how you end up with 40,000 exposed instances on Censys. Here, dashboard auth is on by default behind a bearer token (`OPENLOBSTER_GRAPHQL_AUTH_TOKEN`). Config and secrets are encrypted on disk. API keys and channel tokens are stored in a secrets backend (encrypted file or OpenBao), not sitting in plain YAML. `OPENLOBSTER_*` env vars are never leaked to terminal tools. The CVE that let unauthenticated callers hit the agent API directly? Not a thing here.

* **Backend** — OpenClaw was Node.js/TypeScript. The entire backend has been rewritten in Go. That means a single static binary with no runtime dependency, faster startup, lower memory footprint, and a proper GraphQL API via gqlgen. It also makes deployment significantly simpler — no npm, no Node version pinning, no `node_modules` to worry about.

* **UI** — The web interface was built with actual usability in mind. First launch drops you into a setup wizard that gets you from zero to a running agent without touching a config file. Settings are a dynamic form that adjusts based on what you enable — you only see the fields that matter for your setup. Everything you'd otherwise need to edit YAML for is accessible from the browser.

> [!NOTE]
> **Contributors needed** I'm thinking about adding maintainers to this repository. I'm discussing it on [Discussions #68](https://github.com/Neirth/OpenLobster/discussions/68).

## Stack

| Layer | Tech |
| ----- | ---- |
| Frontend | SolidJS + Vite, vanilla CSS |
| Backend | Go, GraphQL (gqlgen) |
| Database | SQLite / PostgreSQL / MySQL |
| Memory | File (GML) or Neo4j |
| Secrets | Encrypted file or OpenBao |
| Channels | Telegram, Discord, WhatsApp, Slack, Twilio SMS |
| AI | OpenAI, Anthropic, Ollama, OpenRouter, Docker Model Runner, OpenAI-compatible |

## Quick start

```bash
# Install dependencies
pnpm install

# Build frontend + backend (frontend embedded into the binary)
pnpm build --filter=@openlobster/backend

# Build only the frontend
pnpm build --filter=@openlobster/frontend

# Build both
pnpm build

# Run
./dist/openlobster
```

The web dashboard will be at `http://127.0.0.1:8080`. On first launch the setup wizard walks you through the essential config.

## Docker

```bash
docker run -p 8080:8080 \
  -e OPENLOBSTER_GRAPHQL_HOST=0.0.0.0 \
  -e OPENLOBSTER_GRAPHQL_AUTH_TOKEN=your-secret-token \
  -v ~/.openlobster/data:/app/data \
  -v ~/.openlobster/workspace:/app/workspace \
  -d ghcr.io/neirth/openlobster/openlobster:latest
```

Check `.docker/` for the available Dockerfiles (`Dockerfile.basic` for a minimal build, `Dockerfile.static` for a fully static binary).

## Configuration

Configuration lives in the dashboard under Settings, but you can inject everything via environment variables with the `OPENLOBSTER_` prefix. Viper maps them automatically (dots in YAML keys become underscores).

```bash
# Minimal example
OPENLOBSTER_AGENT_NAME=my-agent
OPENLOBSTER_DATABASE_DRIVER=sqlite
OPENLOBSTER_DATABASE_DSN=./data/openlobster.db
OPENLOBSTER_GRAPHQL_AUTH_TOKEN=your-secret-token

# AI provider (pick one)
OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT=http://localhost:11434
OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL=llama3.2:latest
```

<details>
<summary>Full environment variable reference</summary>

| Variable | YAML key | Description |
| -------- | -------- | ----------- |
| `OPENLOBSTER_AGENT_NAME` | `agent.name` | Agent display name |
| `OPENLOBSTER_DATABASE_DRIVER` | `database.driver` | `sqlite`, `postgres`, `mysql` |
| `OPENLOBSTER_DATABASE_DSN` | `database.dsn` | Connection string |
| `OPENLOBSTER_DATABASE_MAX_OPEN_CONNS` | `database.max_open_conns` | Max open connections |
| `OPENLOBSTER_DATABASE_MAX_IDLE_CONNS` | `database.max_idle_conns` | Max idle connections |
| `OPENLOBSTER_MEMORY_BACKEND` | `memory.backend` | `file` or `neo4j` |
| `OPENLOBSTER_MEMORY_FILE_PATH` | `memory.file.path` | Path for file backend |
| `OPENLOBSTER_MEMORY_NEO4J_URI` | `memory.neo4j.uri` | e.g. `bolt://localhost:7687` |
| `OPENLOBSTER_MEMORY_NEO4J_USER` | `memory.neo4j.user` | Neo4j username |
| `OPENLOBSTER_MEMORY_NEO4J_PASSWORD` | `memory.neo4j.password` | Neo4j password |
| `OPENLOBSTER_SECRETS_BACKEND` | `secrets.backend` | `file` or `openbao` |
| `OPENLOBSTER_SECRETS_FILE_PATH` | `secrets.file.path` | Path for file secrets |
| `OPENLOBSTER_SECRETS_OPENBAO_URL` | `secrets.openbao.url` | OpenBao server URL |
| `OPENLOBSTER_SECRETS_OPENBAO_TOKEN` | `secrets.openbao.token` | OpenBao auth token |
| `OPENLOBSTER_PROVIDERS_OPENAI_API_KEY` | `providers.openai.api_key` | OpenAI key |
| `OPENLOBSTER_PROVIDERS_OPENAI_MODEL` | `providers.openai.model` | e.g. `gpt-4o` |
| `OPENLOBSTER_PROVIDERS_OPENAI_BASE_URL` | `providers.openai.base_url` | Custom base URL |
| `OPENLOBSTER_PROVIDERS_OPENROUTER_API_KEY` | `providers.openrouter.api_key` | OpenRouter key |
| `OPENLOBSTER_PROVIDERS_OPENROUTER_DEFAULT_MODEL` | `providers.openrouter.default_model` | e.g. `openai/gpt-4o` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT` | `providers.ollama.endpoint` | e.g. `http://localhost:11434` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL` | `providers.ollama.default_model` | e.g. `llama3.2:latest` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_API_KEY` | `providers.ollama.api_key` | Optional auth |
| `OPENLOBSTER_PROVIDERS_ANTHROPIC_API_KEY` | `providers.anthropic.api_key` | Anthropic key |
| `OPENLOBSTER_PROVIDERS_ANTHROPIC_MODEL` | `providers.anthropic.model` | e.g. `claude-sonnet-4-6` |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_API_KEY` | `providers.openaicompat.api_key` | OpenAI-compatible key |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_BASE_URL` | `providers.openaicompat.base_url` | Base URL |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_MODEL` | `providers.openaicompat.model` | Model name |
| `OPENLOBSTER_PROVIDERS_DOCKER_MODEL_RUNNER_ENDPOINT` | `providers.docker_model_runner.endpoint` | DMR endpoint |
| `OPENLOBSTER_PROVIDERS_DOCKER_MODEL_RUNNER_DEFAULT_MODEL` | `providers.docker_model_runner.default_model` | DMR model |
| `OPENLOBSTER_GRAPHQL_ENABLED` | `graphql.enabled` | Enable GraphQL API |
| `OPENLOBSTER_GRAPHQL_PORT` | `graphql.port` | Default `8080` |
| `OPENLOBSTER_GRAPHQL_HOST` | `graphql.host` | Default `127.0.0.1` |
| `OPENLOBSTER_GRAPHQL_BASE_URL` | `graphql.base_url` | Public URL for OAuth callbacks |
| `OPENLOBSTER_GRAPHQL_AUTH_ENABLED` | `graphql.auth_enabled` | Require token for dashboard |
| `OPENLOBSTER_GRAPHQL_AUTH_TOKEN` | `graphql.auth_token` | Bearer token |
| `OPENLOBSTER_LOGGING_LEVEL` | `logging.level` | `debug`, `info`, `warn`, `error` |
| `OPENLOBSTER_LOGGING_PATH` | `logging.path` | Log directory |
| `OPENLOBSTER_WORKSPACE_PATH` | `workspace.path` | Workspace directory |
| `OPENLOBSTER_CONFIG_ENCRYPT` | — | `1` (default) encrypts config on disk, `0` uses plain YAML |

</details>

## Channels

The agent talks to users wherever they already are. Enable any combination in Settings or via env vars.

- **Telegram** — Create a bot with `@BotFather`, paste the token. Works in DMs and groups.
- **Discord** — Create a bot in the Developer Portal, invite it to your server. Works in DMs and channels.
- **Slack** — Socket Mode app with a bot token (`xoxb-`) and an app-level token (`xapp-`).
- **WhatsApp** — WhatsApp Business API via Meta Business Suite.
- **Twilio SMS** — Standard SMS via Twilio.

## User documentation

The `docs/` folder has a full user guide for the web interface — dashboard, chat, memory browser, MCP management, skills, tasks, and settings. Structured for GitBook but works as plain markdown too.

## FAQ

**Does this work with OpenClaw configs?**

No. The architecture is different enough that OpenClaw configs won't map cleanly. Configs are also incompatible — the permission model, the MCP integration, and the way the agent accesses tools have all changed. You'll need to migrate manually.

**Can I run it without Neo4j?**

Yes. Set `OPENLOBSTER_MEMORY_BACKEND=file` and point `OPENLOBSTER_MEMORY_FILE_PATH` at a directory. The file backend stores the graph in GML format locally. It's perfectly usable for personal setups; Neo4j is there when you need multi-instance or want proper graph queries.

### Can I run this on small devices?

Yes. Single Go binary, blazing fast startup.

**Real specs (measured):**
- Startup time: 200ms (vs ~2-3s for Node.js OpenClaw)
- RAM: 30MB with all services loaded (vs ~150MB+ for OpenClaw)
- Binary size: ~66MB (vs 200MB+ for Node.js + node_modules)

Perfect for:
- Raspberry Pi 3/4
- VPS with 512MB RAM
- NAS with tight resources
- Even the $15 LicheeRV Nano (RISC-V)

**Can I use any AI provider?**

OpenAI, Anthropic, Ollama, OpenRouter, Docker Model Runner, and any OpenAI-compatible endpoint are all supported. Configure whichever one you want in Settings or via env vars. You can only have one active provider at a time.

**Is the GraphQL API public?**

By default, yes — the API is open. To protect it, set `OPENLOBSTER_GRAPHQL_AUTH_TOKEN` to a strong secret. Once set, every request to the API and dashboard must include it as a bearer token. If you're exposing the instance on a public IP, do this before you do anything else. The API is intended for the web UI and is not versioned as a public API — it may change between releases.

**I am a company, How do I add an MCP server to the marketplace?**

Open a pull request adding your server to `apps/frontend/public/marketplace.json`. If your company is sponsoring the project, the PR will be reviewed and merged. If not, it stays open — we appreciate the contribution but can't commit to reviewing and maintaining third-party entries without some form of support. MCP servers that were already in the marketplace at launch were added in good faith and are not subject to this policy.

**I want a specific MCP server in the marketplace but I'm not affiliated with it. Can I request it?**

Open an issue and describe what the server does and why it would be useful. If the issue gets significant upvotes from the community, we'll add it for free — no sponsorship required.

**What does "pairing" mean?**

When a user contacts the agent for the first time through any channel, they go through a pairing flow that associates their platform identity (Telegram user ID, Discord user ID, etc.) with an account in OpenLobster. This is what allows per-user permissions, conversation history, and memory to work correctly across channels.

**How do I update?**

Pull the new image or binary and restart. Database migrations run automatically on startup.

## License

See [LICENSE.md](LICENSE.md) for details.
