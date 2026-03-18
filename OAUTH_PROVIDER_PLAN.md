# OAuth Provider Authentication for AI Providers

## Goal
Add OAuth-based authentication for AI providers, porting the 5 OAuth flows
from OpenClaw's pi-ai library (TypeScript) to Go. This allows users to
authenticate with their existing subscriptions (ChatGPT Plus, GitHub Copilot,
Claude Pro, Google Gemini, Google Antigravity) instead of paying per-API-call.

## OAuth Providers to Implement

| Provider | Flow Type | Callback Port | Client ID |
|---|---|---|---|
| OpenAI Codex | PKCE + local callback | :1455 | `app_EMoamEEZ73f0CkXaXp7hrann` |
| Anthropic | PKCE + local callback | :53692 | `9d1c250a-e61b-44d9-88ed-5944d1962f5e` |
| GitHub Copilot | Device code (poll) | N/A | `Iv1.b507a08c87ecfe98` |
| Google Gemini CLI | Google Cloud OAuth | TBD | TBD |
| Google Antigravity | Google Cloud OAuth | TBD | TBD |

## Architecture

### New Package: `internal/domain/services/provideroauth/`

```
provideroauth/
├── provider.go          # OAuthProvider interface + registry
├── pkce.go              # Shared PKCE helpers (reuse from mcp/oauth.go)
├── callback_server.go   # Local HTTP callback server for PKCE flows
├── openai_codex.go      # OpenAI Codex OAuth
├── anthropic.go         # Anthropic OAuth
├── github_copilot.go    # GitHub Copilot device code flow
├── google_gemini.go     # Google Gemini CLI OAuth
├── google_antigravity.go # Google Antigravity OAuth
├── manager.go           # ProviderOAuthManager (lifecycle, token refresh)
└── *_test.go            # Tests for each provider
```

### Interface Design

```go
// OAuthProvider defines the contract for an AI provider OAuth flow.
type OAuthProvider interface {
    // ID returns the unique provider identifier (e.g. "openai-codex").
    ID() string
    // Name returns a human-readable name (e.g. "ChatGPT Plus/Pro").
    Name() string
    // Login initiates the OAuth flow and returns credentials.
    Login(ctx context.Context, callbacks LoginCallbacks) (*Credentials, error)
    // RefreshToken refreshes expired credentials.
    RefreshToken(ctx context.Context, creds *Credentials) (*Credentials, error)
    // GetAPIKey extracts the usable API key/token from credentials.
    GetAPIKey(creds *Credentials) string
}

type Credentials struct {
    Access    string `json:"access"`
    Refresh   string `json:"refresh"`
    Expires   int64  `json:"expires"`   // unix ms
    AccountID string `json:"account_id,omitempty"`
    Extra     map[string]string `json:"extra,omitempty"`
}

type LoginCallbacks struct {
    // OnAuth is called with the URL the user must open.
    OnAuth func(url string, instructions string)
    // OnPrompt asks the user for input (e.g. paste redirect URL).
    OnPrompt func(message string) (string, error)
    // OnProgress reports status updates.
    OnProgress func(message string)
}
```

### Manager Design

```go
// ProviderOAuthManager handles OAuth lifecycle for AI providers.
// It stores credentials in the SecretsProvider and auto-refreshes tokens.
type ProviderOAuthManager struct {
    secrets   secrets.SecretsProvider
    providers map[string]OAuthProvider
    mu        sync.RWMutex
    stopCh    chan struct{}
}

// Secret key format: "provider-oauth/{providerID}/credentials"
```

### Integration Points

1. **Config** (`config.go`):
   - Add `AuthMode string` to OpenAIConfig, AnthropicConfig
   - Values: `""` (default=api_key), `"oauth"`, `"api_key"`
   - Add `OAuthProvider string` field for selecting which OAuth provider

2. **Provider wiring** (`main.go: buildAIProviderFromConfig`):
   - When `AuthMode == "oauth"`, load credentials from SecretsProvider
   - Create adapter with access token instead of API key
   - Start background token refresh goroutine

3. **GraphQL** (`schema/config.graphql`):
   - Add `initiateProviderOAuth(provider: String!)` mutation
   - Add `providerOAuthStatus(provider: String!)` query
   - Add `providerOAuthProviders` query (list available providers)
   - Extend `UpdateConfigInput` with `authMode` field

4. **Frontend** (`FirstBootWizard`, `SettingsView`):
   - Add "Login with..." buttons for each OAuth provider
   - Show OAuth status (pending/authorized/error)

## Implementation Steps

### Step 1: Core OAuth package
- [ ] `provider.go` - Interface, Credentials, LoginCallbacks, registry
- [ ] `pkce.go` - PKCE verifier/challenge generation
- [ ] `callback_server.go` - Reusable local HTTP callback server

### Step 2: Provider implementations
- [ ] `openai_codex.go` - OpenAI Codex PKCE flow
- [ ] `anthropic.go` - Anthropic PKCE flow
- [ ] `github_copilot.go` - GitHub Copilot device code flow
- [ ] `google_gemini.go` - Google Gemini CLI OAuth
- [ ] `google_antigravity.go` - Google Antigravity OAuth

### Step 3: Manager + secrets integration
- [ ] `manager.go` - Token storage, refresh loop, credential lifecycle

### Step 4: Config changes
- [ ] Add AuthMode to provider configs
- [ ] Update Validate() for OAuth mode
- [ ] Update buildAIProviderFromConfig() to support OAuth credentials

### Step 5: GraphQL + resolvers
- [ ] Schema additions for OAuth mutations/queries
- [ ] Resolver implementations
- [ ] Wire into main.go deps

### Step 6: Tests
- [ ] Unit tests for each OAuth provider (mock HTTP servers)
- [ ] Unit tests for manager (mock secrets provider)
- [ ] Unit tests for PKCE helpers
- [ ] Integration test for full OAuth flow with mock auth server

## Notes
- Anthropic has been blocking third-party OAuth - may not work in practice
  but we implement it anyway since the flow exists
- Token refresh runs in background goroutine, checks every 60s
- Credentials stored encrypted via SecretsProvider (same as MCP OAuth tokens)
- Each provider is self-contained - easy to add/remove providers later
