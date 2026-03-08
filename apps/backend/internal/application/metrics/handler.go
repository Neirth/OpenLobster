package metrics

import (
	"net/http"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler sirve el endpoint /metrics en formato Prometheus.
type Handler struct {
	deps *resolvers.Deps
}

// NewHandler crea el handler HTTP para métricas.
func NewHandler(deps *resolvers.Deps) *Handler {
	return &Handler{deps: deps}
}

// ServeHTTP escribe métricas en formato Prometheus.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := h.deps.Metrics(r.Context())
	if m == nil {
		m = &dto.MetricsSnapshot{}
	}

	reg := prometheus.NewRegistry()
	gauges := []struct {
		name  string
		help  string
		value float64
	}{
		{"openlobster_uptime_seconds", "Agent uptime in seconds.", float64(m.Uptime)},
		{"openlobster_active_sessions", "Number of active messaging sessions.", float64(m.ActiveSessions)},
		{"openlobster_memory_nodes", "Number of nodes in the memory graph.", float64(m.MemoryNodes)},
		{"openlobster_memory_edges", "Number of edges in the memory graph.", float64(m.MemoryEdges)},
		{"openlobster_mcp_tools", "Number of MCP tools currently registered.", float64(m.McpTools)},
		{"openlobster_tasks_pending", "Number of tasks pending execution.", float64(m.TasksPending)},
		{"openlobster_tasks_running", "Number of tasks currently running.", float64(m.TasksRunning)},
		{"openlobster_tasks_done_total", "Total number of tasks completed.", float64(m.TasksDone)},
	}
	for _, g := range gauges {
		gv := prometheus.NewGauge(prometheus.GaugeOpts{Name: g.name, Help: g.help})
		reg.MustRegister(gv)
		gv.Set(g.value)
	}
	counters := []struct {
		name  string
		help  string
		value float64
	}{
		{"openlobster_messages_received_total", "Total number of messages received.", float64(m.MessagesReceived)},
		{"openlobster_messages_sent_total", "Total number of messages sent.", float64(m.MessagesSent)},
		{"openlobster_errors_total", "Total number of errors encountered.", float64(m.ErrorsTotal)},
	}
	for _, c := range counters {
		cv := prometheus.NewCounter(prometheus.CounterOpts{Name: c.name, Help: c.help})
		reg.MustRegister(cv)
		cv.Add(c.value)
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}
