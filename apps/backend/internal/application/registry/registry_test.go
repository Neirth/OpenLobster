package registry

import (
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentRegistry(t *testing.T) {
	r := NewAgentRegistry()
	require.NotNil(t, r)
	assert.Nil(t, r.GetAgent())
	assert.NotZero(t, r.StartTime())
}

func TestAgentRegistry_UpdateAgent_GetAgent(t *testing.T) {
	r := NewAgentRegistry()
	a := &dto.AgentSnapshot{ID: "1", Name: "Test", Status: "ok"}

	r.UpdateAgent(a)
	got := r.GetAgent()
	require.NotNil(t, got)
	assert.Equal(t, "1", got.ID)
	assert.Equal(t, "Test", got.Name)
}

func TestAgentRegistry_GetChannels_UpdateAgentChannels(t *testing.T) {
	r := NewAgentRegistry()
	assert.Nil(t, r.GetChannels())

	channels := []dto.ChannelStatus{{Type: "telegram", Enabled: true}}
	r.UpdateAgentChannels(channels)
	assert.Len(t, r.GetChannels(), 1)
	assert.Equal(t, "telegram", r.GetChannels()[0].Type)
}

func TestAgentRegistry_UpdateAgentChannels_WithAgent(t *testing.T) {
	r := NewAgentRegistry()
	r.UpdateAgent(&dto.AgentSnapshot{ID: "1", Name: "A", Channels: nil})

	r.UpdateAgentChannels([]dto.ChannelStatus{{Type: "discord", Enabled: true}})
	agent := r.GetAgent()
	require.NotNil(t, agent)
	assert.Len(t, agent.Channels, 1)
	assert.Equal(t, "discord", agent.Channels[0].Type)
}

func TestAgentRegistry_UpdateMCPTools_GetMCPTools(t *testing.T) {
	r := NewAgentRegistry()
	tools := []dto.ToolSnapshot{{Name: "read_file", Description: "Read"}}

	r.UpdateMCPTools(tools)
	got := r.GetMCPTools()
	require.Len(t, got, 1)
	assert.Equal(t, "read_file", got[0].Name)
}

func TestAgentRegistry_UpdateMCPs_GetMCPs(t *testing.T) {
	r := NewAgentRegistry()
	mcps := []dto.MCPSnapshot{{Name: "mcp1", URL: "http://localhost"}}

	r.UpdateMCPs(mcps)
	got := r.GetMCPs()
	require.Len(t, got, 1)
	assert.Equal(t, "mcp1", got[0].Name)
}

func TestAgentRegistry_IncErrors_ErrorsCount(t *testing.T) {
	r := NewAgentRegistry()
	assert.Equal(t, int64(0), r.ErrorsCount())

	r.IncErrors()
	r.IncErrors()
	assert.Equal(t, int64(2), r.ErrorsCount())
}
