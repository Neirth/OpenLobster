package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ServeHTTP(t *testing.T) {
	deps := &resolvers.Deps{AgentRegistry: registry.NewAgentRegistry()}
	h := NewHandler(deps)
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	assert.True(t, strings.Contains(body, "openlobster_uptime_seconds") ||
		strings.Contains(body, "# HELP openlobster"), "debe incluir métricas Prometheus")
}
