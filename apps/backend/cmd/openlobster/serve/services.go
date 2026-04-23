package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/application/graphql/subscriptions"
	appcontext "github.com/neirth/openlobster/internal/domain/context"
	"github.com/neirth/openlobster/internal/domain/events"
	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	browser "github.com/neirth/openlobster/internal/infrastructure/adapters/browser/chromedp"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/filesystem"
	inframc "github.com/neirth/openlobster/internal/infrastructure/adapters/mcp"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/terminal"
	pluginadapter "github.com/neirth/openlobster/internal/infrastructure/plugin"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
	"github.com/spf13/viper"
)

type namedMessagingAdapter struct {
	channelType string
	adapter     ports.MessagingPort
}

func normalizeChannelType(channelType string) string {
	return strings.ToLower(strings.TrimSpace(channelType))
}

func pluginMessagingChannelType(pluginID string) string {
	ct := strings.TrimSpace(pluginID)
	const pfx = "openlobster-messages-"
	ct = strings.TrimPrefix(ct, pfx)
	return normalizeChannelType(ct)
}

type messagingAdapterCloser interface {
	Close() error
}

type messagingAdapterChannelType interface {
	ChannelType() string
}

func (a *App) stopMessagingRuntime() {
	const adapterCloseTimeout = 2 * time.Second

	if a.messagingRuntimeCancel != nil {
		a.messagingRuntimeCancel()
		a.messagingRuntimeCancel = nil
	}

	// Granular cleanup using individual cancels
	if a.channelCancels != nil {
		for ct := range a.channelCancels {
			a.stopMessagingChannel(ct)
		}
	}

	// Ensure all adapters are closed if they were tracked legacy-style
	for _, adapter := range a.MessagingAdapters {
		if adapter == nil {
			continue
		}

		channelType := "unknown"
		if typed, ok := adapter.(messagingAdapterChannelType); ok {
			if normalized := normalizeChannelType(typed.ChannelType()); normalized != "" {
				channelType = normalized
			}
		}

		closer, ok := adapter.(messagingAdapterCloser)
		if !ok {
			continue
		}

		done := make(chan error, 1)
		go func(c messagingAdapterCloser) {
			done <- c.Close()
		}(closer)

		select {
		case err := <-done:
			if err != nil {
				log.Printf("channel %s: failed to close adapter: %v", channelType, err)
			}
		case <-time.After(adapterCloseTimeout):
			log.Printf("channel %s: close timed out after %s", channelType, adapterCloseTimeout)
		}
	}
	a.MessagingAdapters = nil
	if a.ChanReg == nil {
		a.initChannels()
	} else {
		a.ChanReg.Clear()
	}
}

// stopMessagingChannel closes a specific messaging adapter by its channel type.
func (a *App) stopMessagingChannel(channelType string) {
	ct := normalizeChannelType(channelType)
	if a.channelCancels == nil {
		return
	}

	cancel, ok := a.channelCancels[ct]
	if !ok {
		return
	}

	log.Printf("channels: stopping adapter %q...", ct)
	cancel()
	delete(a.channelCancels, ct)

	// Also remove from the main registry to prevent further routing
	a.ChanReg.Remove(ct)

	// Clean up legacy MessagingAdapters slice if it contains this adapter
	newAdapters := make([]ports.MessagingPort, 0)
	for _, adp := range a.MessagingAdapters {
		if typed, ok := adp.(messagingAdapterChannelType); ok {
			if normalizeChannelType(typed.ChannelType()) == ct {
				if closer, ok := adp.(messagingAdapterCloser); ok {
					_ = closer.Close()
				}
				continue
			}
		}
		newAdapters = append(newAdapters, adp)
	}
	a.MessagingAdapters = newAdapters
}

func (a *App) startMessagingAdapters(ctx context.Context, adapters []namedMessagingAdapter) {
	started := make([]ports.MessagingPort, 0, len(adapters))
	var startedMu sync.Mutex
	var wg sync.WaitGroup

	appendStarted := func(adapter ports.MessagingPort) {
		startedMu.Lock()
		started = append(started, adapter)
		startedMu.Unlock()
	}

	for _, item := range adapters {
		if item.adapter == nil {
			continue
		}
		channelType := normalizeChannelType(item.channelType)
		if channelType == "" {
			continue
		}
		if modePort, ok := item.adapter.(ports.MessagingInboundModePort); ok && !modePort.RequiresBackgroundLoop() {
			appendStarted(item.adapter)
			log.Printf("channel: %s - adapter ready (inbound_mode=%s, no background start)", channelType, modePort.InboundMode())
			continue
		}

		adapter := item.adapter
		wg.Add(1)

		// Create a per-channel context anchored to the main messaging context
		cctx, ccancel := context.WithCancel(ctx)
		if a.channelCancels == nil {
			a.channelCancels = make(map[string]context.CancelFunc)
		}
		a.channelCancels[channelType] = ccancel

		go func(channelType string, adapter ports.MessagingPort, cctx context.Context) {
			defer wg.Done()

			if err := adapter.Start(cctx, func(_ context.Context, msg *models.Message) {
				a.handleInboundMessage(msg, channelType)
			}); err != nil {
				log.Printf("channel %s: failed to start adapter: %v", channelType, err)
				return
			}

			appendStarted(adapter)
			log.Printf("channel: %s - adapter started", channelType)
		}(channelType, adapter, cctx)
	}

	// We launch adapters in background but don't wait for them to finish here,
	// as they are long-running processes. 
	go func() {
		wg.Wait()
	}()
	a.MessagingAdapters = started
}

func (a *App) SyncMessagingRuntime() {
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()

	// Granular diff-based reconciliation starts here. 
	// We no longer call a.stopMessagingRuntime() globally.

	if a.PluginRegistry == nil {
		if a.AgentRegistry != nil {
			a.AgentRegistry.UpdateAgentChannels(a.rebuildActiveChannels())
		}
		return
	}

	runtimeCtx := a.ChannelStartCtx
	if runtimeCtx == nil {
		runtimeCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(runtimeCtx)
	a.messagingRuntimeCancel = cancel

	started := make([]namedMessagingAdapter, 0)
	wired := make(map[string]string)

	plugins := a.PluginRegistry.GetByType("messaging")
	sort.SliceStable(plugins, func(i, j int) bool { return plugins[i].ID() < plugins[j].ID() })

	keepAlive := make(map[string]bool)
	for _, p := range plugins {
		if p == nil {
			continue
		}
		pluginID := strings.TrimSpace(p.ID())
		channelType := pluginMessagingChannelType(pluginID)
		if channelType == "" {
			continue
		}

		enabled := isMessagingChannelEnabled(a, channelType)
		if !enabled {
			if _, running := a.channelCancels[channelType]; running {
				log.Printf("plugins: messaging channel %s was disabled, stopping it", channelType)
				a.stopMessagingChannel(channelType)
			}
			continue
		}

		// If it's already running, we only restart if we really have to.
		// For now, to ensure tokens are picked up, we'll follow a "stop and restart" 
		// approach IF it was enabled, but critically, we only do it for the ones 
		// that are part of this reconciliation pass.
		// Since SyncMessagingRuntime doesn't know which specific ones changed yet, 
		// we'll at least stop the existing instance of THIS channel before starting a new one.
		if _, running := a.channelCancels[channelType]; running {
			log.Printf("plugins: messaging channel %s is already running, replacing with fresh config", channelType)
			a.stopMessagingChannel(channelType)
			time.Sleep(300 * time.Millisecond) // Give polling loop time to exit
		}

		cfg := liveConfigForPluginFromApp(a, p)
		if err := validatePluginConfig(p, cfg); err != nil {
			log.Printf("plugins: messaging channel %s not wired: %v", channelType, err)
			continue
		}

		wrapper := pluginadapter.NewMessagingWrapper(p, channelType, cfg)
		a.ChanReg.Set(channelType, wrapper)
		wired[channelType] = pluginID
		started = append(started, namedMessagingAdapter{channelType: channelType, adapter: wrapper})
		log.Printf("plugins: messaging channel → %s (%s)", channelType, p.Name())
		keepAlive[channelType] = true
	}

	// Stop any channel that is no longer in the active plugins list (e.g. plugin removed)
	for ct := range a.channelCancels {
		if !keepAlive[ct] {
			log.Printf("channels: cleaning up orphaned adapter %q", ct)
			a.stopMessagingChannel(ct)
		}
	}

	a.startMessagingAdapters(ctx, started)

	if a.AgentRegistry != nil {
		a.AgentRegistry.UpdateAgentChannels(a.rebuildActiveChannels())
	}

	active := a.ChanReg.ListTypes()
	sort.Strings(active)
	log.Printf("channels: active adapters=%v", active)
}

func liveConfigForPluginFromApp(a *App, p ports.PluginPort) map[string]interface{} {
	if a == nil || p == nil {
		return map[string]interface{}{}
	}

	// Determine the Viper category root based on the plugin type
	pType := p.Type()
	_, shortID := parseID(p.ID())

	viperRoot := ""
	switch pType {
	case "ai":
		viperRoot = "providers"
	case "messaging":
		viperRoot = "channels"
	case "memory", "secrets", "audio":
		viperRoot = pType
	}

	clone := make(map[string]interface{})
	if viperRoot != "" {
		key := viperRoot + "." + shortID
		clone = viper.GetStringMap(key)
		token := viper.GetString(key + ".bot_token")
		if token != "" && clone["bot_token"] == nil {
			clone["bot_token"] = token
		}

		// Mask sensitive fields for debugging
		readyMap := make(map[string]interface{})
		for k, v := range clone {
			val := fmt.Sprintf("%v", v)
			lowK := strings.ToLower(k)
			if strings.Contains(lowK, "key") || strings.Contains(lowK, "token") || strings.Contains(lowK, "secret") {
				if len(val) > 8 {
					readyMap[k] = val[:4] + "..." + val[len(val)-4:]
				} else {
					readyMap[k] = "****"
				}
			} else {
				readyMap[k] = v
			}
		}
		logging.Debugf("DEBUG: CONFIG: Initializing plugin %q with configuration: %v", p.ID(), readyMap)
	}

	// 2. Generic absolute path resolution for any "path" key
	if pathVal, ok := clone["path"]; ok {
		if pathStr, ok := pathVal.(string); ok && pathStr != "" && !filepath.IsAbs(pathStr) {
			clone["path"] = filepath.Join(a.Cfg.BaseDir, pathStr)
		}
	}

	// 3. Category-wide mandatory injections
	switch pType {
	case "memory":
		clone["backend"] = string(a.Cfg.Memory.Backend)
		// Special case: "file" and "gml" share the same struct in a.Cfg for simplicity
		if (shortID == "file" || shortID == "gml") && (clone["path"] == nil || clone["path"] == "") {
			path := a.Cfg.Memory.File.Path
			if path != "" && !filepath.IsAbs(path) {
				path = filepath.Join(a.Cfg.BaseDir, path)
			}
			clone["path"] = path
		}
	case "secrets":
		clone["backend"] = a.Cfg.Secrets.Backend
		if (shortID == "file" || shortID == "json") && (clone["path"] == nil || clone["path"] == "") {
			path := a.Cfg.Secrets.File.Path
			if path != "" && !filepath.IsAbs(path) {
				path = filepath.Join(a.Cfg.BaseDir, path)
			}
			clone["path"] = path
		}
	case "ai":
		// Inject provider model as default if not explicitly set in the plugin config
		if clone["model"] == nil || clone["model"] == "" {
			switch shortID {
			case "ollama":
				clone["model"] = a.Cfg.Providers.Ollama.DefaultModel
			case "openai":
				clone["model"] = a.Cfg.Providers.OpenAI.Model
			case "anthropic":
				clone["model"] = a.Cfg.Providers.Anthropic.Model
			}
		}
	}

	return clone
}

// initPlugins loads all native plugins from cfg.Plugins.Dir, registers them in
// PluginRegistry, and wires AI/memory/messaging adapters derived from plugins
// into the application.  Must be called before initServices().
func (a *App) initPlugins() {
	a.PluginRegistry = pluginadapter.NewRegistry()

	// Initialise ChanReg/MsgRouter here so messaging plugins can register.
	a.initChannels()

	ctx := context.Background()
	plugins, err := pluginadapter.LoadPlugins(
		ctx,
		a.Cfg.Plugins.Dir,
		a.onPluginMessage,
		a.Cfg.Plugins.Builtins,
		a.Cfg.Plugins.CallTimeout,
	)
	if err != nil {
		log.Printf("plugins: failed to load: %v", err)
		return
	}

	// Register and find enabled plugins
	enabledPlugins := make([]ports.PluginPort, 0)
	for _, p := range plugins {
		a.PluginRegistry.Register(p)
		if p.Type() == "messaging" || isPluginEnabled(a.Cfg.Plugins.Enabled, p.ID()) {
			enabledPlugins = append(enabledPlugins, p)
			continue
		}
		log.Printf("plugins: %s disabled by config", p.ID())
	}

	// Select canonical defaults for each type
	selectedPluginByType := map[string]string{
		"ai": selectProviderPluginID(
			enabledPlugins,
			"ai",
			configuredBackendForType("ai", a),
			a.Cfg.Agent.Provider,
			nil,
		),
		"memory": selectProviderPluginID(
			enabledPlugins,
			"memory",
			configuredBackendForType("memory", a),
			string(a.Cfg.Memory.Backend),
			nil,
		),
		"secrets": selectProviderPluginID(
			enabledPlugins,
			"secrets",
			configuredBackendForType("secrets", a),
			a.Cfg.Secrets.Backend,
			nil,
		),
		"audio": selectProviderPluginID(
			enabledPlugins,
			"audio",
			"",
			a.Cfg.Audio.Backend,
			nil,
		),
	}

	activePlugins := make([]ports.PluginPort, 0, len(enabledPlugins))
	for _, p := range enabledPlugins {
		selectedPluginID, singleDefaultType := selectedPluginByType[p.Type()]
		if !singleDefaultType {
			activePlugins = append(activePlugins, p)
			continue
		}

		if selectedPluginID == "" {
			log.Printf("plugins: %s skipped (no default selected for %s)", p.ID(), p.Type())
			continue
		}

		if p.ID() != selectedPluginID {
			log.Printf("plugins: %s skipped (not default for %s). Expected: %s, Got: %s", p.ID(), p.Type(), selectedPluginID, p.ID())
			continue
		}

		activePlugins = append(activePlugins, p)
	}

	// Pass 1: audio plugins (must be wired before AI so the handler can use
	// the audio provider as fallback when the AI model lacks native audio).
	audioProviders := make([]namedAudioProvider, 0)
	for _, p := range activePlugins {
		if p.Type() != "audio" {
			continue
		}
		cfg := liveConfigForPluginFromApp(a, p)
		if err := validatePluginConfig(p, cfg); err != nil {
			continue
		}
		audioProviders = append(audioProviders, namedAudioProvider{
			id:       p.ID(),
			name:     p.Name(),
			provider: pluginadapter.NewAudioWrapper(p, cfg),
		})
	}
	if len(audioProviders) > 0 {
		orderedAudioProviders := orderAudioProviders(audioProviders, selectedPluginByType["audio"])
		a.AudioProvider = newFallbackAudioProvider(orderedAudioProviders)
		if len(orderedAudioProviders) > 1 {
			log.Printf("plugins: audio provider → %s (fallbacks: %d)", orderedAudioProviders[0].name, len(orderedAudioProviders)-1)
		} else {
			log.Printf("plugins: audio provider → %s", orderedAudioProviders[0].name)
		}
	}

	// Pass 2: AI, messaging, memory plugins.
	for _, p := range activePlugins {
		cfg := liveConfigForPluginFromApp(a, p)
		switch p.Type() {
		case "ai":
			if selectedPluginByType["ai"] != "" && p.ID() != selectedPluginByType["ai"] {
				continue
			}
			if err := validatePluginConfig(p, cfg); err != nil {
				continue
			}
			if a.AIProvider == nil {
				a.AIProvider = pluginadapter.NewAIWrapper(p, cfg)
				if m, ok := cfg["model"].(string); ok {
					a.AIModel = m
				}
				log.Printf("plugins: AI provider -> %s (model: %s)", p.Name(), a.AIModel)
			}

		case "messaging":
			// Messaging channels are handled by SyncMessagingRuntime during startAndWait
			continue

		case "memory":
			if selectedPluginByType["memory"] != "" && p.ID() != selectedPluginByType["memory"] {
				continue
			}
			if err := validatePluginConfig(p, cfg); err != nil {
				log.Printf("plugins: memory backend %s invalid config: %v", p.ID(), err)
				continue
			}
			if a.MemoryAdapter == nil {
				a.MemoryAdapter = pluginadapter.NewMemoryWrapper(p, cfg)
				log.Printf("plugins: memory backend → %s", p.Name())
			}
		case "secrets":
			if selectedPluginByType["secrets"] != "" && p.ID() != selectedPluginByType["secrets"] {
				continue
			}
			if err := validatePluginConfig(p, cfg); err != nil {
				log.Printf("plugins: secrets backend %s invalid config: %v", p.ID(), err)
				continue
			}
			if a.SecretsProvider == nil {
				a.SecretsProvider = pluginadapter.NewSecretsWrapper(p, cfg)
				log.Printf("plugins: secrets backend → %s", p.Name())
			}
		}
	}

	if a.AudioProvider == nil {
		log.Println("info: no audio plugin loaded — TTS/STT fallback disabled")
	}
}

func validatePluginConfig(p ports.PluginPort, cfg map[string]interface{}) error {
	schema, err := p.Schema()
	if err != nil {
		return nil
	}
	return pluginadapter.ValidateConfigSchema(schema, cfg)
}

func parseID(pluginID string) (pType, pName string) {
	id := strings.ToLower(strings.TrimSpace(pluginID))
	if strings.Contains(id, ":") {
		parts := strings.SplitN(id, ":", 2)
		return parts[0], parts[1]
	}

	prefixes := []string{"openlobster-messages-", "openlobster-ai-", "openlobster-memory-", "openlobster-secrets-", "openlobster-audio-"}
	for _, pfx := range prefixes {
		if strings.HasPrefix(id, pfx) {
			pName = strings.TrimPrefix(id, pfx)
			pType = strings.TrimPrefix(strings.TrimSuffix(pfx, "-"), "openlobster-")
			if pType == "messages" {
				pType = "messaging"
			}
			return pType, pName
		}
	}
	return "", id
}

func isMessagingChannelEnabled(a *App, channelType string) bool {
	enabledByID := a.Cfg.Plugins.Enabled
	if v, ok := enabledByID[channelType]; ok {
		return v
	}

	// Dynamic lookup via config
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "telegram":
		return a.Cfg.Channels.Telegram.Enabled
	case "discord":
		return a.Cfg.Channels.Discord.Enabled
	case "slack":
		return a.Cfg.Channels.Slack.Enabled
	case "whatsapp":
		return a.Cfg.Channels.WhatsApp.Enabled
	case "twilio":
		return a.Cfg.Channels.Twilio.Enabled
	default:
		return false
	}
}



func isPluginEnabled(enabled map[string]bool, pluginID string) bool {
	if enabled == nil {
		return true
	}
	if v, ok := enabled[pluginID]; ok {
		return v
	}
	_, shortID := parseID(pluginID)
	if v, ok := enabled[shortID]; ok {
		return v
	}
	return true
}

func configuredBackendForType(pluginType string, a *App) string {
	switch pluginType {
	case "ai":
		return strings.ToLower(strings.TrimSpace(a.Cfg.Agent.Provider))
	case "memory":
		return strings.ToLower(strings.TrimSpace(string(a.Cfg.Memory.Backend)))
	case "secrets":
		return strings.ToLower(strings.TrimSpace(a.Cfg.Secrets.Backend))
	default:
		return ""
	}
}

func selectProviderPluginID(plugins []ports.PluginPort, pluginType, backend, preferredPluginID string, pluginSettings map[string]map[string]interface{}) string {
	// Pass 1: Mandatory Default Check (Viper/Explicit setting wins)
	if preferredPluginID != "" {
		for _, p := range plugins {
			if p.Type() != pluginType {
				continue
			}
			pID := p.ID()
			_, pShortID := parseID(pID)

			// Match by full ID (memory:file), short ID (file), or friendly name
			if pID == preferredPluginID || pShortID == preferredPluginID || p.Name() == preferredPluginID {
				return p.ID()
			}
		}
	}

	// Pass 2: Semantic Defaults (Internal metadata priority)
	semanticDefault := ""
	switch pluginType {
	case "memory", "secrets":
		semanticDefault = pluginType + ":file"
	case "ai":
		semanticDefault = "ai:ollama"
	}

	if semanticDefault != "" {
		for _, p := range plugins {
			if p.Type() == pluginType && p.ID() == semanticDefault {
				return p.ID()
			}
		}
	}

	// Pass 3: Scoring / Fallback
	bestID := ""
	bestScore := -1001

	for _, p := range plugins {
		if p.Type() != pluginType {
			continue
		}

		// Initial fallback: take the first one seen of this type
		if bestID == "" {
			bestID = p.ID()
			bestScore = -1000
		}

		score := 0
		cfg := pluginSettings[p.ID()]
		schema, err := p.Schema()
		if err == nil {
			score = scoreConfigAgainstSchema(schema, cfg, backend)
		}

		if score > bestScore {
			bestScore = score
			bestID = p.ID()
		}
	}

	return bestID
}

func scoreConfigAgainstSchema(schema []byte, cfg map[string]interface{}, backend string) int {
	schemaDoc := parseSchema(schema)
	if len(schemaDoc.Properties) == 0 {
		return 0
	}

	score := 0
	for key, prop := range schemaDoc.Properties {
		if !isEmptyValue(cfg[key]) && (isEmptyValue(prop.Default) || !reflect.DeepEqual(cfg[key], prop.Default)) {
			score++
		}
	}
	for _, key := range schemaDoc.Required {
		if !isEmptyValue(cfg[key]) {
			score += 8
		} else {
			score -= 16
		}
	}
	if backend != "" {
		if cfgBackend, ok := cfg["backend"].(string); ok && strings.EqualFold(strings.TrimSpace(cfgBackend), backend) {
			score += 2
		}
	}
	return score
}

type schemaDoc struct {
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
}

type schemaProperty struct {
	Default interface{} `json:"default"`
}

func parseSchema(schema []byte) schemaDoc {
	var doc schemaDoc
	if len(schema) == 0 {
		return doc
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return schemaDoc{}
	}
	if doc.Properties == nil {
		doc.Properties = map[string]schemaProperty{}
	}
	return doc
}


func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case map[string]interface{}:
		return len(x) == 0
	case []interface{}:
		return len(x) == 0
	}
	rv := reflect.ValueOf(v)
	return !rv.IsValid() || rv.IsZero()
}

type namedAudioProvider struct {
	id       string
	name     string
	provider ports.AudioProviderPort
}

type fallbackAudioProvider struct {
	providers []namedAudioProvider
}

func newFallbackAudioProvider(providers []namedAudioProvider) ports.AudioProviderPort {
	if len(providers) == 0 {
		return nil
	}
	if len(providers) == 1 {
		return providers[0].provider
	}
	return &fallbackAudioProvider{providers: providers}
}

func orderAudioProviders(providers []namedAudioProvider, preferredID string) []namedAudioProvider {
	if len(providers) <= 1 || strings.TrimSpace(preferredID) == "" {
		return providers
	}
	ordered := make([]namedAudioProvider, 0, len(providers))
	for _, provider := range providers {
		if provider.id == preferredID {
			ordered = append(ordered, provider)
			break
		}
	}
	for _, provider := range providers {
		if provider.id == preferredID {
			continue
		}
		ordered = append(ordered, provider)
	}
	if len(ordered) == 0 {
		return providers
	}
	return ordered
}

func (p *fallbackAudioProvider) TextToSpeech(ctx context.Context, req ports.TTSRequest) (ports.TTSResponse, error) {
	var errs []string
	for _, candidate := range p.providers {
		resp, err := candidate.provider.TextToSpeech(ctx, req)
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", candidate.name, err))
	}
	return ports.TTSResponse{}, fmt.Errorf("all audio providers failed tts: %s", strings.Join(errs, "; "))
}

func (p *fallbackAudioProvider) SpeechToText(ctx context.Context, req ports.STTRequest) (ports.STTResponse, error) {
	var errs []string
	for _, candidate := range p.providers {
		resp, err := candidate.provider.SpeechToText(ctx, req)
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", candidate.name, err))
	}
	return ports.STTResponse{}, fmt.Errorf("all audio providers failed stt: %s", strings.Join(errs, "; "))
}

// onPluginMessage is the callback invoked by messaging plugins (via
// host_emit_message) to deliver inbound messages to the message handler.
func (a *App) onPluginMessage(pluginID string, channelType string, msgJSON []byte) {
	var msg models.Message
	if err := json.Unmarshal(msgJSON, &msg); err != nil {
		log.Printf("plugins: malformed inbound message from plugin %s: %v", pluginID, err)
		return
	}
	a.handleInboundMessage(&msg, channelType)
}

func (a *App) handleInboundMessage(msg *models.Message, fallbackChannelType string) {
	if a.MsgHandler == nil || msg == nil {
		return
	}

	fallbackCT := strings.ToLower(strings.TrimSpace(fallbackChannelType))
	if fallbackCT == "unknown" {
		fallbackCT = ""
	}
	metadataCT := ""
	if msg.Metadata != nil {
		if v, ok := msg.Metadata["channel_type"].(string); ok {
			metadataCT = strings.ToLower(strings.TrimSpace(v))
		}
	}

	ct := metadataCT
	if fallbackCT != "" {
		if metadataCT != "" && metadataCT != fallbackCT {
			log.Printf("plugins: inbound channel_type mismatch metadata=%q fallback=%q (using fallback)", metadataCT, fallbackCT)
		}
		ct = fallbackCT
	}

	if msg.Metadata == nil {
		msg.Metadata = make(map[string]interface{})
	}
	if ct != "" {
		msg.Metadata["channel_type"] = ct
	}
	if ct != "" && a.ChanReg != nil && a.ChanReg.Get(ct) == nil {
		active := a.ChanReg.ListTypes()
		sort.Strings(active)
		log.Printf("plugins: inbound %s dropped: no active adapter in runtime (active_adapters=%v)", ct, active)
		return
	}

	if msg.Content == "" && len(msg.Attachments) == 0 && msg.Audio == nil {
		if !strings.EqualFold(strings.TrimSpace(ct), "discord") {
			return
		}
		log.Printf("plugins: inbound discord event with empty content; forwarding for pairing/routing (sender=%q channel=%q)", msg.SenderID, msg.ChannelID)
	}
	if strings.EqualFold(strings.TrimSpace(ct), "discord") {
		log.Printf("plugins: inbound discord event sender=%q channel=%q len=%d group=%v mentioned=%v", msg.SenderID, msg.ChannelID, len(msg.Content), msg.IsGroup, msg.IsMentioned)
	}
	if err := a.MsgHandler.Handle(context.Background(), domainhandlers.HandleMessageInput{
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
	}); err != nil {
		log.Printf("plugins: message handler error: %v", err)
	}
}

// initServices initialises the AI provider, memory backend, event bus,
// tool registry, message handler and all supporting domain services.
func (a *App) initServices() {
	cfg := a.Cfg

	// AI provider and memory backend are already set by initPlugins() if
	// plugin-provided. Log current state.
	if a.AIProvider != nil {
		log.Println("ai provider: plugin")
	}

	// Event bus + subscription manager
	eventBus := domainservices.NewEventBus()
	a.EventBus = eventBus
	a.SubManager = subscriptions.NewSubscriptionManager(eventBus)

	broadcastToSubs := func(ctx context.Context, e events.Event) error {
		a.SubManager.Broadcast(e)
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

	// Pairing service
	a.PairingService = domainservices.NewPairingService(a.PairingRepo)

	// Permission manager (loaded from config + DB below)
	a.PermManager = permissions.Default()
	a.loadPermissions(a.PermManager)

	// Tool registry
	a.ToolRegistry = mcp.NewToolRegistry(true, a.PermManager)

	// Skills adapter
	a.SkillsAdapter = filesystem.NewSkillsAdapter(cfg.Workspace.Path)
	log.Printf("skills: reading from %s/skills", cfg.Workspace.Path)

	// Sub-agent & compaction services
	a.SubAgentSvc = domainservices.NewSubAgentService(
		a.AIProvider,
		cfg.SubAgents.MaxConcurrent,
		cfg.SubAgents.DefaultTimeout,
	)
	a.CompactionSvc = domainservices.NewMessageCompactionService(a.MessageRepo, a.AIProvider)

	// Register all internal tools
	mcp.RegisterAllInternalTools(a.ToolRegistry, mcp.InternalTools{
		Messaging:           &inframc.MessagingAdapter{Port: a.MsgRouter},
		MessageLog:          &inframc.OutboundMessageLogAdapter{MessageRepo: a.MessageRepo, SessionRepo: a.SessionRepo, UserChannelRepo: a.UserChannelRepo},
		LastChannelResolver: a.UserChannelRepo,
		Memory:              &inframc.MemoryAdapter{Port: a.MemoryAdapter},
		Tasks: &inframc.TaskAdapter{Repo: a.TaskRepo, Notify: func() {
			if a.SchedulerNotify != nil {
				a.SchedulerNotify()
			}
		}},
		SubAgents: a.SubAgentSvc,
		Terminal:  terminal.NewHostAdapter(),
		Browser: &inframc.BrowserAdapter{
			Port: browser.NewChromeDPAdapter(browser.ChromeDPConfig{Headless: true}),
		},
		Cron: &inframc.CronAdapter{Repo: a.TaskRepo, Notify: func() {
			if a.SchedulerNotify != nil {
				a.SchedulerNotify()
			}
		}},
		Filesystem:    filesystem.NewAdapter(a.CfgPath),
		Conversations: &inframc.ConversationAdapter{ConvRepo: a.ConvRepo, MsgRepo: a.MessageRepo},
		Skills:        a.SkillsAdapter,
		ConfigPath:    a.CfgPath,
		SchedulerNotify: func() {
			if a.SchedulerNotify != nil {
				a.SchedulerNotify()
			}
		},
	})
	log.Printf("tools: registered %d internal tools", len(a.ToolRegistry.AllTools()))

	// Wire tool registry into subagents so they can perform tool_use loops.
	a.SubAgentSvc.SetToolRegistry(a.ToolRegistry)
	a.SubAgentSvc.SetPermissionManager(a.PermManager)

	// Context injector
	a.CtxInjector = appcontext.NewContextInjector(
		cfg.Agent.Name,
		filepath.Join(cfg.Workspace.Path, "AGENTS.md"),
		filepath.Join(cfg.Workspace.Path, "SOUL.md"),
		filepath.Join(cfg.Workspace.Path, "IDENTITY.md"),
		filepath.Join(cfg.Workspace.Path, "BOOTSTRAP.md"),
		filepath.Join(cfg.Workspace.Path, "MEMORY.md"),
		a.MemoryAdapter,
		a.ToolRegistry,
	)

	// Message handler
	gormDB := a.db.GormDB()
	a.MsgHandler = domainhandlers.NewMessageHandler(a.AIProvider, a.MsgRouter, a.MemoryAdapter, a.ToolRegistry, a.PermManager, a.SessionRepo, a.MessageRepo, a.UserRepo, a.EventBus, a.CtxInjector, a.CompactionSvc, a.UserChannelRepo, a.PairingService, a.Cfg.SubAgents.MaxConcurrent, a.AudioProvider)
	// Initialize runner model with the one resolved during initPlugins
	if a.AIProvider != nil {
		a.MsgHandler.SetAIProvider(a.AIProvider, a.AIModel)
	}
	a.MsgHandler.SetGroupRegistrar(repositories.NewGroupRepository(gormDB))
	a.MsgHandler.SetPlatformEnsurer(repositories.NewChannelRepository(gormDB))
	a.MsgHandler.SetSkillsProvider(a.SkillsAdapter)
	a.MsgHandler.SetPermissionLoader(func(ctx context.Context, userID string) map[string]string {
		records, err := a.ToolPermRepo.ListByUser(ctx, userID)
		if err != nil {
			return nil
		}
		m := make(map[string]string, len(records))
		for _, r := range records {
			m[r.ToolName] = r.Mode
		}
		return m
	})
}
