// openlobster daemon entry point.
//
// Loads configuration from openlobster.yaml, wires all infrastructure
// adapters, starts the GraphQL dashboard server, the heartbeat loop, and
// any enabled messaging channel adapters.
//
// # License
// See LICENSE in the root of the repository.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	appcontext "github.com/neirth/openlobster/internal/domain/context"
	"github.com/neirth/openlobster/internal/domain/events"
	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	aianthropicadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/anthropic"
	aidockermodelrunner "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/docker"
	aiollama "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/ollama"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
	aiopenaicompat "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openaicompat"
	aiopenrouter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openrouter"
	aizenadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/zen"
	browser "github.com/neirth/openlobster/internal/infrastructure/adapters/browser/chromedp"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/filesystem"
	memfile "github.com/neirth/openlobster/internal/infrastructure/adapters/memory/file"
	memneo4j "github.com/neirth/openlobster/internal/infrastructure/adapters/memory/neo4j"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/discord"
	slackadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/slack"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/telegram"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/twilio"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/whatsapp"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/terminal"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/neirth/openlobster/internal/application/graphql/subscriptions"
	"github.com/neirth/openlobster/internal/application/health"
	"github.com/neirth/openlobster/internal/application/metrics"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/application/webhooks"
	"github.com/neirth/openlobster/internal/domain/repositories"
	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/neirth/openlobster/internal/infrastructure/secrets"
	"github.com/spf13/viper"
)

// version is set at build time via ldflags (-X main.version=...)
var version = "dev"

// public is the single embedded FS containing:
//
//	public/assets/     — compiled SolidJS frontend (Vite outDir)
//	public/*           — any other static resources served at /public/
//
//go:embed all:public
var public embed.FS

// chanRegistry is a thread-safe registry of active messaging adapters by channel type.
// Webhooks are registered once at startup and look up the current adapter on every
// request, which allows hot-reloading adapters without restarting the server.
type chanRegistry struct {
	mu       sync.RWMutex
	adapters map[string]ports.MessagingPort
}

func newChanRegistry() *chanRegistry {
	return &chanRegistry{adapters: make(map[string]ports.MessagingPort)}
}

func (r *chanRegistry) set(channelType string, a ports.MessagingPort) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[channelType] = a
}

func (r *chanRegistry) get(channelType string) ports.MessagingPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType]
}

func (r *chanRegistry) remove(channelType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, channelType)
}

func (r *chanRegistry) listTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.adapters))
	for k := range r.adapters {
		out = append(out, k)
	}
	return out
}

// webhookAdapterRegistry adapts chanRegistry to webhooks.AdapterRegistry.
type webhookAdapterRegistry struct{ reg *chanRegistry }

func (w *webhookAdapterRegistry) Get(channelType string) ports.MessagingPort {
	return w.reg.get(channelType)
}

// configUpdateAdapter persiste UpdateConfigInput en viper y recarga canales.
// Cuando cambian provider/agent keys, onApplied recibe providerTouched=true y debe
// refrescar ConfigSnapshot y ejecutar soft reboot (recrear AI provider, etc.).
type configUpdateAdapter struct {
	configPath    string
	reloadChannel func(channelType string)
	viperKeys     map[string]string
	onApplied     func(providerTouched bool) // tras guardar: refrescar snapshot + soft reboot si provider cambió
}

func (a *configUpdateAdapter) Apply(ctx context.Context, input map[string]interface{}) ([]string, error) {
	var changedChannels []string
	channelTouched := make(map[string]bool)

	// Apply provider-specific keys first (need provider value to route correctly).
	a.applyProviderKeys(input)

	// Apply capabilities as nested agent.capabilities.*
	if caps, ok := input["capabilities"].(map[string]interface{}); ok {
		for k, v := range caps {
			viper.Set("agent.capabilities."+k, v)
		}
	}

	for inputKey, val := range input {
		if inputKey == "capabilities" || a.isProviderInputKey(inputKey) {
			continue // already handled
		}
		viperKey, ok := a.viperKeys[inputKey]
		if !ok {
			continue
		}
		viper.Set(viperKey, val)
		switch inputKey {
		case "channelTelegramEnabled", "channelTelegramToken":
			channelTouched["telegram"] = true
		case "channelDiscordEnabled", "channelDiscordToken":
			channelTouched["discord"] = true
		case "channelSlackEnabled", "channelSlackBotToken", "channelSlackAppToken":
			channelTouched["slack"] = true
		case "channelWhatsAppEnabled", "channelWhatsAppPhoneId", "channelWhatsAppApiToken":
			channelTouched["whatsapp"] = true
		case "channelTwilioEnabled", "channelTwilioAccountSid", "channelTwilioAuthToken", "channelTwilioFromNumber":
			channelTouched["twilio"] = true
		}
	}
	for ch := range channelTouched {
		changedChannels = append(changedChannels, ch)
	}
	providerTouched := false
	for k := range input {
		if a.isProviderInputKey(k) {
			providerTouched = true
			break
		}
	}
	if len(input) > 0 {
		if err := config.WriteEncryptedConfig(a.configPath); err != nil {
			return nil, fmt.Errorf("persisting config to %s: %w", a.configPath, err)
		}
		for _, ch := range changedChannels {
			a.reloadChannel(ch)
		}
		if a.onApplied != nil {
			a.onApplied(providerTouched)
		}
	}
	return changedChannels, nil
}

func (a *configUpdateAdapter) isProviderInputKey(k string) bool {
	switch k {
	case "provider", "model", "apiKey", "baseURL", "ollamaHost", "ollamaApiKey",
		"anthropicApiKey", "dockerModelRunnerEndpoint", "dockerModelRunnerModel":
		return true
	}
	return false
}

func (a *configUpdateAdapter) applyProviderKeys(input map[string]interface{}) {
	provider, _ := input["provider"].(string)
	if provider == "" {
		provider = "ollama" // default
	}

	switch provider {
	case "openrouter":
		if v, ok := input["apiKey"].(string); ok {
			viper.Set("providers.openrouter.api_key", v)
		}
		if v, ok := input["model"].(string); ok {
			viper.Set("providers.openrouter.default_model", v)
		}
	case "ollama":
		if v, ok := input["ollamaHost"].(string); ok {
			viper.Set("providers.ollama.endpoint", v)
		}
		if v, ok := input["ollamaApiKey"].(string); ok {
			viper.Set("providers.ollama.api_key", v)
		}
		if v, ok := input["model"].(string); ok {
			viper.Set("providers.ollama.default_model", v)
		}
	case "openai":
		if v, ok := input["apiKey"].(string); ok {
			viper.Set("providers.openai.api_key", v)
		}
		if v, ok := input["model"].(string); ok {
			viper.Set("providers.openai.model", v)
		}
		if v, ok := input["baseURL"].(string); ok {
			viper.Set("providers.openai.base_url", v)
		}
	case "openai-compatible":
		if v, ok := input["apiKey"].(string); ok {
			viper.Set("providers.openaicompat.api_key", v)
		}
		if v, ok := input["baseURL"].(string); ok {
			viper.Set("providers.openaicompat.base_url", v)
		}
		if v, ok := input["model"].(string); ok {
			viper.Set("providers.openaicompat.model", v)
		}
	case "anthropic":
		if v, ok := input["anthropicApiKey"].(string); ok {
			viper.Set("providers.anthropic.api_key", v)
		}
		if v, ok := input["model"].(string); ok {
			viper.Set("providers.anthropic.model", v)
		}
	case "docker-model-runner":
		if v, ok := input["dockerModelRunnerEndpoint"].(string); ok {
			viper.Set("providers.docker_model_runner.endpoint", v)
		}
		if v, ok := input["dockerModelRunnerModel"].(string); ok {
			viper.Set("providers.docker_model_runner.default_model", v)
		}
	case "opencode-zen":
		if v, ok := input["apiKey"].(string); ok {
			viper.Set("providers.opencode.api_key", v)
		}
		if v, ok := input["model"].(string); ok {
			viper.Set("providers.opencode.model", v)
		}
	}
}

func buildInputToViperKeyMap() map[string]string {
	return map[string]string{
		"agentName":               "agent.name",
		"systemPrompt":            "agent.system_prompt",
		"databaseDriver":          "database.driver",
		"databaseDSN":             "database.dsn",
		"databaseMaxOpenConns":    "database.max_open_conns",
		"databaseMaxIdleConns":    "database.max_idle_conns",
		"memoryBackend":           "memory.backend",
		"memoryFilePath":          "memory.file.path",
		"memoryNeo4jURI":          "memory.neo4j.uri",
		"memoryNeo4jUser":         "memory.neo4j.user",
		"memoryNeo4jPassword":     "memory.neo4j.password",
		"subagentsMaxConcurrent":  "subagents.max_concurrent",
		"subagentsDefaultTimeout": "subagents.default_timeout",
		"graphqlEnabled":          "graphql.enabled",
		"graphqlPort":             "graphql.port",
		"graphqlHost":             "graphql.host",
		"graphqlBaseUrl":          "graphql.base_url",
		"loggingLevel":            "logging.level",
		"loggingPath":             "logging.path",
		"secretsBackend":          "secrets.backend",
		"secretsFilePath":         "secrets.file.path",
		"secretsOpenbaoURL":       "secrets.openbao.url",
		"secretsOpenbaoToken":     "secrets.openbao.token",
		"schedulerEnabled":        "heartbeat.enabled",
		"schedulerMemoryEnabled":  "heartbeat.memory_enabled",
		"schedulerMemoryInterval": "heartbeat.memory_interval",
		"channelTelegramEnabled":  "channels.telegram.enabled",
		"channelTelegramToken":    "channels.telegram.bot_token",
		"channelDiscordEnabled":   "channels.discord.enabled",
		"channelDiscordToken":     "channels.discord.bot_token",
		"channelWhatsAppEnabled":  "channels.whatsapp.enabled",
		"channelWhatsAppPhoneId":  "channels.whatsapp.phone_id",
		"channelWhatsAppApiToken": "channels.whatsapp.api_token",
		"channelTwilioEnabled":    "channels.twilio.enabled",
		"channelTwilioAccountSid": "channels.twilio.account_sid",
		"channelTwilioAuthToken":  "channels.twilio.auth_token",
		"channelTwilioFromNumber": "channels.twilio.from_number",
		"channelSlackEnabled":     "channels.slack.enabled",
		"channelSlackBotToken":    "channels.slack.bot_token",
		"channelSlackAppToken":    "channels.slack.app_token",
		"wizardCompleted":         "wizard.completed",
	}
}

// SendTextToChannel delivers a plain-text message to a channel user identified
// by channelType and channelID. Used by the GraphQL resolver to notify users
// after pairing events (approval/denial).
func (r *chanRegistry) SendTextToChannel(ctx context.Context, channelType, channelID, text string) error {
	adapter := r.get(channelType)
	if adapter == nil {
		return nil // adapter not active – skip silently
	}
	msg := models.NewMessage(channelID, text)
	return adapter.SendMessage(ctx, msg)
}

// messagingRouter implements ports.MessagingPort by routing SendMessage to the
// correct adapter based on msg.Metadata["channel_type"]. The message handler
// must set this when sending so replies reach the right channel (e.g. Discord
// vs Telegram).
type messagingRouter struct {
	reg *chanRegistry
}

func (m *messagingRouter) SendTyping(ctx context.Context, channelID string) error {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil
	}
	adapter := m.reg.get(ct)
	if adapter == nil {
		return nil
	}
	return adapter.SendTyping(ctx, channelID)
}

func (m *messagingRouter) SendMessage(ctx context.Context, msg *models.Message) error {
	if msg == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(msg.Content), "no_reply") {
		return nil // abort: do not send messages containing no_reply
	}
	ct := ""
	if msg.Metadata != nil {
		if v, ok := msg.Metadata["channel_type"].(string); ok {
			ct = strings.TrimSpace(strings.ToLower(v))
		}
	}
	if ct == "" {
		err := fmt.Errorf("messaging: cannot route — msg has no channel_type in Metadata (channel_id=%q)", msg.ChannelID)
		log.Print(err)
		return err
	}
	adapter := m.reg.get(ct)
	if adapter == nil {
		err := fmt.Errorf("messaging: cannot route — no adapter for channel_type=%q (channel_id=%q)", ct, msg.ChannelID)
		log.Print(err)
		return err
	}
	return adapter.SendMessage(ctx, msg)
}

func (m *messagingRouter) SendMedia(ctx context.Context, media *ports.Media) error {
	if media == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(media.Caption), "no_reply") {
		return nil // abort: do not send media with caption containing no_reply
	}
	ct := media.ChannelType
	if ct == "" {
		return nil // no channel type – cannot route
	}
	adapter := m.reg.get(ct)
	if adapter == nil {
		return nil
	}
	return adapter.SendMedia(ctx, media)
}

// HandleWebhook is not used by the router — WhatsApp, Twilio, etc. hit
// platform-specific webhook URLs that dispatch directly to each adapter.
func (m *messagingRouter) HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error) {
	return nil, nil
}

func (m *messagingRouter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil, nil
	}
	adapter := m.reg.get(ct)
	if adapter == nil {
		return nil, nil
	}
	return adapter.GetUserInfo(ctx, userID)
}

func (m *messagingRouter) React(ctx context.Context, messageID string, emoji string) error {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil
	}
	adapter := m.reg.get(ct)
	if adapter == nil {
		return nil
	}
	return adapter.React(ctx, messageID, emoji)
}

func (m *messagingRouter) GetCapabilities() ports.ChannelCapabilities {
	merged := ports.ChannelCapabilities{}
	for _, ct := range m.reg.listTypes() {
		adapter := m.reg.get(ct)
		if adapter == nil {
			continue
		}
		caps := adapter.GetCapabilities()
		if caps.HasVoiceMessage {
			merged.HasVoiceMessage = true
		}
		if caps.HasCallStream {
			merged.HasCallStream = true
		}
		if caps.HasTextStream {
			merged.HasTextStream = true
		}
		if caps.HasMediaSupport {
			merged.HasMediaSupport = true
		}
	}
	return merged
}

func (m *messagingRouter) ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error) {
	ct, _ := ctx.Value(ports.ContextKeyChannelType).(string)
	if ct == "" {
		return nil, "", nil
	}
	adapter := m.reg.get(ct)
	if adapter == nil {
		return nil, "", nil
	}
	return adapter.ConvertAudioForPlatform(ctx, audioData, format)
}

// Start is not used by the router — each adapter is started individually in main.
func (m *messagingRouter) Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error {
	return nil
}

// conversationPortAdapter adapts the persistence.conversationRepository to the
// dashboard.ConversationPort interface, converting ConversationRow values to
// dashboard.ConversationInfo values.
type conversationPortAdapter struct {
	repo interface {
		ListConversations() ([]repositories.ConversationRow, error)
		DeleteUser(ctx context.Context, conversationID string) error
	}
}

func (a *conversationPortAdapter) ListConversations() ([]dto.ConversationSnapshot, error) {
	rows, err := a.repo.ListConversations()
	if err != nil {
		return nil, err
	}
	result := make([]dto.ConversationSnapshot, len(rows))
	for i, r := range rows {
		result[i] = dto.ConversationSnapshot{
			ID:              r.ID,
			ChannelID:       r.ChannelID,
			ChannelType:     r.ChannelType,
			ChannelName:     r.ChannelName,
			GroupName:       r.GroupName,
			IsGroup:         r.IsGroup,
			ParticipantID:   r.ParticipantID,
			ParticipantName: r.ParticipantName,
			LastMessageAt:   r.LastMessageAt,
			UnreadCount:     r.UnreadCount,
		}
	}
	return result, nil
}

func (a *conversationPortAdapter) DeleteUser(ctx context.Context, conversationID string) error {
	return a.repo.DeleteUser(ctx, conversationID)
}

// toolPermAdapter adapts repositories.ToolPermissionRepositoryPort to
// dashboard.ToolPermissionsRepository, bridging the two package-local record
// types without coupling the domain packages to each other.
type toolPermAdapter struct {
	repo repositories.ToolPermissionRepositoryPort
}

// mcpServerAdapter adapts repositories.MCPServerRepositoryPort to
// dashboard.MCPServerRepository.
type mcpServerAdapter struct {
	repo repositories.MCPServerRepositoryPort
}

func (a *mcpServerAdapter) Save(ctx context.Context, name, url string) error {
	return a.repo.Save(ctx, name, url)
}

func (a *mcpServerAdapter) Delete(ctx context.Context, name string) error {
	return a.repo.Delete(ctx, name)
}

func (a *mcpServerAdapter) ListAll(ctx context.Context) ([]dto.MCPServerRecord, error) {
	rows, err := a.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.MCPServerRecord, len(rows))
	for i, r := range rows {
		result[i] = dto.MCPServerRecord{Name: r.Name, URL: r.URL}
	}
	return result, nil
}

// syncToolsToAgentRegistry copies all tools from the ToolRegistry into the
// AgentRegistry so the GraphQL tools/mcpTools queries and Status/Metrics expose
// the full list (internal + MCP) to the frontend.
func syncToolsToAgentRegistry(reg *mcp.ToolRegistry, agentReg *registry.AgentRegistry) {
	if reg == nil || agentReg == nil {
		return
	}
	defs := reg.AllTools()
	snapshots := make([]dto.ToolSnapshot, len(defs))
	for i, d := range defs {
		source := "internal"
		serverName := ""
		if strings.Contains(d.Name, ":") {
			source = "mcp"
			if idx := strings.Index(d.Name, ":"); idx >= 0 {
				serverName = d.Name[:idx]
			}
		}
		snapshots[i] = dto.ToolSnapshot{
			Name:        d.Name,
			Description: d.Description,
			Source:      source,
			ServerName:  serverName,
		}
	}
	agentReg.UpdateMCPTools(snapshots)
}

// mcpConnectAdapter implements dto.McpConnectPort for connectMcp/disconnectMcp mutations.
type mcpConnectAdapter struct {
	client   *mcp.MCPClientSDK
	registry *mcp.ToolRegistry
	agentReg *registry.AgentRegistry
	repo     repositories.MCPServerRepositoryPort
	oauth    *mcp.OAuthManager
	eventBus dto.EventBusPort
}

func (a *mcpConnectAdapter) Connect(ctx context.Context, name, transport, url string) (bool, error) {
	if transport != "http" && transport != "" {
		return false, fmt.Errorf("only http transport is supported, got %q", transport)
	}
	if url == "" {
		return false, fmt.Errorf("url is required")
	}
	err := a.client.Connect(ctx, mcp.ServerConfig{Name: name, Type: "http", URL: url})
	if err != nil {
		errStr := err.Error()
		requiresAuth := strings.Contains(errStr, "401") || strings.Contains(strings.ToLower(errStr), "unauthorized")
		if requiresAuth {
			a.oauth.RegisterPendingServer(name, url)
		}
		return requiresAuth, err
	}
	// Persist and register tools
	if err := a.repo.Save(ctx, name, url); err != nil {
		log.Printf("mcp: failed to persist %q: %v", name, err)
	}
	if tools := a.client.GetServerTools(name); len(tools) > 0 {
		_ = a.registry.RegisterMCP(name, a.client, tools)
		log.Printf("mcp: registered %d tools from %q", len(tools), name)
	}
	syncToolsToAgentRegistry(a.registry, a.agentReg)
	if a.eventBus != nil {
		_ = a.eventBus.Publish(ctx, events.EventMCPServerConnected, map[string]string{"name": name})
	}
	return false, nil
}

func (a *mcpConnectAdapter) Disconnect(ctx context.Context, name string) error {
	if err := a.client.Disconnect(name); err != nil {
		return err
	}
	a.registry.UnregisterMCP(name)
	syncToolsToAgentRegistry(a.registry, a.agentReg)
	return a.repo.Delete(ctx, name)
}

func (a *mcpConnectAdapter) GetConnectionStatus(name string) string {
	if tools := a.client.GetServerTools(name); len(tools) > 0 {
		return "online"
	}
	return "unknown"
}

func (a *mcpConnectAdapter) GetServerToolCount(name string) int {
	return len(a.client.GetServerTools(name))
}

// mcpOAuthAdapter implements dto.McpOAuthPort for initiateOAuth and mcpOAuthStatus.
type mcpOAuthAdapter struct {
	oauth *mcp.OAuthManager
}

func (a *mcpOAuthAdapter) InitiateOAuth(ctx context.Context, serverName, mcpURL string) (string, error) {
	return a.oauth.InitiateOAuth(ctx, serverName, mcpURL)
}

func (a *mcpOAuthAdapter) Status(serverName string) (status, errMsg string) {
	s, errStr := a.oauth.Status(serverName)
	return string(s), errStr
}

func (a *toolPermAdapter) Set(ctx context.Context, userID, toolName, mode string) error {
	return a.repo.Set(ctx, userID, toolName, mode)
}

func (a *toolPermAdapter) Delete(ctx context.Context, userID, toolName string) error {
	return a.repo.Delete(ctx, userID, toolName)
}

func (a *toolPermAdapter) ListByUser(ctx context.Context, userID string) ([]dto.ToolPermissionRecord, error) {
	rows, err := a.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ToolPermissionRecord, len(rows))
	for i, r := range rows {
		result[i] = dto.ToolPermissionRecord{UserID: r.UserID, ToolName: r.ToolName, Mode: r.Mode}
	}
	return result, nil
}

func (a *toolPermAdapter) ListAll(ctx context.Context) ([]dto.ToolPermissionRecord, error) {
	rows, err := a.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ToolPermissionRecord, len(rows))
	for i, r := range rows {
		result[i] = dto.ToolPermissionRecord{UserID: r.UserID, ToolName: r.ToolName, Mode: r.Mode}
	}
	return result, nil
}

// toolNamesAdapter expone los nombres de todas las herramientas para SetAllToolPermissions.
type toolNamesAdapter struct{ reg *mcp.ToolRegistry }

func (a *toolNamesAdapter) AllToolNames() []string {
	if a.reg == nil {
		return nil
	}
	defs := a.reg.AllTools()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}

type msgRepoAdapter struct {
	repo *repositories.DashboardMessageRepository
}

func (a *msgRepoAdapter) Save(ctx context.Context, msg *models.Message) error {
	return a.repo.Save(msg)
}
func (a *msgRepoAdapter) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	return a.repo.GetByConversation(conversationID, limit)
}
func (a *msgRepoAdapter) GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]models.Message, error) {
	return a.repo.GetByConversationPaged(ctx, conversationID, before, limit)
}
func (a *msgRepoAdapter) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error) {
	return a.repo.GetSinceLastCompaction(ctx, conversationID)
}
func (a *msgRepoAdapter) CountMessages(ctx context.Context) (int64, int64, error) {
	return a.repo.CountMessages(ctx)
}

type pairingPortAdapter struct {
	svc             *domainservices.PairingService
	userRepo        ports.UserRepositoryPort
	userChannelRepo ports.UserChannelRepositoryPort
	channelRepo     ports.ChannelRepositoryPort
	messageSender   *chanRegistry
	eventBus        domainservices.EventBus
}

func (a *pairingPortAdapter) Approve(ctx context.Context, code, userID, displayName string) (*dto.PairingSnapshot, error) {
	p, err := a.svc.ApproveCode(ctx, code)
	if err != nil {
		return nil, err
	}

	platformUserID := p.PlatformUserID
	if platformUserID == "" {
		platformUserID = p.ChannelID
	}
	// platformUsername is the username as reported by the messaging platform.
	// It is stored in user_channels.username only.
	platformUsername := p.PlatformUserName

	if a.channelRepo != nil {
		_ = a.channelRepo.EnsurePlatform(ctx, p.ChannelType, p.ChannelType)
	}

	resolveUserID := userID
	if resolveUserID == "" {
		// New user path: look up by platform user ID first (idempotent), then create.
		if a.userRepo != nil {
			u, err := a.userRepo.GetByPrimaryID(ctx, platformUserID)
			if err == nil && u != nil {
				resolveUserID = u.ID.String()
			}
		}
		if resolveUserID == "" && a.userRepo != nil {
			// displayName provided by admin in the pairing modal becomes users.name.
			u := models.NewUser(platformUserID)
			u.Name = displayName
			if err := a.userRepo.Create(ctx, u); err == nil {
				resolveUserID = u.ID.String()
			}
		}
	}
	// If a displayName was provided by the admin and the user already exists
	// with an empty name, backfill it now.
	if resolveUserID != "" && displayName != "" && a.userRepo != nil {
		if existing, err := a.userRepo.GetByID(ctx, resolveUserID); err == nil && existing != nil && existing.Name == "" {
			existing.Name = displayName
			_ = a.userRepo.Update(ctx, existing)
		}
	}

	if resolveUserID != "" && a.userChannelRepo != nil {
		if err := a.userChannelRepo.Create(ctx, resolveUserID, p.ChannelType, platformUserID, platformUsername); err != nil {
			return &dto.PairingSnapshot{Code: p.Code, Status: p.Status}, nil
		}
	}

	if a.eventBus != nil {
		_ = a.eventBus.Publish(ctx, events.NewEvent(events.EventPairingApproved, events.PairingApprovedPayload{
			RequestID:  p.Code,
			Code:       p.Code,
			ApprovedBy: "admin",
			Timestamp:  time.Now(),
		}))
	}

	if a.messageSender != nil && p.ChannelID != "" {
		_ = a.messageSender.SendTextToChannel(ctx, p.ChannelType, p.ChannelID, "Your access request has been approved. You can start chatting now.")
	}

	return &dto.PairingSnapshot{Code: p.Code, Status: p.Status}, nil
}
func (a *pairingPortAdapter) Deny(ctx context.Context, code, reason string) error {
	return a.svc.DenyCode(ctx, code)
}
func (a *pairingPortAdapter) ListActive(ctx context.Context) ([]dto.PairingSnapshot, error) {
	list, err := a.svc.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PairingSnapshot, len(list))
	for i, p := range list {
		out[i] = dto.PairingSnapshot{Code: p.Code, Status: p.Status}
	}
	return out, nil
}

type userRepoAdapter struct {
	repo ports.UserRepositoryPort
}

func (a *userRepoAdapter) Create(ctx context.Context, user *models.User) error {
	return a.repo.Create(ctx, user)
}
func (a *userRepoAdapter) GetByID(ctx context.Context, id string) (*models.User, error) {
	return a.repo.GetByID(ctx, id)
}
func (a *userRepoAdapter) ListAll(ctx context.Context) ([]models.User, error) {
	return a.repo.ListAll(ctx)
}

type eventBusAdapter struct {
	eb domainservices.EventBus
}

func (e *eventBusAdapter) Publish(ctx context.Context, eventType string, payload interface{}) error {
	if e.eb == nil {
		return nil
	}
	return e.eb.Publish(ctx, events.NewEvent(eventType, payload))
}

// eventSubscriptionAdapter adapta domainservices.EventBus al puerto de suscripciones GraphQL.
type eventSubscriptionAdapter struct {
	eb domainservices.EventBus
}

func (a *eventSubscriptionAdapter) Subscribe(ctx context.Context, eventType string) (<-chan events.Event, error) {
	if a.eb == nil {
		ch := make(chan events.Event)
		return ch, nil
	}
	ch := make(chan events.Event, 64)
	done := ctx.Done()
	if err := a.eb.Subscribe(eventType, func(_ context.Context, event events.Event) error {
		select {
		case ch <- event:
		case <-done:
			// Subscription cancelled, drop event
		default:
			// Buffer full, drop
		}
		return nil
	}); err != nil {
		return nil, err
	}
	// No close(ch): resolver exits on ctx.Done() via select; closing could race with handler sends
	return ch, nil
}

// ---------------------------------------------------------------------------
// mcp.MessagingService adapter — bridges ports.MessagingPort to mcp.MessagingService.
// ---------------------------------------------------------------------------

type mcpMessagingAdapter struct{ port ports.MessagingPort }

func (m *mcpMessagingAdapter) SendMessage(ctx context.Context, channelType, channelID, content string) error {
	if m.port == nil {
		return fmt.Errorf("messaging: no adapter configured")
	}
	msg := models.NewMessage(channelID, content)
	if channelType != "" {
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]interface{})
		}
		msg.Metadata["channel_type"] = channelType
	}
	return m.port.SendMessage(ctx, msg)
}

func (m *mcpMessagingAdapter) SendFile(ctx context.Context, channelID, filePath string) error {
	return fmt.Errorf("messaging: send_file not supported")
}

func (m *mcpMessagingAdapter) SendMedia(ctx context.Context, media *ports.Media) error {
	if m.port == nil {
		return fmt.Errorf("messaging: no adapter configured")
	}
	return m.port.SendMedia(ctx, media)
}

// ---------------------------------------------------------------------------
// mcp.MemoryService adapter — bridges ports.MemoryPort to mcp.MemoryService.
// ---------------------------------------------------------------------------

type mcpMemoryAdapter struct{ port ports.MemoryPort }

func (m *mcpMemoryAdapter) AddKnowledge(ctx context.Context, userID, content, label, relation string) error {
	if m.port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.port.AddKnowledge(ctx, userID, content, label, relation, nil)
}

func (m *mcpMemoryAdapter) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	if m.port == nil {
		return nil
	}
	return m.port.UpdateUserLabel(ctx, userID, displayName)
}

func (m *mcpMemoryAdapter) SearchMemory(ctx context.Context, userID, query string) (string, error) {
	if m.port == nil {
		return "", fmt.Errorf("memory: no adapter configured")
	}

	// Use the user's memory graph for targeted search. This avoids leaking
	// facts from other users and gives the correct scope.
	graph, err := m.port.GetUserGraph(ctx, userID)
	if err != nil {
		return "", err
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(queryLower)

	var sb strings.Builder
	count := 0
	for _, node := range graph.Nodes {
		if node.Type != "fact" {
			continue
		}
		valueLower := strings.ToLower(node.Value)
		labelLower := strings.ToLower(node.Label)
		// Match if any query word appears in the value or label.
		matched := false
		for _, w := range queryWords {
			if strings.Contains(valueLower, w) || strings.Contains(labelLower, w) {
				matched = true
				break
			}
		}
		// Fallback: if no query words matched, still include all facts so the
		// model gets full context when asking broad questions like "what do I know?".
		if !matched && queryLower != "" && len(queryWords) > 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("[node_id:%s] %s\n", node.ID, node.Value))
		count++
		if count >= 10 {
			break
		}
	}

	// If nothing found via word match, return all user facts as fallback
	// so the model never gets a false "empty memory" response.
	if sb.Len() == 0 && len(graph.Nodes) > 0 {
		for _, node := range graph.Nodes {
			if node.Type == "fact" {
				sb.WriteString(fmt.Sprintf("[node_id:%s] %s\n", node.ID, node.Value))
				count++
				if count >= 10 {
					break
				}
			}
		}
	}
	return sb.String(), nil
}

func (m *mcpMemoryAdapter) SetUserProperty(ctx context.Context, userID, key, value string) error {
	if m.port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.port.SetUserProperty(ctx, userID, key, value)
}

func (m *mcpMemoryAdapter) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	if m.port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.port.EditMemoryNode(ctx, userID, nodeID, newValue)
}

func (m *mcpMemoryAdapter) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	if m.port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.port.DeleteMemoryNode(ctx, userID, nodeID)
}

// AddRelation creates an edge between two nodes in the underlying memory backend.
func (m *mcpMemoryAdapter) AddRelation(ctx context.Context, from, to, relType string) error {
	if m.port == nil {
		return fmt.Errorf("memory: no adapter configured")
	}
	return m.port.AddRelation(ctx, from, to, relType)
}

// QueryGraph executes a Cypher query and returns the raw result (data + errors)
// as provided by the backend. This is a thin wrapper around ports.MemoryPort.QueryGraph.
func (m *mcpMemoryAdapter) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	if m.port == nil {
		return ports.GraphResult{}, fmt.Errorf("memory: no adapter configured")
	}
	return m.port.QueryGraph(ctx, cypher)
}

// ---------------------------------------------------------------------------
// mcp.TaskService adapter — bridges repositories.TaskRepository to mcp.TaskService.
// ---------------------------------------------------------------------------

type mcpTaskAdapter struct {
	repo   repositories.TaskRepository
	notify func()
}

func (a *mcpTaskAdapter) Add(ctx context.Context, prompt, schedule string) (string, error) {
	t := models.NewTask(prompt, schedule)
	if err := a.repo.Add(ctx, t); err != nil {
		return "", err
	}
	if a.notify != nil {
		a.notify()
	}
	return t.ID, nil
}

func (a *mcpTaskAdapter) Done(ctx context.Context, id string) error {
	return a.repo.Done(ctx, id)
}

func (a *mcpTaskAdapter) List(ctx context.Context) ([]mcp.TaskInfo, error) {
	tasks, err := a.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.TaskInfo, len(tasks))
	for i, t := range tasks {
		result[i] = mcp.TaskInfo{
			ID:       t.ID,
			Prompt:   t.Prompt,
			Schedule: t.Schedule,
			Status:   t.Status,
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// mcpConversationAdapter bridges persistence repositories to mcp.ConversationService.
// ---------------------------------------------------------------------------

type mcpConversationAdapter struct {
	convRepo *repositories.ConversationRepository
	msgRepo  ports.MessageRepositoryPort
}

func (a *mcpConversationAdapter) ListConversations(ctx context.Context) ([]mcp.ConversationSummary, error) {
	rows, err := a.convRepo.ListConversations()
	if err != nil {
		return nil, err
	}
	result := make([]mcp.ConversationSummary, 0, len(rows))
	for _, r := range rows {
		result = append(result, mcp.ConversationSummary{
			ID:              r.ID,
			ChannelID:       r.ChannelID,
			ChannelName:     r.ChannelName,
			ParticipantID:   r.ParticipantID,
			ParticipantName: r.ParticipantName,
			LastMessageAt:   r.LastMessageAt,
			MessageCount:    r.UnreadCount,
		})
	}
	return result, nil
}

func (a *mcpConversationAdapter) GetConversationMessages(ctx context.Context, conversationID string, limit int) ([]mcp.ConversationMessage, error) {
	msgs, err := a.msgRepo.GetByConversation(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.ConversationMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "compaction" {
			continue
		}
		result = append(result, mcp.ConversationMessage{
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// mcpBrowserAdapter bridges ports.BrowserPort (ChromeDPAdapter page model)
// to the session-based mcp.BrowserService interface expected by internal tools.
// Each sessionID maps to an open browser page.
// ---------------------------------------------------------------------------

type mcpBrowserAdapter struct {
	port  *browser.ChromeDPAdapter
	pages sync.Map // sessionID -> ports.BrowserPage
}

func (a *mcpBrowserAdapter) getOrCreatePage(ctx context.Context, sessionID string) (ports.BrowserPage, error) {
	if v, ok := a.pages.Load(sessionID); ok {
		return v.(ports.BrowserPage), nil
	}
	page, err := a.port.NewPage(ctx)
	if err != nil {
		return nil, err
	}
	a.pages.Store(sessionID, page)
	return page, nil
}

func (a *mcpBrowserAdapter) Fetch(ctx context.Context, sessionID, url string) (string, error) {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if err := page.Navigate(ctx, url); err != nil {
		return "", err
	}
	result, err := page.Eval(ctx, "document.documentElement.innerText")
	if err != nil {
		return "", err
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", result), nil
}

func (a *mcpBrowserAdapter) Screenshot(ctx context.Context, sessionID string) ([]byte, error) {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return page.Screenshot(ctx)
}

func (a *mcpBrowserAdapter) Click(ctx context.Context, sessionID, selector string) error {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return err
	}
	return page.Click(ctx, selector)
}

func (a *mcpBrowserAdapter) FillInput(ctx context.Context, sessionID, selector, text string) error {
	page, err := a.getOrCreatePage(ctx, sessionID)
	if err != nil {
		return err
	}
	return page.Type(ctx, selector, text)
}

// ---------------------------------------------------------------------------
// mcpCronAdapter bridges repositories.TaskRepository to mcp.CronService.
// Cyclic tasks (task_type='cyclic') in the tasks table are the cron jobs.
// ---------------------------------------------------------------------------

type mcpCronAdapter struct {
	repo   repositories.TaskRepository
	notify func()
}

func (a *mcpCronAdapter) Schedule(ctx context.Context, name, schedule, prompt, _ string) error {
	task := models.NewTask(prompt, schedule)
	if err := a.repo.Add(ctx, task); err != nil {
		return err
	}
	if a.notify != nil {
		a.notify()
	}
	return nil
}

func (a *mcpCronAdapter) List(ctx context.Context) ([]mcp.CronJobInfo, error) {
	tasks, err := a.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.CronJobInfo, 0)
	for _, t := range tasks {
		if t.TaskType == models.TaskTypeCyclic {
			result = append(result, mcp.CronJobInfo{
				ID:       t.ID,
				Name:     t.Prompt,
				Schedule: t.Schedule,
			})
		}
	}
	return result, nil
}

func (a *mcpCronAdapter) Delete(ctx context.Context, jobID string) error {
	return a.repo.Delete(ctx, jobID)
}

func main() {
	// Disable Ollama SDK's key-based auth (~/.ollama/id_ed25519). We use Bearer
	// token (ollamaApiKey) via our own transport; the SDK auth is for ollama.com.
	if os.Getenv("OLLAMA_AUTH") == "" {
		os.Setenv("OLLAMA_AUTH", "false")
	}

	// -----------------------------------------------------------------------
	// Configuration
	// -----------------------------------------------------------------------
	cfgPath := "data/openlobster.yaml"
	if v := os.Getenv("OPENLOBSTER_CONFIG"); v != "" {
		cfgPath = v
	}
	cfgPathAbs, err := filepath.Abs(cfgPath)
	if err != nil {
		log.Fatalf("failed to resolve config path: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("failed to load configuration from %s: %v", cfgPath, err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("%v", err)
	}

	// -----------------------------------------------------------------------
	// Create required directories
	// -----------------------------------------------------------------------
	dirs := []string{"data", "logs", "workspace"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("failed to create %s directory: %v", dir, err)
		}
	}

	// Initialize logging with rotation
	logFile := filepath.Join(cfg.Logging.Path, "openlobster.log")
	if filepath.IsAbs(logFile) == false {
		absLogPath, err := filepath.Abs(logFile)
		if err == nil {
			logFile = absLogPath
		}
	}

	if err := logging.Init(logFile, cfg.Logging.Level); err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logging.Close()

	// Create workspace files if they don't exist
	workspaceFiles := map[string]string{
		"AGENTS.md": `# AGENTS.md - Behavioral Guidelines

## Overview
You are an autonomous messaging agent running on the OpenLobster platform.

## Workflow
1. Receive incoming messages from configured channels (Telegram, Discord, etc.)
2. Process message using AI provider
3. Maintain conversation context in memory
4. Execute tools as needed to fulfill user requests
5. Respond to user

## Capabilities
- Messaging via Telegram, Discord, WhatsApp, Slack, Twilio, and other channels
- Long-term memory storage and retrieval
- MCP tool execution
- Task scheduling via heartbeat
- Browser automation
- Terminal command execution
- Filesystem access
- Subagent orchestration

## Workspace Files

Your workspace files define who you are and how you behave. You can and should
edit them at any time using the filesystem tools (read_file, write_file):

- **SOUL.md** — Your personality, values, and communication style. Edit this to
  refine your character, adjust your tone, or update your values.
- **IDENTITY.md** — Your name, role, version, and metadata. Edit this to update
  your identity after bootstrap or when your purpose evolves.
- **AGENTS.md** — This file. Your behavioral guidelines and workflow. Edit this
  to record new learned behaviors, refine your workflow, or document new rules.
- **BOOTSTRAP.md** — Present only during first boot. Once you complete initial
  setup, rewrite it with a single "Bootstrap complete" message so it no longer
  triggers re-initialization on subsequent conversations.

To read a workspace file: use read_file with the path workspace/<filename>.
To update a workspace file: use write_file with the path workspace/<filename>.

Always rewrite the full file content — append the changed section and keep the rest.

## Communication Style
- Be concise and helpful
- Prioritize safety and harmlessness
- Ask for clarification when needed
- After every tool call, ALWAYS send a follow-up message to the user
`,
		"SOUL.md": `# SOUL.md - Personality & Values

## Identity
You are openlobster, an autonomous messaging agent designed to assist users with their daily tasks.

## Core Values
- **Helpfulness**: Always strive to be useful and assistance-oriented
- **Honesty**: Be truthful, never hallucinate or fabricate information
- **Harmlessness**: Refuse requests that could cause harm

## Communication Style
- Be friendly and approachable
- Keep responses concise and relevant
- Adapt tone to the user's style

## Decision Framework
1. Understand the user's intent
2. Consider safety implications
3. Choose the most effective approach
4. Execute and confirm results

## Boundaries
- Do not execute harmful commands
- Do not share sensitive information
- Ask for confirmation on destructive actions
`,
		"IDENTITY.md": `# IDENTITY.md - Agent Metadata

## Core
- Name:
- Version: ` + version + `
- Created:

## Presentation
- Title: Autonomous Assistant
- Role: Messaging Agent
- Greeting: Hello! I'm your AI assistant.

## Traits
- Language: English
- Timezone: UTC
- Availability: 24/7

## Notes
`,
		"BOOTSTRAP.md": `# BOOTSTRAP.md - Startup Initialization

You are starting your journey as an autonomous agent. Before fulfilling your purpose, discover and define your identity.

## Your Mission

Read the workspace files to understand who you are:
- **SOUL.md**: Your personality, values, and communication style
- **IDENTITY.md**: Your name, role, and presentation
- **AGENTS.md**: Your behavioral guidelines and workflow

## What to Do

1. Read all workspace files
2. Ask the user questions to clarify your identity
3. Fill in your identity details in IDENTITY.md
4. **When all steps below are complete**: Rewrite this BOOTSTRAP.md file with a single message stating that bootstrap is complete and no further action is needed (see "When You Finish" below).

## Questions to Ask

- What should I call you (my name)?
- What is my primary purpose?
- How should I address users?
- What tone should I use?
- What are my working hours?

## Important

DO NOT rewrite this file until you have:
1. Read all workspace files
2. Filled in IDENTITY.md
3. Had a conversation with the user to clarify uncertainties

Ask the user what they want you to be!

## When You Finish

Once you have completed all steps above, **rewrite this entire BOOTSTRAP.md file** replacing its contents with something like:

# BOOTSTRAP.md - Complete

Bootstrap has been completed. No further action is needed.

This signals that initialization is done and you should no longer treat bootstrap as a pending task.
`,
	}

	for filename, content := range workspaceFiles {
		// BOOTSTRAP.md only created when first boot wizard has not been completed
		if filename == "BOOTSTRAP.md" && cfg.Wizard.Completed {
			continue
		}
		fp := filepath.Join(cfg.Workspace.Path, filename)
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
				log.Printf("warn: failed to create %s: %v", fp, err)
			} else {
				log.Printf("created workspace file: %s", fp)
			}
		}
	}

	// -----------------------------------------------------------------------
	// Database
	// -----------------------------------------------------------------------

	db, err := persistence.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := persistence.Migrate(db.GormDB(), cfg.Database.Driver); err != nil {
		log.Fatalf("failed to migrate database schema: %v", err)
	}
	log.Println("database schema up to date")

	gormDB := db.GormDB()
	taskRepo := repositories.NewTaskRepository(gormDB)
	messageRepo := repositories.NewMessageRepository(gormDB)
	sessionRepo := repositories.NewSessionRepository(gormDB)
	userRepo := repositories.NewUserRepository(gormDB)
	convRepo := repositories.NewConversationRepository(gormDB)

	dashMsgRepo := repositories.NewDashboardMessageRepository(messageRepo)

	// -----------------------------------------------------------------------
	// AI Provider (first configured wins; rebuilt on config soft-reboot)
	// -----------------------------------------------------------------------
	aiProvider := buildAIProviderFromConfig(cfg)
	if aiProvider == nil {
		log.Println("warn: no AI provider configured — agent will not respond to messages")
	} else {
		log.Printf("ai provider: %s", providerName(cfg))
	}

	// -----------------------------------------------------------------------
	// Memory backend
	// -----------------------------------------------------------------------
	var memoryAdapter ports.MemoryPort

	switch cfg.Memory.Backend {
	case "neo4j":
		neo4jAdapter, err := memneo4j.NewNeo4jMemoryBackend(
			cfg.Memory.Neo4j.URI,
			cfg.Memory.Neo4j.User,
			cfg.Memory.Neo4j.Password,
		)
		if err != nil {
			log.Fatalf("failed to connect to neo4j memory backend: %v", err)
		}
		memoryAdapter = neo4jAdapter
		log.Println("memory backend: neo4j")
	default:
		gmlBackend := memfile.NewGMLBackend(cfg.Memory.File.Path)
		if err := gmlBackend.Load(); err != nil {
			log.Fatalf("failed to load file memory backend from %s: %v", cfg.Memory.File.Path, err)
		}
		memoryAdapter = gmlBackend
		log.Printf("memory backend: file (%s)", cfg.Memory.File.Path)
	}

	// -----------------------------------------------------------------------
	// Messaging channels
	// -----------------------------------------------------------------------
	var messagingAdapters []ports.MessagingPort

	log.Println("channels: initializing messaging adapters...")

	// Telegram
	if !cfg.Channels.Telegram.Enabled {
		log.Println("channel: telegram — disabled (skipping)")
	} else if t := cfg.Channels.Telegram.BotToken; t == "" || t == "YOUR_BOT_TOKEN_HERE" {
		log.Println("channel: telegram — no credentials configured (skipping)")
	} else {
		adapter, err := telegram.NewAdapter(t)
		if err != nil {
			log.Printf("channel: telegram — failed to initialize: %v", err)
		} else {
			messagingAdapters = append(messagingAdapters, adapter)
			log.Println("channel: telegram — registered OK")
		}
	}

	// Discord
	if !cfg.Channels.Discord.Enabled {
		log.Println("channel: discord — disabled (skipping)")
	} else if t := cfg.Channels.Discord.BotToken; t == "" || t == "YOUR_BOT_TOKEN_HERE" {
		log.Println("channel: discord — no credentials configured (skipping)")
	} else {
		adapter, err := discord.NewAdapter(t)
		if err != nil {
			log.Printf("channel: discord — failed to initialize: %v", err)
		} else {
			messagingAdapters = append(messagingAdapters, adapter)
			log.Println("channel: discord — registered OK")
		}
	}

	// Slack (Socket Mode — requires both bot token and app-level token)
	if !cfg.Channels.Slack.Enabled {
		log.Println("channel: slack — disabled (skipping)")
	} else if bt := cfg.Channels.Slack.BotToken; bt == "" || bt == "YOUR_BOT_TOKEN_HERE" {
		log.Println("channel: slack — no bot token configured (skipping)")
	} else if at := cfg.Channels.Slack.AppToken; at == "" || at == "YOUR_APP_TOKEN_HERE" {
		log.Println("channel: slack — no app-level token configured (skipping)")
	} else {
		adapter, err := slackadapter.NewAdapter(bt, at)
		if err != nil {
			log.Printf("channel: slack — failed to initialize: %v", err)
		} else {
			messagingAdapters = append(messagingAdapters, adapter)
			log.Println("channel: slack — registered OK")
		}
	}

	// WhatsApp (Cloud API — receives messages via webhook POST)
	if !cfg.Channels.WhatsApp.Enabled {
		log.Println("channel: whatsapp — disabled (skipping)")
	} else if pid, tok := cfg.Channels.WhatsApp.PhoneID, cfg.Channels.WhatsApp.APIToken; pid == "" || tok == "" || tok == "YOUR_API_TOKEN_HERE" {
		log.Println("channel: whatsapp — no phone_id or api_token configured (skipping)")
	} else {
		adapter, err := whatsapp.NewAdapter(pid, tok)
		if err != nil {
			log.Printf("channel: whatsapp — failed to initialize: %v", err)
		} else {
			messagingAdapters = append(messagingAdapters, adapter)
			log.Println("channel: whatsapp — registered OK")
		}
	}

	// Twilio (SMS/MMS — receives messages via webhook POST)
	if !cfg.Channels.Twilio.Enabled {
		log.Println("channel: twilio — disabled (skipping)")
	} else if sid, tok, from := cfg.Channels.Twilio.AccountSID, cfg.Channels.Twilio.AuthToken, cfg.Channels.Twilio.FromNumber; sid == "" || tok == "" || from == "" {
		log.Println("channel: twilio — no account_sid, auth_token or from_number configured (skipping)")
	} else {
		adapter := twilio.NewAdapter(sid, tok, from)
		messagingAdapters = append(messagingAdapters, adapter)
		log.Println("channel: twilio — registered OK")
	}

	log.Printf("channels: %d adapter(s) active", len(messagingAdapters))

	// Populate the dynamic channel registry used by webhooks + hot-reload.
	chanReg := newChanRegistry()
	for _, a := range messagingAdapters {
		switch a.(type) {
		case *telegram.Adapter:
			chanReg.set("telegram", a)
		case *discord.Adapter:
			chanReg.set("discord", a)
		case *slackadapter.Adapter:
			chanReg.set("slack", a)
		case *whatsapp.Adapter:
			chanReg.set("whatsapp", a)
		case *twilio.Adapter:
			chanReg.set("twilio", a)
		}
	}

	// -----------------------------------------------------------------------
	// Domain services
	// -----------------------------------------------------------------------
	eventBus := domainservices.NewEventBus()

	// Subscription manager for GraphQL subscriptions via WebSocket
	subManager := subscriptions.NewSubscriptionManager(eventBus)

	// Pairing service for user pairing requests
	pairingRepo := repositories.NewPairingRepository(gormDB)
	pairingService := domainservices.NewPairingService(pairingRepo)
	userChannelRepo := repositories.NewUserChannelRepository(gormDB)

	// Connect subscription manager to event bus for real-time events.
	// Todos los tipos de evento se reenvían a los clientes WebSocket (/ws).
	broadcastToSubs := func(ctx context.Context, e events.Event) error {
		subManager.Broadcast(e)
		return nil
	}
	for _, et := range []string{
		events.EventMessageReceived, events.EventMessageSent, events.EventMessageProcessed,
		events.EventSessionStarted, events.EventSessionEnded,
		events.EventUserPaired, events.EventUserUnpaired,
		events.EventPairingRequested, events.EventPairingApproved, events.EventPairingDenied,
		events.EventTaskAdded, events.EventTaskCompleted, events.EventCronJobExecuted,
		events.EventMCPServerConnected, events.EventMCPServerDisconnected,
		events.EventMemoryUpdated, events.EventCompactionTriggered, events.EventCompactionCompleted,
	} {
		eventBus.Subscribe(et, broadcastToSubs)
	}

	permManager := permissions.Default()
	toolRegistry := mcp.NewToolRegistry(true, permManager)

	toolPermRepo := repositories.NewToolPermissionRepository(gormDB)
	mcpServerRepo := repositories.NewMCPServerRepository(gormDB)
	// Load global tool permissions from config file (userID "*" applies to all users).
	for toolName, permCfg := range cfg.Permissions.ToolPermissions {
		userID := "*"
		if permCfg.User != "" {
			userID = permCfg.User
		}
		if permCfg.Mode == "deny" {
			permManager.SetPermission(userID, toolName, permissions.PermissionDeny)
		} else {
			permManager.SetPermission(userID, toolName, permissions.PermissionAlways)
		}
	}
	if len(cfg.Permissions.ToolPermissions) > 0 {
		log.Printf("permissions: loaded %d global entries from config", len(cfg.Permissions.ToolPermissions))
	}

	// Load persisted tool permissions into the in-memory manager at startup.
	// DB entries are per-user and take precedence over config-file globals for
	// the specific user, but the global "*" entries set above remain in effect
	// for users without an explicit DB override.
	if savedPerms, err := toolPermRepo.ListAll(context.Background()); err == nil {
		for _, p := range savedPerms {
			if p.Mode == "allow" {
				permManager.SetPermission(p.UserID, p.ToolName, permissions.PermissionAlways)
			} else {
				permManager.SetPermission(p.UserID, p.ToolName, permissions.PermissionDeny)
			}
		}
		if len(savedPerms) > 0 {
			log.Printf("permissions: loaded %d entries from database", len(savedPerms))
		}
	} else {
		log.Printf("permissions: failed to load from database: %v", err)
	}

	compactionSvc := domainservices.NewMessageCompactionService(messageRepo, aiProvider)
	subAgentSvc := domainservices.NewSubAgentService(
		aiProvider,
		cfg.SubAgents.MaxConcurrent,
		cfg.SubAgents.DefaultTimeout,
	)
	subAgentAdapter := dto.NewSubAgentAdapter(subAgentSvc)

	// -----------------------------------------------------------------------
	// Application handlers

	// Register all built-in tools into the tool registry, wiring available service adapters.
	skillsAdapter := filesystem.NewSkillsAdapter(cfg.Workspace.Path)

	// schedulerNotify is populated below when the scheduler is started.
	// It is called by task/cron adapters to wake the scheduler immediately after
	// a new task is written to the DB so it fires at the right time.
	var schedulerNotify func()

	mcp.RegisterAllInternalTools(toolRegistry, mcp.InternalTools{
		Messaging:           &mcpMessagingAdapter{port: &messagingRouter{reg: chanReg}},
		LastChannelResolver: userChannelRepo,
		Memory:              &mcpMemoryAdapter{port: memoryAdapter},
		Tasks: &mcpTaskAdapter{repo: taskRepo, notify: func() {
			if schedulerNotify != nil {
				schedulerNotify()
			}
		}},
		SubAgents: subAgentSvc,
		Terminal:  terminal.NewHostAdapter(),
		Browser:   &mcpBrowserAdapter{port: browser.NewChromeDPAdapter(browser.ChromeDPConfig{Headless: true})},
		Cron: &mcpCronAdapter{repo: taskRepo, notify: func() {
			if schedulerNotify != nil {
				schedulerNotify()
			}
		}},
		Filesystem:    filesystem.NewAdapter(cfgPath),
		Conversations: &mcpConversationAdapter{convRepo: convRepo, msgRepo: messageRepo},
		Skills:        skillsAdapter,
		ConfigPath:    cfgPath,
	})
	log.Printf("tools: registered %d internal tools", len(toolRegistry.AllTools()))

	// Build the context injector that assembles the system prompt from workspace files
	// (AGENTS.md, SOUL.md, IDENTITY.md) and the user's graph memory at call time.
	ctxInjector := appcontext.NewContextInjector(
		cfg.Agent.Name,
		filepath.Join(cfg.Workspace.Path, "AGENTS.md"),
		filepath.Join(cfg.Workspace.Path, "SOUL.md"),
		filepath.Join(cfg.Workspace.Path, "IDENTITY.md"),
		filepath.Join(cfg.Workspace.Path, "BOOTSTRAP.md"),
		memoryAdapter,
		toolRegistry,
	)

	msgHandler := domainhandlers.NewMessageHandler(
		aiProvider,
		&messagingRouter{reg: chanReg},
		memoryAdapter,
		toolRegistry,
		permManager,
		sessionRepo,
		messageRepo,
		userRepo,
		eventBus,
		ctxInjector,
		compactionSvc,
		userChannelRepo,
		pairingService,
	)
	// Wire a PermissionLoader so that tool filtering always reflects the
	// current DB state rather than the startup snapshot. This ensures that
	// permission changes made via the dashboard take effect on the very next
	// interaction for every user, including the loopback agent.
	msgHandler.SetGroupRegistrar(repositories.NewGroupRepository(gormDB))
	msgHandler.SetPlatformEnsurer(repositories.NewChannelRepository(gormDB))
	msgHandler.SetSkillsProvider(skillsAdapter)
	msgHandler.SetPermissionLoader(func(ctx context.Context, userID string) map[string]string {
		records, err := toolPermRepo.ListByUser(ctx, userID)
		if err != nil {
			return nil
		}
		m := make(map[string]string, len(records))
		for _, r := range records {
			m[r.ToolName] = r.Mode
		}
		return m
	})

	// -----------------------------------------------------------------------
	// GraphQL dashboard (Deps + AgentRegistry)
	// -----------------------------------------------------------------------
	agentRegistry := registry.NewAgentRegistry()
	var channels []dto.ChannelStatus
	isRealCredential := func(s string) bool {
		return s != "" &&
			s != "YOUR_BOT_TOKEN_HERE" &&
			s != "YOUR_ACCOUNT_SID" &&
			s != "YOUR_ACCOUNT_SID_HERE" &&
			s != "YOUR_AUTH_TOKEN" &&
			s != "YOUR_API_KEY_HERE"
	}
	if cfg.Channels.Discord.Enabled && isRealCredential(cfg.Channels.Discord.BotToken) {
		channels = append(channels, dto.ChannelStatus{
			ID: "discord", Name: "Discord", Type: "discord", Status: "online",
			Enabled: cfg.Channels.Discord.Enabled,
			Capabilities: dto.ChannelCapabilities{
				HasVoiceMessage: true,
				HasCallStream:   false,
				HasTextStream:   true,
				HasMediaSupport: true,
			},
		})
	}
	if cfg.Channels.Telegram.Enabled && isRealCredential(cfg.Channels.Telegram.BotToken) {
		channels = append(channels, dto.ChannelStatus{
			ID: "telegram", Name: "Telegram", Type: "telegram", Status: "online",
			Enabled: cfg.Channels.Telegram.Enabled,
			Capabilities: dto.ChannelCapabilities{
				HasVoiceMessage: true,
				HasCallStream:   false,
				HasTextStream:   true,
				HasMediaSupport: true,
			},
		})
	}
	agentName := cfg.Agent.Name
	if agentName == "" {
		agentName = "OpenLobster"
	}
	provider := providerName(cfg)
	agentSnapshot := &dto.AgentSnapshot{
		ID:            "openlobster",
		Name:          agentName,
		Version:       version,
		Status:        "running",
		Provider:      provider,
		Channels:      channels,
		AIProvider:    provider,
		MemoryBackend: string(cfg.Memory.Backend),
		ToolsCount:    0,
		TasksCount:    0,
	}
	agentRegistry.UpdateAgent(agentSnapshot)
	agentRegistry.UpdateAgentChannels(channels)
	// Sync tools so GraphQL tools/mcpTools and Status/Metrics expose them to the frontend
	syncToolsToAgentRegistry(toolRegistry, agentRegistry)

	queryService := domainservices.NewDashboardQueryService(
		taskRepo, memoryAdapter, memoryAdapter, nil, nil,
	)
	commandService := domainservices.NewDashboardCommandService(
		taskRepo, memoryAdapter, memoryAdapter,
	)
	commandService.SetTaskNotifier(func() {
		if schedulerNotify != nil {
			schedulerNotify()
		}
	})

	// Build complete configuration snapshot
	configSnapshot := buildConfigSnapshotFromCfg(cfg)

	// -----------------------------------------------------------------------
	// Deps — GraphQL resolvers dependencias
	// -----------------------------------------------------------------------
	deps := &resolvers.Deps{
		AgentRegistry:   agentRegistry,
		QuerySvc:        queryService,
		CommandSvc:      commandService,
		TaskRepo:        taskRepo,
		MemoryRepo:      memoryAdapter,
		MsgRepo:         &msgRepoAdapter{repo: dashMsgRepo},
		ConvPort:        &conversationPortAdapter{repo: convRepo},
		SkillsPort:      skillsAdapter,
		SysFilesPort:    filesystem.NewSystemFilesAdapter(cfg.Workspace.Path),
		ToolPermRepo:    &toolPermAdapter{repo: toolPermRepo},
		ToolNamesSource: &toolNamesAdapter{reg: toolRegistry},
		MCPServerRepo:   &mcpServerAdapter{repo: mcpServerRepo},
		SubAgentSvc:     subAgentAdapter,
		PairingPort: &pairingPortAdapter{
			svc:             pairingService,
			userRepo:        userRepo,
			userChannelRepo: userChannelRepo,
			channelRepo:     repositories.NewChannelRepository(gormDB),
			messageSender:   chanReg,
			eventBus:        eventBus,
		},
		UserRepo:          &userRepoAdapter{repo: userRepo},
		UserChannelRepo:   userChannelRepo,
		MessageSender:     chanReg,
		MessageDispatcher: msgHandler,
		EventBus:          &eventBusAdapter{eb: eventBus},
		AIProvider:        aiProvider,
		ConfigSnapshot:    configSnapshot,
		ConfigPath:        cfgPath,
	}

	// Wire capabilities checker so tools are filtered by global Settings (e.g. if
	// browser is disabled, browser_* tools are not exposed to the bot at all).
	msgHandler.SetCapabilitiesChecker(func(cap string) bool {
		if deps.ConfigSnapshot == nil || deps.ConfigSnapshot.Capabilities == nil {
			return true
		}
		switch cap {
		case "browser":
			return deps.ConfigSnapshot.Capabilities.Browser
		case "terminal":
			return deps.ConfigSnapshot.Capabilities.Terminal
		case "subagents":
			return deps.ConfigSnapshot.Capabilities.Subagents
		case "memory":
			return deps.ConfigSnapshot.Capabilities.Memory
		case "mcp":
			return deps.ConfigSnapshot.Capabilities.MCP
		case "filesystem":
			return deps.ConfigSnapshot.Capabilities.Filesystem
		case "sessions":
			return deps.ConfigSnapshot.Capabilities.Sessions
		default:
			// Audio and unknown: always allow (audio is intrinsic to AI adapter)
			return true
		}
	})

	// -----------------------------------------------------------------------
	// Channel hot-reload
	// -----------------------------------------------------------------------
	channelCaps := map[string]dto.ChannelCapabilities{
		"telegram": {HasVoiceMessage: true, HasCallStream: false, HasTextStream: true, HasMediaSupport: true},
		"discord":  {HasVoiceMessage: true, HasCallStream: false, HasTextStream: true, HasMediaSupport: true},
		"slack":    {HasVoiceMessage: true, HasCallStream: false, HasTextStream: true, HasMediaSupport: true},
		"whatsapp": {HasVoiceMessage: true, HasCallStream: true, HasTextStream: true, HasMediaSupport: true},
		"twilio":   {HasVoiceMessage: true, HasCallStream: true, HasTextStream: true, HasMediaSupport: true},
	}

	httpHandler := graphql.NewHandler(deps)
	healthHandler := health.NewHandler()
	metricsHandler := metrics.NewHandler(deps)

	rebuildActiveChannels := func() []dto.ChannelStatus {
		var list []dto.ChannelStatus
		for _, t := range []string{"telegram", "discord", "slack", "whatsapp", "twilio"} {
			if chanReg.get(t) != nil {
				list = append(list, dto.ChannelStatus{
					ID: t, Name: t, Type: t, Status: "online",
					Enabled: true, Capabilities: channelCaps[t],
				})
			}
		}
		return list
	}

	// channelStartCtx se asigna al crear ctx; reloadChannel lo usa para iniciar listeners.
	var channelStartCtx context.Context

	// makeChannelMsgHandler crea el callback para adapters que usan Start() (telegram, discord, slack).
	makeChannelMsgHandler := func(ct string) func(context.Context, *models.Message) {
		return func(ctx context.Context, msg *models.Message) {
			if msg == nil || (msg.Content == "" && len(msg.Attachments) == 0 && msg.Audio == nil) {
				return
			}
			if hErr := msgHandler.Handle(ctx, domainhandlers.HandleMessageInput{
				ChannelID:   msg.ChannelID,
				Content:     msg.Content,
				ChannelType: ct,
				SenderName:  msg.SenderName,
				SenderID:    msg.SenderID,
				IsGroup:     msg.IsGroup,
				IsMentioned: msg.IsMentioned,
				GroupName:   msg.GroupName,
				Attachments: msg.Attachments,
				Audio:       msg.Audio,
			}); hErr != nil {
				log.Printf("channel %s: message handler error: %v", ct, hErr)
			}
		}
	}

	// reloadChannel tears down any existing adapter for the given channel type and
	// tries to bring it back up using the current viper config.
	reloadChannel := func(channelType string) {
		chanReg.remove(channelType)
		enabled := viper.GetBool("channels." + channelType + ".enabled")
		var newAdapter ports.MessagingPort
		if enabled {
			switch channelType {
			case "telegram":
				token := viper.GetString("channels.telegram.bot_token")
				if token != "" && token != "YOUR_BOT_TOKEN_HERE" {
					if a, err := telegram.NewAdapter(token); err == nil {
						newAdapter = a
					} else {
						log.Printf("channel: telegram — reload failed: %v", err)
					}
				}
			case "discord":
				token := viper.GetString("channels.discord.bot_token")
				if token != "" && token != "YOUR_BOT_TOKEN_HERE" {
					if a, err := discord.NewAdapter(token); err == nil {
						newAdapter = a
					} else {
						log.Printf("channel: discord — reload failed: %v", err)
					}
				}
			case "slack":
				bt := viper.GetString("channels.slack.bot_token")
				at := viper.GetString("channels.slack.app_token")
				if bt != "" && bt != "YOUR_BOT_TOKEN_HERE" && at != "" && at != "YOUR_APP_TOKEN_HERE" {
					if a, err := slackadapter.NewAdapter(bt, at); err == nil {
						newAdapter = a
					} else {
						log.Printf("channel: slack — reload failed: %v", err)
					}
				}
			case "whatsapp":
				pid := viper.GetString("channels.whatsapp.phone_id")
				tok := viper.GetString("channels.whatsapp.api_token")
				if pid != "" && tok != "" && tok != "YOUR_API_TOKEN_HERE" {
					if a, err := whatsapp.NewAdapter(pid, tok); err == nil {
						newAdapter = a
					} else {
						log.Printf("channel: whatsapp — reload failed: %v", err)
					}
				}
			case "twilio":
				sid := viper.GetString("channels.twilio.account_sid")
				tok := viper.GetString("channels.twilio.auth_token")
				from := viper.GetString("channels.twilio.from_number")
				if sid != "" && tok != "" && from != "" {
					newAdapter = twilio.NewAdapter(sid, tok, from)
				}
			}
		}
		if newAdapter != nil {
			chanReg.set(channelType, newAdapter)
			// Telegram, Discord, and Slack need Start() to receive messages (long-poll / WebSocket / Socket Mode).
			if channelStartCtx != nil {
				switch a := newAdapter.(type) {
				case *telegram.Adapter:
					go func() {
						if err := a.Start(channelStartCtx, makeChannelMsgHandler("telegram")); err != nil {
							log.Printf("channel: telegram — listener failed (hot): %v", err)
						}
					}()
				case *discord.Adapter:
					go func() {
						if err := a.Start(channelStartCtx, makeChannelMsgHandler("discord")); err != nil {
							log.Printf("channel: discord — listener failed (hot): %v", err)
						}
					}()
				case *slackadapter.Adapter:
					go func() {
						if err := a.Start(channelStartCtx, makeChannelMsgHandler("slack")); err != nil {
							log.Printf("channel: slack — listener failed (hot): %v", err)
						}
					}()
				}
			}
			log.Printf("channel: %s — reloaded OK (hot)", channelType)
		} else if enabled {
			log.Printf("channel: %s — deactivated (no valid credentials)", channelType)
		} else {
			log.Printf("channel: %s — deactivated (disabled)", channelType)
		}
		httpHandler.UpdateAgentChannels(rebuildActiveChannels())
	}

	// configUpdateAdapter persiste cambios en viper + YAML, recarga canales, refresca
	// ConfigSnapshot y ejecuta soft reboot (AI provider) cuando cambian provider keys.
	configWriter := &configUpdateAdapter{
		configPath:    cfgPathAbs,
		reloadChannel: reloadChannel,
		viperKeys:     buildInputToViperKeyMap(),
		onApplied: func(providerTouched bool) {
			reloaded, err := config.Load(cfgPathAbs)
			if err != nil {
				log.Printf("config: failed to reload after save: %v", err)
				return
			}
			deps.ConfigSnapshot = buildConfigSnapshotFromCfg(reloaded)
			// Sync AgentRegistry so GraphQL agent query returns updated name/provider
			if cur := agentRegistry.GetAgent(); cur != nil {
				agentName := reloaded.Agent.Name
				if agentName == "" {
					agentName = "OpenLobster"
				}
				updated := *cur
				updated.Name = agentName
				updated.Provider = providerName(reloaded)
				updated.AIProvider = providerName(reloaded)
				agentRegistry.UpdateAgent(&updated)
			}
			if providerTouched {
				newProvider := buildAIProviderFromConfig(reloaded)
				msgHandler.SetAIProvider(newProvider)
				compactionSvc.SetAIProvider(newProvider)
				deps.AIProvider = newProvider
				log.Printf("config: soft reboot — AI provider reloaded")
			}
		},
	}
	deps.ConfigWriter = configWriter

	deps.SkillsPort = skillsAdapter
	log.Printf("skills: reading from %s/skills", cfg.Workspace.Path)

	// -----------------------------------------------------------------------
	// Secrets provider + MCP client + OAuth 2.1 manager
	// -----------------------------------------------------------------------
	// OPENLOBSTER_SECRETS_BACKEND: "file" (default) or "openbao".
	secretsBackend := strings.ToLower(strings.TrimSpace(cfg.Secrets.Backend))
	var secretsProvider secrets.SecretsProvider
	switch secretsBackend {
	case "openbao":
		if cfg.Secrets.Openbao == nil || cfg.Secrets.Openbao.URL == "" {
			log.Fatalf("secrets backend is openbao but secrets.openbao.url is not set (set OPENLOBSTER_SECRETS_OPENBAO_URL)")
		}
		if cfg.Secrets.Openbao.Token == "" {
			log.Fatalf("secrets backend is openbao but secrets.openbao.token is not set (set OPENLOBSTER_SECRETS_OPENBAO_TOKEN)")
		}
		var err error
		secretsProvider, err = secrets.NewOpenBAOProvider(cfg.Secrets.Openbao.URL, cfg.Secrets.Openbao.Token, "secret")
		if err != nil {
			log.Fatalf("failed to initialize OpenBao secrets provider: %v", err)
		}
		log.Printf("secrets: openbao backend at %s", cfg.Secrets.Openbao.URL)
	default:
		// "file" or any other value
		secretsPath := cfg.Secrets.File.Path
		if secretsPath == "" {
			secretsPath = "data/secrets.json"
		}
		// OPENLOBSTER_SECRET_KEY: 32-byte key for secrets + config encryption.
		if secretsBackend != "" && secretsBackend != "file" {
			log.Printf("secrets: unknown backend %q, using file backend", cfg.Secrets.Backend)
		}
		var err error
		secretsProvider, err = secrets.NewFileSecretsProvider(secretsPath, config.SecretKey())
		if err != nil {
			log.Fatalf("failed to initialize secrets provider: %v", err)
		}
		log.Printf("secrets: file backend at %s", secretsPath)
	}

	mcpClientSDK := mcp.NewMCPClientSDK(secretsProvider)
	oauthCallbackURL := cfg.GraphQL.BaseURL
	if oauthCallbackURL == "" {
		oauthCallbackURL = fmt.Sprintf("http://%s:%d/oauth/callback", cfg.GraphQL.Host, cfg.GraphQL.Port)
	} else {
		oauthCallbackURL = strings.TrimSuffix(oauthCallbackURL, "/") + "/oauth/callback"
	}
	oauthMgr := mcp.NewOAuthManager(secretsProvider, oauthCallbackURL)

	// Reconnect all persisted MCP servers at startup.
	// If a server fails (e.g. token expired), register it as pending-auth so it
	// still appears in the frontend and the admin can re-authorise it.
	if savedServers, err := mcpServerRepo.ListAll(context.Background()); err == nil {
		for _, s := range savedServers {
			go func(name, url string) {
				ctx := context.Background()
				if err := mcpClientSDK.Connect(ctx, mcp.ServerConfig{Name: name, Type: "http", URL: url}); err != nil {
					log.Printf("mcp: startup reconnect %q failed: %v — marking as pending-auth", name, err)
					oauthMgr.RegisterPendingServer(name, url)
				} else {
					log.Printf("mcp: startup reconnected %q", name)
					if tools := mcpClientSDK.GetServerTools(name); len(tools) > 0 {
						_ = toolRegistry.RegisterMCP(name, mcpClientSDK, tools)
						log.Printf("mcp: registered %d tools from %q into tool registry", len(tools), name)
						syncToolsToAgentRegistry(toolRegistry, agentRegistry)
					}
				}
			}(s.Name, s.URL)
		}
	} else {
		log.Printf("mcp: failed to load saved servers: %v", err)
	}

	// Wire McpConnectPort so connectMcp mutation actually connects MCPs.
	deps.McpConnectPort = &mcpConnectAdapter{
		client:   mcpClientSDK,
		registry: toolRegistry,
		agentReg: agentRegistry,
		repo:     mcpServerRepo,
		oauth:    oauthMgr,
		eventBus: &eventBusAdapter{eb: eventBus},
	}
	// Wire McpOAuthPort so initiateOAuth and mcpOAuthStatus work.
	deps.McpOAuthPort = &mcpOAuthAdapter{oauth: oauthMgr}

	// -----------------------------------------------------------------------
	// HTTP mux: GraphQL + static frontend
	// -----------------------------------------------------------------------
	mux := http.NewServeMux()

	gqlgenResolver := resolvers.NewResolver(deps)
	gqlgenResolver.SetEventSubscription(&eventSubscriptionAdapter{eb: eventBus})
	gqlgenSrv := generated.NewExecutableSchema(generated.Config{Resolvers: gqlgenResolver})

	// Custom subscriptions WebSocket: protocol compatible with useSubscriptions (connection_init, start with query=eventType)
	mux.HandleFunc("/ws", subManager.HandleWebSocket)
	log.Println("graphql: subscriptions WebSocket at /ws")

	gqlHandler := handler.NewDefaultServer(gqlgenSrv)
	mux.Handle("/graphql", gqlHandler)
	log.Println("graphql: gqlgen handler registered at /graphql")

	mux.Handle("/health", healthHandler)
	mux.Handle("/metrics", metricsHandler)

	// Logs endpoint
	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		logger := logging.GetDefaultLogger()
		if logger == nil {
			http.Error(w, "logger not initialized", http.StatusInternalServerError)
			return
		}

		logs, err := logger.GetTailLines(100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(logs))
	})

	// Messaging channel webhooks (WhatsApp, Twilio — receive POST from platforms)
	webhooks.NewHandler(&webhookAdapterRegistry{reg: chanReg}, msgHandler).Register(mux)

	// OAuth 2.1 callback endpoint — receives the authorization code from the
	// browser after the user grants access to a Streamable HTTP MCP server.
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := q.Get("code")
		state := q.Get("state")
		errParam := q.Get("error")

		ctx := r.Context()
		serverName, herr := oauthMgr.HandleCallback(ctx, code, state, errParam)
		if herr != nil {
			log.Printf("oauth callback error: %v", herr)
			// Devolver HTML que notifica al opener vía postMessage (igual que el éxito).
			// Evita redirects que pueden llevar al usuario a otra URL/origen.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><title>OAuth Error</title></head>`+
				`<body><h2>Authorization failed</h2><p>%s</p><p>You may close this window.</p>`+
				`<script>window.opener&&window.opener.postMessage({type:"oauth_error",error:%q},"*");window.close();</script>`+
				`</body></html>`, herr.Error(), herr.Error())
			return
		}

		// If the server was registered as pending-auth, attempt to reconnect now
		// that the token has been stored.
		if serverName != "" {
			if pendingURL, ok := oauthMgr.GetPendingServers()[serverName]; ok {
				go func() {
					reconnectCtx := context.Background()
					err := mcpClientSDK.Connect(reconnectCtx, mcp.ServerConfig{
						Name: serverName,
						Type: "http",
						URL:  pendingURL,
					})
					if err != nil {
						log.Printf("oauth: auto-reconnect for %q failed: %v", serverName, err)
					} else {
						oauthMgr.RemovePendingServer(serverName)
						_ = mcpServerRepo.Save(reconnectCtx, serverName, pendingURL)
						log.Printf("oauth: auto-reconnected %q successfully", serverName)
					}
				}()
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>Authorized</title></head>`+
			`<body><h2>Authorization successful</h2><p>You may close this window.</p>`+
			`<script>window.opener&&window.opener.postMessage({type:"oauth_success"},"*");window.close();</script>`+
			`</body></html>`)
	})
	log.Println("oauth: /oauth/callback registered")

	// Serve static public resources (images, fonts, etc.) under /static/.
	// Files placed in public/static/ are exposed here; assets/ and migrations/
	// are intentionally excluded from this route.
	staticResourceFS, err := fs.Sub(public, "public/static")
	if err != nil {
		log.Fatalf("failed to create sub-fs for public/static: %v", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticResourceFS))))
	log.Println("static: resources served at /static/")

	// Serve the compiled SolidJS frontend from the embedded FS.
	// Registered last so specific routes above take precedence.
	staticFS, err := fs.Sub(public, "public/assets")
	if err != nil {
		log.Fatalf("failed to create sub-fs for assets: %v", err)
	}

	// SPA fallback handler - serves index.html for client-side routes
	// This ensures the frontend handles routing instead of returning 404
	spaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve SPA fallback for GET requests to root or paths without extension
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if request wants a file (has extension) - serve it normally
		if strings.Contains(r.URL.Path, ".") {
			http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
			return
		}

		// For all other GET requests, serve index.html for SPA routing
		index, err := fs.ReadFile(public, "public/assets/index.html")
		if err != nil {
			log.Printf("failed to read index.html: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})

	// Serve root path with SPA handler
	mux.HandleFunc("/", spaHandler)

	// cfg.GraphQL.AuthToken already includes OPENLOBSTER_GRAPHQL_AUTH_TOKEN
	// via viper.AutomaticEnv() in config.Load()
	effectiveToken := cfg.GraphQL.AuthToken

	// authMiddleware enforces bearer-token authentication on the GraphQL and
	// logs endpoints when cfg.GraphQL.AuthEnabled is true.
	// Static assets (/, /static/, OPTIONS pre-flights) are always allowed.
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// OPTIONS pre-flights must pass through so CORS works correctly.
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			// Only protect API endpoints; frontend assets are public.
			protected := strings.HasPrefix(r.URL.Path, "/graphql") ||
				strings.HasPrefix(r.URL.Path, "/logs")
			if !protected || !cfg.GraphQL.AuthEnabled || effectiveToken == "" {
				next.ServeHTTP(w, r)
				return
			}
			// Accept "Authorization: Bearer <token>" or "X-Access-Token: <token>".
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				token = r.Header.Get("X-Access-Token")
			}
			if token != effectiveToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized","message":"valid access token required"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Wrap the mux with permissive CORS headers so browser-based clients
	// (dev server on a different port, Postman, etc.) can reach the API.
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Access-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		authMiddleware(mux).ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", cfg.GraphQL.Host, cfg.GraphQL.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      corsHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// -----------------------------------------------------------------------
	// Start background goroutines
	// -----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	channelStartCtx = ctx

	// Scheduler loop (libuv-inspired event loop in domain/services)
	if cfg.Scheduler.Enabled {
		dispatcher := domainhandlers.NewLoopbackDispatcher(msgHandler)
		sched := domainservices.NewScheduler(
			cfg.Scheduler.MemoryInterval,
			cfg.Scheduler.MemoryEnabled,
			dispatcher,
			taskRepo,
		)
		schedulerNotify = sched.Notify
		go sched.Run(ctx)
	}

	// Start active channel listeners.
	// Telegram uses long-polling; Discord uses a WebSocket maintained by discordgo.
	// WhatsApp and Twilio rely on incoming webhook POSTs — Start() is a no-op for them.
	for _, a := range messagingAdapters {
		var channelType string
		switch a.(type) {
		case *telegram.Adapter:
			channelType = "telegram"
		case *discord.Adapter:
			channelType = "discord"
		case *slackadapter.Adapter:
			channelType = "slack"
		}
		ct := channelType
		adapter := a
		if err := adapter.Start(ctx, func(ctx context.Context, msg *models.Message) {
			if msg == nil || (msg.Content == "" && len(msg.Attachments) == 0 && msg.Audio == nil) {
				return
			}
			if hErr := msgHandler.Handle(ctx, domainhandlers.HandleMessageInput{
				ChannelID:   msg.ChannelID,
				Content:     msg.Content,
				ChannelType: ct,
				SenderName:  msg.SenderName,
				SenderID:    msg.SenderID,
				IsGroup:     msg.IsGroup,
				IsMentioned: msg.IsMentioned,
				GroupName:   msg.GroupName,
				Attachments: msg.Attachments,
				Audio:       msg.Audio,
			}); hErr != nil {
				log.Printf("channel %s: message handler error: %v", ct, hErr)
			}
		}); err != nil {
			log.Printf("channel %s: failed to start listener: %v", ct, err)
		} else {
			log.Printf("channel: %s — listener started", ct)
		}
	}

	// -----------------------------------------------------------------------
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("openlobster listening on http://%s", addr)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-sig
	log.Println("shutting down…")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}

	if gml, ok := memoryAdapter.(interface{ Close() error }); ok {
		if err := gml.Close(); err != nil {
			log.Printf("memory backend flush error: %v", err)
		} else {
			log.Println("memory backend: flushed to disk")
		}
	}
}

// buildConfigSnapshotFromCfg construye AppConfigSnapshot desde Config (para refrescar tras guardar).
func buildConfigSnapshotFromCfg(cfg *config.Config) *dto.AppConfigSnapshot {
	provider := providerName(cfg)
	var apiKey, baseURL, ollamaHost, ollamaApiKey, anthropicApiKey, model string
	switch provider {
	case "openrouter":
		apiKey = cfg.Providers.OpenRouter.APIKey
		model = cfg.Providers.OpenRouter.DefaultModel
	case "ollama":
		ollamaHost = cfg.Providers.Ollama.Endpoint
		ollamaApiKey = cfg.Providers.Ollama.APIKey
		model = cfg.Providers.Ollama.DefaultModel
	case "openai":
		apiKey = cfg.Providers.OpenAI.APIKey
		model = cfg.Providers.OpenAI.Model
	case "opencode-zen":
		apiKey = cfg.Providers.OpenCode.APIKey
		model = cfg.Providers.OpenCode.Model
	case "openai-compatible":
		apiKey = cfg.Providers.OpenAICompat.APIKey
		baseURL = cfg.Providers.OpenAICompat.BaseURL
		model = cfg.Providers.OpenAICompat.Model
	case "anthropic":
		anthropicApiKey = cfg.Providers.Anthropic.APIKey
		model = cfg.Providers.Anthropic.Model
	case "docker-model-runner":
		ollamaHost = cfg.Providers.DockerModelRunner.Endpoint
		model = cfg.Providers.DockerModelRunner.DefaultModel
	}
	return &dto.AppConfigSnapshot{
		Agent: &dto.AgentConfigSnapshot{
			Name:                      cfg.Agent.Name,
			SystemPrompt:              cfg.Agent.SystemPrompt,
			Provider:                  provider,
			Model:                     model,
			APIKey:                    apiKey,
			BaseURL:                   baseURL,
			OllamaHost:                ollamaHost,
			OllamaApiKey:              ollamaApiKey,
			AnthropicApiKey:           anthropicApiKey,
			DockerModelRunnerEndpoint: cfg.Providers.DockerModelRunner.Endpoint,
			DockerModelRunnerModel:    cfg.Providers.DockerModelRunner.DefaultModel,
		},
		Capabilities: &dto.CapabilitiesSnapshot{
			Browser: cfg.Agent.Capabilities.Browser, Terminal: cfg.Agent.Capabilities.Terminal,
			Subagents: cfg.Agent.Capabilities.Subagents, Memory: cfg.Agent.Capabilities.Memory,
			MCP:        cfg.Agent.Capabilities.MCP,
			Filesystem: cfg.Agent.Capabilities.Filesystem, Sessions: cfg.Agent.Capabilities.Sessions,
		},
		Database: &dto.DatabaseConfigSnapshot{
			Driver: cfg.Database.Driver, DSN: cfg.Database.DSN,
			MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns,
		},
		Memory: &dto.MemoryConfigSnapshot{
			Backend: string(cfg.Memory.Backend), FilePath: cfg.Memory.File.Path,
			Neo4j:    &dto.Neo4jConfigSnapshot{URI: cfg.Memory.Neo4j.URI, User: cfg.Memory.Neo4j.User, Password: cfg.Memory.Neo4j.Password},
			Postgres: &dto.PostgresConfigSnapshot{DSN: cfg.Memory.Postgres.DSN},
		},
		Subagents: &dto.SubagentsConfigSnapshot{
			MaxConcurrent: cfg.SubAgents.MaxConcurrent, DefaultTimeout: cfg.SubAgents.DefaultTimeout.String(),
		},
		GraphQL:   &dto.GraphQLConfigSnapshot{Enabled: cfg.GraphQL.Enabled, Port: cfg.GraphQL.Port, Host: cfg.GraphQL.Host, BaseURL: cfg.GraphQL.BaseURL},
		Logging:   &dto.LoggingConfigSnapshot{Level: cfg.Logging.Level, Path: cfg.Logging.Path},
		Scheduler: &dto.SchedulerConfigSnapshot{Enabled: cfg.Scheduler.Enabled, MemoryEnabled: cfg.Scheduler.MemoryEnabled, MemoryInterval: cfg.Scheduler.MemoryInterval.String()},
		Secrets: &dto.SecretsConfigSnapshot{
			Backend: cfg.Secrets.Backend,
			File:    &dto.FileSecretsSnapshot{Path: cfg.Secrets.File.Path},
			Openbao: func() *dto.OpenbaoSecretsSnapshot {
				if cfg.Secrets.Openbao == nil {
					return nil
				}
				return &dto.OpenbaoSecretsSnapshot{URL: cfg.Secrets.Openbao.URL, Token: cfg.Secrets.Openbao.Token}
			}(),
		},
		ChannelSecrets: &dto.ChannelSecretsSnapshot{
			TelegramEnabled: cfg.Channels.Telegram.Enabled, TelegramToken: cfg.Channels.Telegram.BotToken,
			DiscordEnabled: cfg.Channels.Discord.Enabled, DiscordToken: cfg.Channels.Discord.BotToken,
			SlackEnabled: cfg.Channels.Slack.Enabled, SlackBotToken: cfg.Channels.Slack.BotToken, SlackAppToken: cfg.Channels.Slack.AppToken,
		},
		WizardCompleted: cfg.Wizard.Completed,
	}
}

// maxOutputTokens is the fixed limit for AI completion output (~2000 chars, fits Discord).
const maxOutputTokens = 500

// buildAIProviderFromConfig creates an AIProviderPort from config (used at startup and on soft reboot).
func buildAIProviderFromConfig(cfg *config.Config) ports.AIProviderPort {
	var p ports.AIProviderPort
	switch {
	case cfg.Providers.OpenAI.APIKey != "" && cfg.Providers.OpenAI.APIKey != "YOUR_API_KEY_HERE":
		model := cfg.Providers.OpenAI.Model
		if model == "" {
			model = "gpt-4o"
		}
		baseURL := cfg.Providers.OpenAI.BaseURL
		if baseURL != "" {
			p = aiopenai.NewAdapterWithEndpoint(baseURL, cfg.Providers.OpenAI.APIKey, model, maxOutputTokens)
		} else {
			p = aiopenai.NewAdapter(cfg.Providers.OpenAI.APIKey, model, maxOutputTokens)
		}
	case cfg.Providers.OpenRouter.APIKey != "" && cfg.Providers.OpenRouter.APIKey != "YOUR_API_KEY_HERE":
		model := cfg.Providers.OpenRouter.DefaultModel
		if model == "" {
			model = "openai/gpt-4o"
		}
		p = aiopenrouter.NewAdapter(cfg.Providers.OpenRouter.APIKey, model, maxOutputTokens)
	case cfg.Providers.OpenAICompat.APIKey != "" &&
		cfg.Providers.OpenAICompat.APIKey != "YOUR_API_KEY_HERE" &&
		cfg.Providers.OpenAICompat.BaseURL != "":
		model := cfg.Providers.OpenAICompat.Model
		if model == "" {
			model = "default"
		}
		p = aiopenaicompat.NewAdapter(
			cfg.Providers.OpenAICompat.BaseURL,
			cfg.Providers.OpenAICompat.APIKey,
			model,
			maxOutputTokens,
		)
	case cfg.Providers.Ollama.Endpoint != "":
		model := cfg.Providers.Ollama.DefaultModel
		if model == "" {
			model = "llama3"
		}
		p = aiollama.NewAdapterWithOptions(cfg.Providers.Ollama.Endpoint, cfg.Providers.Ollama.APIKey, model, maxOutputTokens, cfg.Logging.Level)
	case cfg.Providers.Anthropic.APIKey != "" && cfg.Providers.Anthropic.APIKey != "YOUR_API_KEY_HERE":
		model := cfg.Providers.Anthropic.Model
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		p = aianthropicadapter.NewAdapter(cfg.Providers.Anthropic.APIKey, model, maxOutputTokens)
	case cfg.Providers.OpenCode.APIKey != "" && cfg.Providers.OpenCode.APIKey != "YOUR_API_KEY_HERE":
		model := cfg.Providers.OpenCode.Model
		if model == "" {
			model = "kimi-k2.5"
		}
		p = aizenadapter.NewAdapter(cfg.Providers.OpenCode.APIKey, model, maxOutputTokens)
	case cfg.Providers.DockerModelRunner.Endpoint != "":
		model := cfg.Providers.DockerModelRunner.DefaultModel
		if model == "" {
			model = "ai/mistral-nemo"
		}
		p = aidockermodelrunner.NewAdapter(cfg.Providers.DockerModelRunner.Endpoint, model, maxOutputTokens)
	}
	return p
}

// providerName returns a human-readable AI provider label from the config.
func providerName(cfg *config.Config) string {
	switch {
	case cfg.Providers.OpenAI.APIKey != "" && cfg.Providers.OpenAI.APIKey != "YOUR_API_KEY_HERE":
		return "openai"
	case cfg.Providers.OpenRouter.APIKey != "" && cfg.Providers.OpenRouter.APIKey != "YOUR_API_KEY_HERE":
		return "openrouter"
	case cfg.Providers.Ollama.Endpoint != "":
		return "ollama"
	case cfg.Providers.Anthropic.APIKey != "" && cfg.Providers.Anthropic.APIKey != "YOUR_API_KEY_HERE":
		return "anthropic"
	case cfg.Providers.OpenCode.APIKey != "" && cfg.Providers.OpenCode.APIKey != "YOUR_API_KEY_HERE":
		return "opencode-zen"
	case cfg.Providers.DockerModelRunner.Endpoint != "":
		return "docker-model-runner"
	default:
		return "none"
	}
}
