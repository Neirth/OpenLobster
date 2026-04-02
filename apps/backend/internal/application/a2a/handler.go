package a2a

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	a2aproto "github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"github.com/a2aproject/a2a-go/a2asrv/eventqueue"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/config"
)

const (
	// InvokePath is the JSON-RPC endpoint used to serve A2A protocol methods.
	InvokePath = "/a2a"

	defaultAgentName         = "OpenLobster Agent"
	defaultNoInputReply      = "I received the request, but no text input was provided."
	defaultNoProviderReply   = "A2A endpoint is online but no AI provider is configured."
	defaultGenerationFailure = "I could not complete the request right now."
	defaultCancelReply       = "Task canceled by client."
	a2aAgentChannelID        = "agent"
	a2aAgentChannelType      = "agent"
	a2aSenderName            = "A2A"
)

// Handler registers A2A HTTP endpoints.
type Handler struct {
	jsonrpc http.Handler
	card    http.Handler
}

// NewHandler builds an A2A controller with:
// - /.well-known/agent-card.json for discovery
// - /a2a JSON-RPC endpoint for protocol methods
func NewHandler(cfg *config.Config, deps *resolvers.Deps) *Handler {
	agentCard := buildAgentCard(cfg, deps)
	executor := &agentExecutor{
		aiProvider: aiProviderFromDeps(deps),
		dispatcher: dispatcherFromDeps(deps),
		model:      resolveModel(cfg, deps),
	}

	requestHandler := a2asrv.NewHandler(executor, a2asrv.WithExtendedAgentCard(agentCard))
	return &Handler{
		jsonrpc: a2asrv.NewJSONRPCHandler(requestHandler),
		card:    a2asrv.NewStaticAgentCardHandler(agentCard),
	}
}

// Register mounts A2A endpoints on the provided mux.
func (h *Handler) Register(mux *http.ServeMux) {
	if h == nil || mux == nil {
		return
	}

	if h.card != nil {
		mux.Handle(a2asrv.WellKnownAgentCardPath, h.card)
	}
	if h.jsonrpc != nil {
		mux.Handle(InvokePath, h.jsonrpc)
	}

	log.Printf("a2a: %s and %s registered", a2asrv.WellKnownAgentCardPath, InvokePath)
}

type agentExecutor struct {
	aiProvider ports.AIProviderPort
	dispatcher resolvers.MessageDispatcherPort
	model      string
}

func (e *agentExecutor) Execute(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	if reqCtx == nil {
		return fmt.Errorf("missing request context")
	}
	if queue == nil {
		return fmt.Errorf("missing event queue")
	}

	prompt := extractPrompt(reqCtx.Message)
	if prompt == "" {
		reply := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: defaultNoInputReply})
		return queue.Write(ctx, reply)
	}

	if e.dispatcher != nil {
		finalReply := ""
		err := e.dispatcher.Handle(ctx, domainhandlers.HandleMessageInput{
			ChannelID:   a2aAgentChannelID,
			ChannelType: a2aAgentChannelType,
			SenderID:    a2aAgentChannelID,
			SenderName:  a2aSenderName,
			Content:     prompt,
			OnAssistantResponse: func(content string) {
				finalReply = strings.TrimSpace(content)
			},
		})
		if err != nil {
			failure := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: defaultGenerationFailure})
			status := a2aproto.NewStatusUpdateEvent(reqCtx, a2aproto.TaskStateFailed, failure)
			status.Final = true
			return queue.Write(ctx, status)
		}
		if finalReply == "" {
			finalReply = defaultGenerationFailure
		}

		reply := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: finalReply})
		return queue.Write(ctx, reply)
	}

	if e.aiProvider == nil {
		reply := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: defaultNoProviderReply})
		return queue.Write(ctx, reply)
	}

	req := ports.ChatRequest{
		Model: e.model,
		Messages: []ports.ChatMessage{{
			Role:    "user",
			Content: prompt,
		}},
	}
	if maxTokens := e.aiProvider.GetMaxTokens(); maxTokens > 0 {
		req.MaxTokens = maxTokens
	}

	resp, err := e.aiProvider.Chat(ctx, req)
	if err != nil {
		failure := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: defaultGenerationFailure})
		status := a2aproto.NewStatusUpdateEvent(reqCtx, a2aproto.TaskStateFailed, failure)
		status.Final = true
		return queue.Write(ctx, status)
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		content = defaultGenerationFailure
	}

	reply := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: content})
	return queue.Write(ctx, reply)
}

func (e *agentExecutor) Cancel(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	if reqCtx == nil {
		return fmt.Errorf("missing request context")
	}
	if queue == nil {
		return fmt.Errorf("missing event queue")
	}

	msg := a2aproto.NewMessageForTask(a2aproto.MessageRoleAgent, reqCtx, a2aproto.TextPart{Text: defaultCancelReply})
	status := a2aproto.NewStatusUpdateEvent(reqCtx, a2aproto.TaskStateCanceled, msg)
	status.Final = true
	return queue.Write(ctx, status)
}

func extractPrompt(msg *a2aproto.Message) string {
	if msg == nil || len(msg.Parts) == 0 {
		return ""
	}

	parts := make([]string, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case a2aproto.TextPart:
			text := strings.TrimSpace(p.Text)
			if text != "" {
				parts = append(parts, text)
			}
		case *a2aproto.TextPart:
			if p == nil {
				continue
			}
			text := strings.TrimSpace(p.Text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func aiProviderFromDeps(deps *resolvers.Deps) ports.AIProviderPort {
	if deps == nil {
		return nil
	}
	return deps.AIProvider
}

func dispatcherFromDeps(deps *resolvers.Deps) resolvers.MessageDispatcherPort {
	if deps == nil {
		return nil
	}
	return deps.MessageDispatcher
}

func buildAgentCard(cfg *config.Config, deps *resolvers.Deps) *a2aproto.AgentCard {
	baseURL := resolveBaseURL(cfg, deps)
	provider := resolveProvider(cfg, deps)
	model := resolveModel(cfg, deps)

	description := "OpenLobster A2A endpoint for interoperable agent-to-agent requests."
	if provider != "" {
		description = fmt.Sprintf("%s Provider: %s.", description, provider)
		if model != "" {
			description = fmt.Sprintf("%s Model: %s.", description, model)
		}
	}

	inputModes := []string{"text/plain"}
	outputModes := []string{"text/plain"}
	invokeURL := strings.TrimRight(baseURL, "/") + InvokePath

	return &a2aproto.AgentCard{
		Capabilities: a2aproto.AgentCapabilities{
			Streaming: true,
		},
		DefaultInputModes:    inputModes,
		DefaultOutputModes:   outputModes,
		Description:          description,
		Name:                 resolveAgentName(cfg, deps),
		PreferredTransport:   a2aproto.TransportProtocolJSONRPC,
		ProtocolVersion:      string(a2aproto.Version),
		Provider:             &a2aproto.AgentProvider{Org: "OpenLobster", URL: strings.TrimRight(baseURL, "/")},
		Skills:               []a2aproto.AgentSkill{chatSkill(inputModes, outputModes)},
		URL:                  invokeURL,
		Version:              "0.1.0",
		AdditionalInterfaces: []a2aproto.AgentInterface{{Transport: a2aproto.TransportProtocolJSONRPC, URL: invokeURL}},
	}
}

func chatSkill(inputModes []string, outputModes []string) a2aproto.AgentSkill {
	return a2aproto.AgentSkill{
		ID:          "openlobster-chat",
		Name:        "OpenLobster Chat",
		Description: "General-purpose chat completion through the active OpenLobster AI provider.",
		Tags:        []string{"chat", "assistant", "openlobster"},
		Examples: []string{
			"Summarize this conversation in three bullets.",
			"Draft a concise reply to this customer message.",
		},
		InputModes:  inputModes,
		OutputModes: outputModes,
	}
}

func resolveAgentName(cfg *config.Config, deps *resolvers.Deps) string {
	if deps != nil && deps.ConfigSnapshot != nil && deps.ConfigSnapshot.Agent != nil {
		if name := strings.TrimSpace(deps.ConfigSnapshot.Agent.Name); name != "" {
			return name
		}
	}
	if cfg != nil {
		if name := strings.TrimSpace(cfg.Agent.Name); name != "" {
			return name
		}
	}
	return defaultAgentName
}

func resolveProvider(cfg *config.Config, deps *resolvers.Deps) string {
	if deps != nil && deps.ConfigSnapshot != nil && deps.ConfigSnapshot.Agent != nil {
		if provider := strings.TrimSpace(deps.ConfigSnapshot.Agent.Provider); provider != "" {
			return provider
		}
	}
	if cfg != nil {
		return strings.TrimSpace(cfg.Agent.Provider)
	}
	return ""
}

func resolveModel(cfg *config.Config, deps *resolvers.Deps) string {
	if deps != nil && deps.ConfigSnapshot != nil && deps.ConfigSnapshot.Agent != nil {
		if model := strings.TrimSpace(deps.ConfigSnapshot.Agent.Model); model != "" {
			return model
		}
	}

	if cfg == nil {
		return ""
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Agent.Provider))
	switch provider {
	case "openrouter":
		return strings.TrimSpace(cfg.Providers.OpenRouter.DefaultModel)
	case "ollama":
		return strings.TrimSpace(cfg.Providers.Ollama.DefaultModel)
	case "openai":
		return strings.TrimSpace(cfg.Providers.OpenAI.Model)
	case "openaicompat", "openai_compat", "openai-compatible":
		return strings.TrimSpace(cfg.Providers.OpenAICompat.Model)
	case "anthropic":
		return strings.TrimSpace(cfg.Providers.Anthropic.Model)
	case "docker_model_runner", "dockermodelrunner", "docker-model-runner":
		return strings.TrimSpace(cfg.Providers.DockerModelRunner.DefaultModel)
	case "opencode":
		return strings.TrimSpace(cfg.Providers.OpenCode.Model)
	}

	for _, model := range []string{
		strings.TrimSpace(cfg.Providers.OpenAI.Model),
		strings.TrimSpace(cfg.Providers.OpenRouter.DefaultModel),
		strings.TrimSpace(cfg.Providers.Ollama.DefaultModel),
		strings.TrimSpace(cfg.Providers.OpenAICompat.Model),
		strings.TrimSpace(cfg.Providers.Anthropic.Model),
		strings.TrimSpace(cfg.Providers.DockerModelRunner.DefaultModel),
		strings.TrimSpace(cfg.Providers.OpenCode.Model),
	} {
		if model != "" {
			return model
		}
	}

	return ""
}

func resolveBaseURL(cfg *config.Config, deps *resolvers.Deps) string {
	if deps != nil && deps.ConfigSnapshot != nil && deps.ConfigSnapshot.GraphQL != nil {
		if baseURL := strings.TrimSpace(deps.ConfigSnapshot.GraphQL.BaseURL); baseURL != "" {
			return strings.TrimRight(baseURL, "/")
		}

		host := sanitizeHost(deps.ConfigSnapshot.GraphQL.Host)
		port := deps.ConfigSnapshot.GraphQL.Port
		if host != "" && port > 0 {
			return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
		}
	}

	if cfg != nil {
		if baseURL := strings.TrimSpace(cfg.GraphQL.BaseURL); baseURL != "" {
			return strings.TrimRight(baseURL, "/")
		}

		host := sanitizeHost(cfg.GraphQL.Host)
		port := cfg.GraphQL.Port
		if host != "" && port > 0 {
			return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
		}
	}

	return "http://127.0.0.1:8080"
}

func sanitizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "127.0.0.1"
	}
	if host == "0.0.0.0" || host == "::" || host == "[::]" {
		return "127.0.0.1"
	}
	return host
}
