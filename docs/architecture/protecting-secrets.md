---
description: How OpenLobster manages and protects secrets in the backend
icon: lock
---

# Secrets protection

Short summary: OpenLobster provides a `SecretsProvider` abstraction and can encrypt configuration on disk using AES‑GCM. The master key is derived from `OPENLOBSTER_SECRET_KEY`.

## Flow and components

- `SecretsProvider` (interface) → implementations: `file` (encrypted `secrets.json`), and a stub `OpenBAOProvider` for vault integrations.
- Configuration: `viper` loads `data/openlobster.yaml` and allows overrides via `OPENLOBSTER_*` environment variables.
- Config encryption: `OLENC1` prefix + AES‑GCM implemented in [apps/backend/internal/infrastructure/config/encrypted.go].

## Operational recommendations (minimum for production)

1. Generate a secure master key:

```bash
export OPENLOBSTER_SECRET_KEY=$(openssl rand -base64 32)
```

2. Enable config encryption:

```bash
export OPENLOBSTER_CONFIG_ENCRYPT=1
```

3. Use `secrets.backend=vault` in production or inject `OPENLOBSTER_SECRET_KEY` from a KMS/secret-provider operator.

4. Never expose secrets via GraphQL/UI — mask `APIKey`/`Token` fields in any configuration snapshots.

## Detected risks

- If `OPENLOBSTER_SECRET_KEY` is not set, the application falls back to a derived default key — this is not recommended for production; provide a secure key via KMS or environment injection.  
- `OpenBAOProvider` is provided as an integration point for vault-style backends; verify and harden its configuration for your deployment or use a managed Vault/KMS for critical workloads.

Code references: [apps/backend/internal/infrastructure/secrets/provider.go](apps/backend/internal/infrastructure/secrets/provider.go), [apps/backend/internal/infrastructure/secrets/file_provider.go](apps/backend/internal/infrastructure/secrets/file_provider.go), [apps/backend/internal/infrastructure/config/encrypted.go](apps/backend/internal/infrastructure/config/encrypted.go)
