// Copyright (c) OpenLobster contributors. See LICENSE for details.

package serve

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/subscriptions"
	"github.com/neirth/openlobster/internal/infrastructure/config"
)

func TestInitHTTP_DisablesGraphQLAndWSWhenGraphQLEndpointsDisabled(t *testing.T) {
	app := newHTTPTestApp(t, false, true, true)

	gqlResp := performRequest(app.Mux, http.MethodPost, "/graphql", `{"query":"query { __typename }"}`)
	if gqlResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("/graphql status = %d, want %d when graphql.enabled=false", gqlResp.Code, http.StatusMethodNotAllowed)
	}

	wsResp := performRequest(app.Mux, http.MethodGet, "/ws", "")
	if wsResp.Code != http.StatusOK {
		t.Fatalf("/ws status = %d, want %d when graphql.enabled=false", wsResp.Code, http.StatusOK)
	}
	if !strings.Contains(strings.ToLower(wsResp.Header().Get("Content-Type")), "text/html") {
		t.Fatalf("/ws content-type = %q, want text/html fallback when graphql.enabled=false", wsResp.Header().Get("Content-Type"))
	}
}

func TestInitHTTP_EnablesGraphQLAndWSWhenGraphQLEndpointsEnabled(t *testing.T) {
	app := newHTTPTestApp(t, true, false, true)

	wsResp := performRequest(app.Mux, http.MethodGet, "/ws", "")
	if wsResp.Code != http.StatusBadRequest {
		t.Fatalf("/ws status = %d, want %d when graphql.enabled=true", wsResp.Code, http.StatusBadRequest)
	}

	gqlResp := performRequest(app.Mux, http.MethodPost, "/graphql", `{"query":"query { __typename }"}`)
	if gqlResp.Code == http.StatusMethodNotAllowed {
		t.Fatalf("/graphql unexpectedly fell back to root handler when graphql.enabled=true")
	}
}

func TestInitHTTP_A2ARoutesRemainIndependentFromGraphQLAndWS(t *testing.T) {
	app := newHTTPTestApp(t, false, true, true)

	cardResp := performRequest(app.Mux, http.MethodGet, "/.well-known/agent-card.json", "")
	if cardResp.Code != http.StatusOK {
		t.Fatalf("A2A card status = %d, want %d when a2a.enabled=true and graphql.enabled=false", cardResp.Code, http.StatusOK)
	}
	if !strings.Contains(cardResp.Body.String(), "protocolVersion") {
		t.Fatalf("A2A card response does not look like an agent card JSON payload")
	}

	gqlResp := performRequest(app.Mux, http.MethodPost, "/graphql", `{"query":"query { __typename }"}`)
	if gqlResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("/graphql status = %d, want %d when graphql.enabled=false", gqlResp.Code, http.StatusMethodNotAllowed)
	}
}

func TestInitHTTP_ServerRootStaysAvailableWhenGraphQLWSAndA2AAreDisabled(t *testing.T) {
	app := newHTTPTestApp(t, false, false, true)

	rootResp := performRequest(app.Mux, http.MethodGet, "/", "")
	if rootResp.Code != http.StatusOK {
		t.Fatalf("/ status = %d, want %d", rootResp.Code, http.StatusOK)
	}

	cardResp := performRequest(app.Mux, http.MethodGet, "/.well-known/agent-card.json", "")
	if cardResp.Code == http.StatusOK {
		t.Fatalf("A2A card endpoint should not be reachable when a2a.enabled=false")
	}
}

func TestInitHTTP_WebFrontendCanBeDisabledWithoutStoppingHTTP(t *testing.T) {
	app := newHTTPTestApp(t, true, true, false)

	rootResp := performRequest(app.Mux, http.MethodGet, "/", "")
	if rootResp.Code != http.StatusNotFound {
		t.Fatalf("/ status = %d, want %d when web.enabled=false", rootResp.Code, http.StatusNotFound)
	}

	staticResp := performRequest(app.Mux, http.MethodGet, "/static/app.js", "")
	if staticResp.Code != http.StatusNotFound {
		t.Fatalf("/static/app.js status = %d, want %d when web.enabled=false", staticResp.Code, http.StatusNotFound)
	}

	healthResp := performRequest(app.Mux, http.MethodGet, "/health", "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want %d when web.enabled=false", healthResp.Code, http.StatusOK)
	}

	gqlResp := performRequest(app.Mux, http.MethodPost, "/graphql", `{"query":"query { __typename }"}`)
	if gqlResp.Code == http.StatusNotFound {
		t.Fatalf("/graphql should remain mounted when graphql.enabled=true even if web.enabled=false")
	}
}

func newHTTPTestApp(t *testing.T, graphqlEnabled bool, a2aEnabled bool, webEnabled bool) *App {
	t.Helper()

	cfg := &config.Config{
		GraphQL: config.GraphQLConfig{
			Enabled: graphqlEnabled,
			Host:    "127.0.0.1",
			Port:    8080,
		},
		Web: config.WebConfig{
			Enabled: webEnabled,
		},
		A2A: config.A2AConfig{
			Enabled: a2aEnabled,
		},
	}

	app := &App{
		Cfg:        cfg,
		PublicFS:   newPublicFSTestData(t),
		SubManager: subscriptions.NewSubscriptionManager(nil),
	}
	app.initHTTP()
	return app
}

func newPublicFSTestData(t *testing.T) fs.FS {
	t.Helper()

	root := t.TempDir()
	staticDir := filepath.Join(root, "public", "static")
	assetsDir := filepath.Join(root, "public", "assets")

	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("mkdir static dir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir assets dir: %v", err)
	}

	indexPath := filepath.Join(assetsDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html><body>OpenLobster test index</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	return os.DirFS(root)
}

func performRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}
