package serve

import (
	"context"
	"fmt"
	"log"
	"strings"

	appmcp "github.com/neirth/openlobster/internal/application/mcp"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/neirth/openlobster/internal/infrastructure/secrets"
	"github.com/spf13/viper"
)

// initMCP configures the secrets provider, MCP client SDK and OAuth 2.1
// manager, reconnects saved MCP servers from the database and wires the
// MCP ports into the GraphQL deps struct.
func (a *App) initMCP() {
	cfg := a.Cfg

	// Secrets backend
	var err error
	secretsBackend := strings.ToLower(strings.TrimSpace(cfg.Secrets.Backend))

	switch secretsBackend {
	case "openbao":
		if cfg.Secrets.Openbao == nil || cfg.Secrets.Openbao.URL == "" {
			log.Fatalf("secrets backend is openbao but secrets.openbao.url is not set")
		}
		if cfg.Secrets.Openbao.Token == "" {
			log.Fatalf("secrets backend is openbao but secrets.openbao.token is not set")
		}
		a.SecretsProvider, err = secrets.NewOpenBAOProvider(cfg.Secrets.Openbao.URL, cfg.Secrets.Openbao.Token, "secret")
		if err != nil {
			log.Fatalf("failed to initialize OpenBao secrets provider: %v", err)
		}
		log.Printf("secrets: openbao backend at %s", cfg.Secrets.Openbao.URL)
	default:
		secretsPath := cfg.Secrets.File.Path
		if secretsPath == "" {
			secretsPath = "data/secrets.json"
		}
		if secretsBackend != "" && secretsBackend != "file" {
			log.Printf("secrets: unknown backend %q, using file backend", cfg.Secrets.Backend)
		}
		a.SecretsProvider, err = secrets.NewFileSecretsProvider(secretsPath, config.SecretKey())
		if err != nil {
			log.Fatalf("failed to initialize secrets provider: %v", err)
		}
		log.Printf("secrets: file backend at %s", secretsPath)
	}

	// MCP client SDK
	a.MCPClientSDK = mcp.NewMCPClientSDK(a.SecretsProvider)

	// OAuth 2.1 manager — callback URL is resolved on every InitiateOAuth call so
	// that runtime changes via updateConfig(graphqlBaseUrl) take effect immediately.
	oauthCallbackURLFn := func() string {
		baseURL := viper.GetString("graphql.base_url")
		if baseURL != "" && (strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://")) {
			return strings.TrimSuffix(baseURL, "/") + "/oauth/callback"
		}
		return fmt.Sprintf("http://%s:%d/oauth/callback", cfg.GraphQL.Host, cfg.GraphQL.Port)
	}
	a.OAuthMgr = mcp.NewOAuthManager(a.SecretsProvider, oauthCallbackURLFn)

	// Reconnect saved MCP servers
	if savedServers, err := a.MCPServerRepo.ListAll(context.Background()); err == nil {
		for _, s := range savedServers {
			go func(name, url string) {
				ctx := context.Background()
				if err := a.MCPClientSDK.Connect(ctx, mcp.ServerConfig{Name: name, Type: "http", URL: url}); err != nil {
					log.Printf("mcp: startup reconnect %q failed: %v — marking as pending-auth", name, err)
					a.OAuthMgr.RegisterPendingServer(name, url)
				} else {
					log.Printf("mcp: startup reconnected %q", name)
					if tools := a.MCPClientSDK.GetServerTools(name); len(tools) > 0 {
						_ = a.ToolRegistry.RegisterMCP(name, a.MCPClientSDK, tools)
						log.Printf("mcp: registered %d tools from %q", len(tools), name)
						appmcp.SyncToolsToRegistry(a.ToolRegistry, a.AgentRegistry)
					}
				}
			}(s.Name, s.URL)
		}
	} else {
		log.Printf("mcp: failed to load saved servers: %v", err)
	}

	// Wire MCP ports into deps
	a.Deps.McpConnectPort = &appmcp.ConnectAdapter{
		Client:   a.MCPClientSDK,
		Registry: a.ToolRegistry,
		AgentReg: a.AgentRegistry,
		Repo:     a.MCPServerRepo,
		OAuth:    a.OAuthMgr,
		EventBus: &dto.EventBusAdapter{Eb: a.EventBus},
	}
	a.Deps.McpOAuthPort = &appmcp.OAuthAdapter{OAuth: a.OAuthMgr}
}
