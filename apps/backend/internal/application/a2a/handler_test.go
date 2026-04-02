package a2a

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAIProvider struct {
	response ports.ChatResponse
	err      error
	lastReq  ports.ChatRequest
	calls    int
}

func (s *stubAIProvider) Chat(_ context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	s.calls++
	s.lastReq = req
	if s.err != nil {
		return ports.ChatResponse{}, s.err
	}
	return s.response, nil
}

func (s *stubAIProvider) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, errors.New("not implemented")
}

func (s *stubAIProvider) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, errors.New("not implemented")
}

func (s *stubAIProvider) SupportsAudioInput() bool {
	return false
}

func (s *stubAIProvider) SupportsAudioOutput() bool {
	return false
}

func (s *stubAIProvider) GetMaxTokens() int {
	return 512
}

func (s *stubAIProvider) GetContextWindow() int {
	return 8192
}

type stubMessageDispatcher struct {
	response  string
	err       error
	calls     int
	lastInput domainhandlers.HandleMessageInput
}

func (s *stubMessageDispatcher) Handle(_ context.Context, input domainhandlers.HandleMessageInput) error {
	s.calls++
	s.lastInput = input
	if s.err != nil {
		return s.err
	}
	if input.OnAssistantResponse != nil {
		input.OnAssistantResponse(s.response)
	}
	return nil
}

func TestHandler_AgentCardEndpoint(t *testing.T) {
	cfg := newTestConfig()
	deps := &resolvers.Deps{
		ConfigSnapshot: &dto.AppConfigSnapshot{
			Agent: &dto.AgentConfigSnapshot{Name: "OpenLobster Snapshot Agent"},
			GraphQL: &dto.GraphQLConfigSnapshot{
				Host: "127.0.0.1",
				Port: 7070,
			},
		},
	}

	mux := http.NewServeMux()
	NewHandler(cfg, deps).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `"name":"OpenLobster Snapshot Agent"`)
	assert.Contains(t, rec.Body.String(), `"url":"http://127.0.0.1:7070/a2a"`)
	assert.Contains(t, rec.Body.String(), `"preferredTransport":"JSONRPC"`)
}

func TestHandler_Invoke_UsesAIProvider(t *testing.T) {
	provider := &stubAIProvider{response: ports.ChatResponse{Content: "respuesta A2A"}}
	cfg := newTestConfig()
	deps := &resolvers.Deps{
		AIProvider: provider,
		ConfigSnapshot: &dto.AppConfigSnapshot{
			Agent: &dto.AgentConfigSnapshot{Model: "gpt-4o-mini", Provider: "openai"},
		},
	}

	mux := http.NewServeMux()
	NewHandler(cfg, deps).Register(mux)

	payload := `{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"messageId":"m-1","role":"user","parts":[{"kind":"text","text":"Hola A2A"}]}}}`
	req := httptest.NewRequest(http.MethodPost, InvokePath, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"jsonrpc":"2.0"`)
	assert.Contains(t, rec.Body.String(), "respuesta A2A")
	require.Len(t, provider.lastReq.Messages, 1)
	assert.Equal(t, "Hola A2A", provider.lastReq.Messages[0].Content)
	assert.Equal(t, "gpt-4o-mini", provider.lastReq.Model)
	assert.Equal(t, 512, provider.lastReq.MaxTokens)
	assert.Equal(t, 1, provider.calls)
}

func TestHandler_Invoke_UsesMessageDispatcherWithAgentChannel(t *testing.T) {
	provider := &stubAIProvider{response: ports.ChatResponse{Content: "respuesta AI"}}
	dispatcher := &stubMessageDispatcher{response: "respuesta dispatcher"}
	cfg := newTestConfig()
	deps := &resolvers.Deps{
		AIProvider:        provider,
		MessageDispatcher: dispatcher,
	}

	mux := http.NewServeMux()
	NewHandler(cfg, deps).Register(mux)

	payload := `{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"messageId":"m-3","role":"user","parts":[{"kind":"text","text":"Hola por dispatcher"}]}}}`
	req := httptest.NewRequest(http.MethodPost, InvokePath, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "respuesta dispatcher")
	assert.Equal(t, 1, dispatcher.calls)
	assert.Equal(t, a2aAgentChannelID, dispatcher.lastInput.ChannelID)
	assert.Equal(t, a2aAgentChannelType, dispatcher.lastInput.ChannelType)
	assert.Equal(t, "Hola por dispatcher", dispatcher.lastInput.Content)
	assert.Equal(t, 0, provider.calls, "AI provider should not be called when dispatcher is wired")
}

func TestHandler_Invoke_WithoutProviderReturnsFallbackMessage(t *testing.T) {
	cfg := newTestConfig()
	deps := &resolvers.Deps{}

	mux := http.NewServeMux()
	NewHandler(cfg, deps).Register(mux)

	payload := `{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"messageId":"m-2","role":"user","parts":[{"kind":"text","text":"Hola"}]}}}`
	req := httptest.NewRequest(http.MethodPost, InvokePath, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), defaultNoProviderReply)
}

func newTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Agent.Name = "OpenLobster Test Agent"
	cfg.Agent.Provider = "openai"
	cfg.GraphQL.Host = "127.0.0.1"
	cfg.GraphQL.Port = 7070
	return cfg
}
