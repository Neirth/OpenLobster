package resolvers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type routingMockPlugin struct {
	mock.Mock
}

func (m *routingMockPlugin) ID() string             { return m.Called().String(0) }
func (m *routingMockPlugin) Name() string           { return m.Called().String(0) }
func (m *routingMockPlugin) Version() string        { return m.Called().String(0) }
func (m *routingMockPlugin) Description() string    { return m.Called().String(0) }
func (m *routingMockPlugin) Type() string           { return m.Called().String(0) }
func (m *routingMockPlugin) Schema() ([]byte, error) { args := m.Called(); return args.Get(0).([]byte), args.Error(1) }
func (m *routingMockPlugin) Call(fn string, in []byte) ([]byte, error) {
	args := m.Called(fn, in)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *routingMockPlugin) Close() error      { return m.Called().Error(0) }
func (m *routingMockPlugin) Properties() []byte { return m.Called().Get(0).([]byte) }

type routingMockRegistry struct {
	mock.Mock
}

func (m *routingMockRegistry) All() []ports.PluginPort             { return m.Called().Get(0).([]ports.PluginPort) }
func (m *routingMockRegistry) Get(id string) ports.PluginPort      { return m.Called(id).Get(0).(ports.PluginPort) }
func (m *routingMockRegistry) GetByType(t string) []ports.PluginPort { return m.Called(t).Get(0).([]ports.PluginPort) }
func (m *routingMockRegistry) Register(p ports.PluginPort)         { m.Called(p) }
func (m *routingMockRegistry) Remove(id string)                    { m.Called(id) }
func (m *routingMockRegistry) Close()                              { m.Called() }

type routingMockConfigWriter struct {
	mock.Mock
}

func (m *routingMockConfigWriter) Apply(ctx context.Context, input map[string]interface{}) ([]string, error) {
	args := m.Called(ctx, input)
	return args.Get(0).([]string), args.Error(1)
}

func TestUpdatePluginConfigRouting(t *testing.T) {
	viper.Reset()
	ctx := context.Background()

	p := new(routingMockPlugin)
	p.On("ID").Return("telegram")
	p.On("Type").Return("messaging")
	p.On("Schema").Return([]byte("{}"), nil)

	registry := new(routingMockRegistry)
	registry.On("Get", "telegram").Return(p)

	writer := new(routingMockConfigWriter)

	resolver := &mutationResolver{
		Resolver: &Resolver{
			Deps: &Deps{
				PluginRegistry: registry,
				ConfigWriter:   writer,
			},
		},
	}

	// Test data
	config := map[string]interface{}{
		"bot_token": "secret_token",
		"enabled":   true,
	}
	configJSON, _ := json.Marshal(config)

	// Expected: routed to channels.telegram
	writer.On("Apply", ctx, mock.MatchedBy(func(input map[string]interface{}) bool {
		return input["channels.telegram.bot_token"] == "secret_token" && input["channels.telegram.enabled"] == true
	})).Return([]string{"telegram"}, nil)

	success, err := resolver.UpdatePluginConfig(ctx, "telegram", string(configJSON))
	assert.NoError(t, err)
	assert.True(t, success)

	writer.AssertExpectations(t)
}

func TestPluginConfigJSONRecycling(t *testing.T) {
	viper.Reset()
	viper.Set("providers.openai.api_key", "sk-12345")
	viper.Set("providers.openai.model", "gpt-4")

	jsonStr := pluginConfigJSON("ai", "openai")
	
	var cfg map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &cfg)
	assert.NoError(t, err)
	
	assert.Equal(t, "sk-12345", cfg["api_key"])
	assert.Equal(t, "gpt-4", cfg["model"])
}
