package serve

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	appa2a "github.com/neirth/openlobster/internal/application/a2a"
	"github.com/neirth/openlobster/internal/application/health"
	appmcp "github.com/neirth/openlobster/internal/application/mcp"
	"github.com/neirth/openlobster/internal/application/metrics"
	"github.com/neirth/openlobster/internal/application/webhooks"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/infrastructure/logging"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
)

// initHTTP builds the HTTP mux, registers all routes (GraphQL, WebSocket
// subscriptions, dynamic webhooks, OAuth callback, static assets, health and metrics)
// and creates the http.Server.
func (a *App) initHTTP() {
	cfg := a.Cfg
	a.Mux = http.NewServeMux()

	gqlgenResolver := resolvers.NewResolver(a.Deps)
	gqlgenResolver.SetEventSubscription(&dto.EventSubscriptionAdapter{Eb: a.EventBus})
	gqlgenSrv := generated.NewExecutableSchema(generated.Config{Resolvers: gqlgenResolver})

	if cfg.GraphQL.Enabled {
		a.Mux.HandleFunc("/ws", a.SubManager.HandleWebSocket)
		log.Println("graphql: subscriptions WebSocket at /ws")

		gqlHandler := handler.NewDefaultServer(gqlgenSrv)
		a.Mux.Handle("/graphql", gqlHandler)
		log.Println("graphql: gqlgen handler registered at /graphql")
	} else {
		log.Println("graphql: disabled by config (graphql.enabled=false)")
	}

	healthHandler := health.NewHandler()
	metricsHandler := metrics.NewHandler(a.Deps)
	a.Mux.Handle("/health", healthHandler)
	a.Mux.Handle("/metrics", metricsHandler)

	a.Mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		logger := logging.GetDefaultLogger()
		if logger == nil {
			http.Error(w, "logger not initialized", http.StatusInternalServerError)
			return
		}
		logs, err := logger.GetTailLines(100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(logs))
	})

	webhooks.NewHandler(a.ChanReg, a.MsgHandler).Register(a.Mux)
	if cfg.A2A.Enabled {
		appa2a.NewHandler(a.Cfg, a.Deps).Register(a.Mux)
	} else {
		log.Println("a2a: disabled by config (a2a.enabled=false)")
	}

	a.Mux.HandleFunc("/oauth/callback", a.oauthCallbackHandler)
	log.Println("oauth: /oauth/callback registered")

	if cfg.Web.Enabled {
		staticResourceFS, err := fs.Sub(a.PublicFS, "public/static")
		if err != nil {
			log.Fatalf("failed to create sub-fs for public/static: %v", err)
		}
		a.Mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticResourceFS))))
		log.Println("static: resources served at /static/")

		staticFS, err := fs.Sub(a.PublicFS, "public/assets")
		if err != nil {
			log.Fatalf("failed to create sub-fs for assets: %v", err)
		}
		a.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if strings.Contains(r.URL.Path, ".") {
				http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
				return
			}
			index, err := fs.ReadFile(a.PublicFS, "public/assets/index.html")
			if err != nil {
				log.Printf("failed to read index.html: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(index)
		})
	} else {
		log.Println("web: disabled by config (web.enabled=false)")
	}

	addr := fmt.Sprintf("%s:%d", cfg.GraphQL.Host, cfg.GraphQL.Port)
	a.HTTPServer = &http.Server{
		Addr:         addr,
		Handler:      a.buildCORSHandler(cfg.GraphQL.AuthToken, cfg.GraphQL.AuthEnabled),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

}

// buildCORSHandler wraps the mux with CORS headers and optional Bearer-token
// authentication for /graphql and /logs endpoints.
func (a *App) buildCORSHandler(effectiveToken string, authEnabled bool) http.Handler {
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			protected := strings.HasPrefix(r.URL.Path, "/graphql") || strings.HasPrefix(r.URL.Path, "/logs")
			if !protected || !authEnabled || effectiveToken == "" || (a.Cfg != nil && !a.Cfg.Wizard.Completed) {
				next.ServeHTTP(w, r)
				return
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				token = r.Header.Get("X-Access-Token")
			}
			if token != effectiveToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized","message":"valid access token required"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Access-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		authMiddleware(a.Mux).ServeHTTP(w, r)
	})
}

// oauthCallbackHandler handles the /oauth/callback redirect from OAuth providers.
// On success it re-connects the pending MCP server and registers its tools.
func (a *App) oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	serverName, herr := a.OAuthMgr.HandleCallback(r.Context(), q.Get("code"), q.Get("state"), q.Get("error"))
	if herr != nil {
		log.Printf("oauth callback error: %v", herr)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>OAuth Error</title></head>`+
			`<body><h2>Authorization failed</h2><p>%s</p><p>You may close this window.</p>`+
			`<script>window.opener&&window.opener.postMessage({type:"oauth_error",error:%q},"*");window.close();</script>`+
			`</body></html>`, herr.Error(), herr.Error())
		return
	}
	if serverName != "" {
		if pendingURL, ok := a.OAuthMgr.GetPendingServers()[serverName]; ok {
			go func() {
				ctx := context.Background()
				if err := a.MCPClientSDK.Connect(ctx, mcp.ServerConfig{Name: serverName, Type: "http", URL: pendingURL}); err != nil {
					log.Printf("oauth: auto-reconnect for %q failed: %v", serverName, err)
				} else {
					a.OAuthMgr.RemovePendingServer(serverName)
					_ = a.MCPServerRepo.Save(ctx, serverName, pendingURL)
					if tools := a.MCPClientSDK.GetServerTools(serverName); len(tools) > 0 {
						_ = a.ToolRegistry.RegisterMCP(serverName, a.MCPClientSDK, tools)
						log.Printf("mcp: registered %d tools from %q (oauth callback)", len(tools), serverName)
						appmcp.SyncToolsToRegistry(a.ToolRegistry, a.AgentRegistry)
					}
				}
			}()
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><title>Authorized</title></head>`+
		`<body><h2>Authorization successful</h2><p>You may close this window.</p>`+
		`<script>window.opener&&window.opener.postMessage({type:"oauth_success"},"*");window.close();</script>`+
		`</body></html>`)
}
