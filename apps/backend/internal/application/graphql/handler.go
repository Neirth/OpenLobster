package graphql

import (
	"net/http"

	gqlhandler "github.com/99designs/gqlgen/graphql/handler"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
)

// Handler provides utilities for GraphQL (e.g. updating channels).
type Handler struct {
	deps *resolvers.Deps
}

// NewHandler creates the GraphQL handler.
func NewHandler(deps *resolvers.Deps) *Handler {
	return &Handler{deps: deps}
}

// UpdateAgentChannels updates the agent channels in the registry.
func (h *Handler) UpdateAgentChannels(channels []dto.ChannelStatus) {
	h.deps.AgentRegistry.UpdateAgentChannels(channels)
}

// NewGraphQLServer returns an http.Handler that serves the GraphQL API.
// Used by integration and e2e tests.
func NewGraphQLServer(deps *resolvers.Deps) http.Handler {
	r := resolvers.NewResolver(deps)
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: r})
	return gqlhandler.NewDefaultServer(schema)
}
