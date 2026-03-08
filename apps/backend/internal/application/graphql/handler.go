package graphql

import (
	"net/http"

	gqlhandler "github.com/99designs/gqlgen/graphql/handler"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
)

// Handler proporciona utilidades para GraphQL (p. ej. actualizar canales).
type Handler struct {
	deps *resolvers.Deps
}

// NewHandler crea el handler para GraphQL.
func NewHandler(deps *resolvers.Deps) *Handler {
	return &Handler{deps: deps}
}

// UpdateAgentChannels actualiza los canales del agente en el registry.
func (h *Handler) UpdateAgentChannels(channels []dto.ChannelStatus) {
	h.deps.AgentRegistry.UpdateAgentChannels(channels)
}

// NewGraphQLServer devuelve un http.Handler que sirve la API GraphQL.
// Usado por tests de integración y e2e.
func NewGraphQLServer(deps *resolvers.Deps) http.Handler {
	r := resolvers.NewResolver(deps)
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: r})
	return gqlhandler.NewDefaultServer(schema)
}
