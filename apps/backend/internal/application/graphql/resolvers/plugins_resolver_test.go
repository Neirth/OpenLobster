package resolvers

import (
	"context"
	"fmt"
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPlugin struct {
	id        string
	name      string
	ptype     string
	callFn    func(function string, input []byte) ([]byte, error)
	schema    []byte
	available bool
}

func (m *mockPlugin) ID() string          { return m.id }
func (m *mockPlugin) Name() string        { return m.name }
func (m *mockPlugin) Version() string     { return "test" }
func (m *mockPlugin) Description() string { return "test plugin" }
func (m *mockPlugin) Type() string        { return m.ptype }
func (m *mockPlugin) Schema() ([]byte, error) {
	if len(m.schema) == 0 {
		return []byte(`{"type":"object"}`), nil
	}
	return m.schema, nil
}
func (m *mockPlugin) Call(function string, input []byte) ([]byte, error) {
	if m.callFn != nil {
		return m.callFn(function, input)
	}
	return nil, nil
}
func (m *mockPlugin) Properties() []byte {
	return []byte("{}")
}
func (m *mockPlugin) Close() error { return nil }

type mockPluginRegistry struct {
	plugins map[string]ports.PluginPort
	order   []string
}

func (r *mockPluginRegistry) All() []ports.PluginPort {
	out := make([]ports.PluginPort, 0, len(r.order))
	for _, id := range r.order {
		if p, ok := r.plugins[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

func (r *mockPluginRegistry) Get(id string) ports.PluginPort {
	return r.plugins[id]
}

func (r *mockPluginRegistry) GetByType(pluginType string) []ports.PluginPort {
	out := make([]ports.PluginPort, 0)
	for _, p := range r.plugins {
		if p.Type() == pluginType {
			out = append(out, p)
		}
	}
	return out
}

type mockConfigWriter struct {
	applyCalls int
	lastInput  map[string]interface{}
}

func (w *mockConfigWriter) Apply(_ context.Context, input map[string]interface{}) ([]string, error) {
	w.applyCalls++
	w.lastInput = input
	return nil, nil
}

func TestReloadPlugins_InvokesRuntimeReload(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	plugin := &mockPlugin{id: "openlobster-messages-telegram", name: "tg", ptype: "messaging"}
	deps := &Deps{
		AgentRegistry:  registry.NewAgentRegistry(),
		PluginRegistry: &mockPluginRegistry{plugins: map[string]ports.PluginPort{plugin.id: plugin}, order: []string{plugin.id}},
		ReloadPlugins:  nil,
		ConfigSnapshot: &dto.AppConfigSnapshot{},
	}

	reloadCalls := 0
	deps.ReloadPlugins = func(ctx context.Context) error {
		reloadCalls++
		return nil
	}

	r := NewResolver(deps)
	out, err := r.Mutation().ReloadPlugins(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, reloadCalls)
	require.Len(t, out, 1)
	assert.Equal(t, plugin.id, out[0].ID)
}

func TestUpdatePluginConfig_MessagingTriggersRuntimeReload(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	calledFunctions := make([]string, 0)
	plugin := &mockPlugin{
		id:     "openlobster-messages-telegram",
		name:   "tg",
		ptype:  "messaging",
		schema: []byte(`{"type":"object","properties":{"token":{"type":"string"}},"required":["token"]}`),
		callFn: func(function string, input []byte) ([]byte, error) {
			calledFunctions = append(calledFunctions, function)
			if function == "start" {
				return nil, fmt.Errorf("start should not be called by resolver")
			}
			return nil, nil
		},
	}

	writer := &mockConfigWriter{}
	reloadCalls := 0
	deps := &Deps{
		AgentRegistry:  registry.NewAgentRegistry(),
		PluginRegistry: &mockPluginRegistry{plugins: map[string]ports.PluginPort{plugin.id: plugin}, order: []string{plugin.id}},
		ConfigWriter:   writer,
		ReloadPlugins: func(ctx context.Context) error {
			reloadCalls++
			return nil
		},
	}

	r := NewResolver(deps)
	ok, err := r.Mutation().UpdatePluginConfig(context.Background(), plugin.id, `{"token":"abc"}`)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, writer.applyCalls)
	assert.Equal(t, 1, reloadCalls)
	assert.NotContains(t, calledFunctions, "start")
}
