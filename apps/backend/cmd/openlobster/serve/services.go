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
		go func(channelType string, adapter ports.MessagingPort) {
			defer wg.Done()

			if err := adapter.Start(ctx, func(_ context.Context, msg *models.Message) {
				a.handleInboundMessage(msg, channelType)
			}); err != nil {
				log.Printf("channel %s: failed to start adapter: %v", channelType, err)
				return
			}

			appendStarted(adapter)
			log.Printf("channel: %s - adapter started", channelType)
		}(channelType, adapter)
	}

	wg.Wait()
	a.MessagingAdapters = started
}

func (a *App) rebuildMessagingRuntime() {
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()

	a.stopMessagingRuntime()

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

	for _, p := range plugins {
		if p == nil {
			continue
		}
		pluginID := strings.TrimSpace(p.ID())
		channelType := pluginMessagingChannelType(pluginID)
		if channelType == "" {
			log.Printf("plugins: messaging plugin %q skipped (empty channel_type)", pluginID)
			continue
		}
		if !isMessagingChannelEnabled(a, channelType) {
			log.Printf("plugins: messaging channel %s disabled (plugin=%s)", channelType, pluginID)
			continue
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
	if a.Cfg == nil || a.Cfg.Plugins.Settings == nil {
		return map[string]interface{}{}
	}
	cfg := a.Cfg.Plugins.Settings[p.ID()]
	if cfg == nil {
		return map[string]interface{}{}
	}
	clone := make(map[string]interface{}, len(cfg))
	for k, v := range cfg {
		clone[k] = v
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

	// Register valid plugins first, then wire in two passes so that the audio
	// provider is available when the AI wrapper is created.
	if a.Cfg.Plugins.Settings == nil {
		a.Cfg.Plugins.Settings = make(map[string]map[string]interface{})
	}
	loadedPlugins := make([]ports.PluginPort, 0, len(plugins))
	for _, p := range plugins {
		schema, schemaErr := p.Schema()
		pluginCfg := buildPluginConfig(p.Type(), p.ID(), schema, a.Cfg.Plugins.Settings[p.ID()], a)
		a.Cfg.Plugins.Settings[p.ID()] = pluginCfg

		if schemaErr != nil {
			log.Printf("plugins: %s schema read failed: %v", p.ID(), schemaErr)
		}

		a.PluginRegistry.Register(p)
		loadedPlugins = append(loadedPlugins, p)
	}

	enabledPlugins := make([]ports.PluginPort, 0, len(loadedPlugins))
	for _, p := range loadedPlugins {
		if p.Type() == "messaging" || isPluginEnabled(a.Cfg.Plugins.Enabled, p.ID()) {
			enabledPlugins = append(enabledPlugins, p)
			continue
		}
		log.Printf("plugins: %s disabled by config", p.ID())
	}

	desiredAIPluginID := configuredDefaultPluginID("ai", a)
	if desiredAIPluginID == "" {
		desiredAIPluginID = aiPluginIDForProvider(a.Cfg.Agent.Provider)
	}
	if desiredAIPluginID == "" {
		desiredAIPluginID = selectProviderPluginID(
			enabledPlugins,
			"ai",
			configuredBackendForType("ai", a),
			"",
			a.Cfg.Plugins.Settings,
		)
	}

	preferredMemoryPluginID := selectProviderPluginID(
		enabledPlugins,
		"memory",
		configuredBackendForType("memory", a),
		configuredDefaultPluginID("memory", a),
		a.Cfg.Plugins.Settings,
	)
	preferredSecretsPluginID := selectProviderPluginID(
		enabledPlugins,
		"secrets",
		configuredBackendForType("secrets", a),
		configuredDefaultPluginID("secrets", a),
		a.Cfg.Plugins.Settings,
	)
	preferredAudioPluginID := selectProviderPluginID(
		enabledPlugins,
		"audio",
		"",
		configuredDefaultPluginID("audio", a),
		a.Cfg.Plugins.Settings,
	)

	selectedPluginByType := map[string]string{
		"ai":      desiredAIPluginID,
		"memory":  preferredMemoryPluginID,
		"secrets": preferredSecretsPluginID,
		"audio":   preferredAudioPluginID,
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
			log.Printf("plugins: %s skipped (not default for %s)", p.ID(), p.Type())
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
		cfg := a.Cfg.Plugins.Settings[p.ID()]
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
		orderedAudioProviders := orderAudioProviders(audioProviders, preferredAudioPluginID)
		a.AudioProvider = newFallbackAudioProvider(orderedAudioProviders)
		if len(orderedAudioProviders) > 1 {
			log.Printf("plugins: audio provider → %s (fallbacks: %d)", orderedAudioProviders[0].name, len(orderedAudioProviders)-1)
		} else {
			log.Printf("plugins: audio provider → %s", orderedAudioProviders[0].name)
		}
	}

	// Pass 2: AI, messaging, memory plugins.
	for _, p := range activePlugins {
		cfg := a.Cfg.Plugins.Settings[p.ID()]
		switch p.Type() {
		case "ai":
			if desiredAIPluginID != "" && p.ID() != desiredAIPluginID {
				continue
			}
			if err := validatePluginConfig(p, cfg); err != nil {
				if desiredAIPluginID == p.ID() {
					log.Printf("plugins: AI provider %s invalid config: %v", p.ID(), err)
				}
				continue
			}
			if a.AIProvider == nil {
				a.AIProvider = pluginadapter.NewAIWrapper(p, cfg)
				log.Printf("plugins: AI provider → %s", p.Name())
			}

		case "messaging":
			channelType := p.ID()
			// Strip the "openlobster-messages-" prefix if present.
			const pfx = "openlobster-messages-"
			if len(channelType) > len(pfx) && channelType[:len(pfx)] == pfx {
				channelType = channelType[len(pfx):]
			}
			if !isMessagingChannelEnabled(a, channelType) {
				continue
			}
			if err := validatePluginConfig(p, cfg); err != nil {
				log.Printf("plugins: messaging channel %s not wired: %v", channelType, err)
				continue
			}
			wrapper := pluginadapter.NewMessagingWrapper(p, channelType, cfg)
			a.ChanReg.Set(channelType, wrapper)
			a.MessagingAdapters = append(a.MessagingAdapters, wrapper)
			log.Printf("plugins: messaging channel → %s (%s)", channelType, p.Name())

		case "memory":
			if preferredMemoryPluginID != "" && p.ID() != preferredMemoryPluginID {
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
			if preferredSecretsPluginID != "" && p.ID() != preferredSecretsPluginID {
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

func aiPluginIDForProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "ollama":
		return "openlobster-ai-ollama"
	case "anthropic":
		return "openlobster-ai-anthropic"
	case "openai":
		return "openlobster-ai-openai"
	default:
		return ""
	}
}

func isMessagingChannelEnabled(a *App, channelType string) bool {
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


func buildPluginConfig(pluginType, pluginID string, schema []byte, rawCfg map[string]interface{}, a *App) map[string]interface{} {
	cfg := cloneMap(rawCfg)
	source := domainConfigSource(pluginType, pluginID, a)
	backend := configuredBackend(source, cfg)
	if backend == "" {
		backend = configuredBackendForType(pluginType, a)
	}

	if backend != "" {
		setDefaultValue(cfg, "backend", backend)
	}

	schemaDoc := parseSchema(schema)
	candidates := configCandidates(source, backend)

	for key, prop := range schemaDoc.Properties {
		if !isEmptyValue(cfg[key]) {
			continue
		}
		if value, ok := lookupConfigValue(key, candidates...); ok {
			cfg[key] = value
			continue
		}
		if !isEmptyValue(prop.Default) {
			cfg[key] = prop.Default
		}
	}

	return cfg
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func setDefaultValue(cfg map[string]interface{}, key string, value interface{}) {
	if isEmptyValue(value) {
		return
	}
	if isEmptyValue(cfg[key]) {
		cfg[key] = value
	}
}

func isPluginEnabled(enabled map[string]bool, pluginID string) bool {
	if enabled == nil {
		return true
	}
	v, ok := enabled[pluginID]
	if !ok {
		return true
	}
	return v
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

func configuredDefaultPluginID(pluginType string, a *App) string {
	if a == nil || a.Cfg.Plugins.Defaults == nil {
		return ""
	}
	return strings.TrimSpace(a.Cfg.Plugins.Defaults[strings.ToLower(strings.TrimSpace(pluginType))])
}

func selectProviderPluginID(plugins []ports.PluginPort, pluginType, backend, preferredPluginID string, pluginSettings map[string]map[string]interface{}) string {
	bestID := ""
	bestScore := -1001 // Threshold for mandatory fallback

	// Pass 1: Mandatory Default Check
	if preferredPluginID != "" {
		for _, p := range plugins {
			if p.Type() == pluginType && (p.ID() == preferredPluginID || p.Name() == preferredPluginID) {
				return p.ID()
			}
		}
	}

	// Pass 2: Scoring / Fallback
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

func domainConfigSource(pluginType, pluginID string, a *App) map[string]interface{} {
	switch pluginType {
	case "ai":
		switch pluginID {
		case "openlobster-ai-anthropic":
			return structToConfigMap(a.Cfg.Providers.Anthropic)
		case "openlobster-ai-openai":
			return structToConfigMap(a.Cfg.Providers.OpenAI)
		case "openlobster-ai-ollama":
			src := structToConfigMap(a.Cfg.Providers.Ollama)
			if v, ok := src["endpoint"]; ok && !isEmptyValue(v) {
				src["base_url"] = v
			}
			return src
		}

	case "messaging":
		switch pluginID {
		case "openlobster-messages-telegram":
			src := structToConfigMap(a.Cfg.Channels.Telegram)
			if v, ok := src["bot_token"]; ok && !isEmptyValue(v) {
				src["token"] = v
			}
			return src
		case "openlobster-messages-discord":
			src := structToConfigMap(a.Cfg.Channels.Discord)
			if v, ok := src["bot_token"]; ok && !isEmptyValue(v) {
				src["token"] = v
			}
			return src
		case "openlobster-messages-slack":
			return structToConfigMap(a.Cfg.Channels.Slack)
		case "openlobster-messages-whatsapp":
			src := structToConfigMap(a.Cfg.Channels.WhatsApp)
			if v, ok := src["api_token"]; ok && !isEmptyValue(v) {
				src["api_access_token"] = v
			}
			if v, ok := src["phone_id"]; ok && !isEmptyValue(v) {
				src["phone_number_id"] = v
			}
			return src
		case "openlobster-messages-twilio":
			return structToConfigMap(a.Cfg.Channels.Twilio)
		}

	case "memory":
		switch pluginID {
		case "openlobster-memory-gml":
			return structToConfigMap(a.Cfg.Memory.File)
		case "openlobster-memory-neo4j":
			src := structToConfigMap(a.Cfg.Memory.Neo4j)
			if v, ok := src["user"]; ok && !isEmptyValue(v) {
				src["username"] = v
			}
			return src
		}

	case "secrets":
		switch pluginID {
		case "openlobster-secrets-json":
			return structToConfigMap(a.Cfg.Secrets.File)
		case "openlobster-secrets-openbao":
			if a.Cfg.Secrets.Openbao != nil {
				return structToConfigMap(*a.Cfg.Secrets.Openbao)
			}
		}
	}
	return map[string]interface{}{}
}

func configuredBackend(source map[string]interface{}, cfg map[string]interface{}) string {
	if v, ok := cfg["backend"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := source["backend"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return ""
}

func configCandidates(source map[string]interface{}, backend string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, 2)
	if backend != "" {
		if m, ok := source[backend].(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	out = append(out, source)
	return out
}

func lookupConfigValue(key string, candidates ...map[string]interface{}) (interface{}, bool) {
	for _, m := range candidates {
		if m == nil {
			continue
		}
		if v, ok := m[key]; ok && !isEmptyValue(v) {
			return v, true
		}
		for mk, mv := range m {
			if strings.EqualFold(mk, key) && !isEmptyValue(mv) {
				return mv, true
			}
		}
	}

	aliases := map[string][]string{
		"username":         {"user"},
		"user":             {"username"},
		"token":            {"bot_token", "api_token", "auth_token", "api_access_token"},
		"bot_token":        {"token"},
		"api_access_token": {"api_token"},
		"api_token":        {"api_access_token"},
		"phone_number_id":  {"phone_id"},
		"phone_id":         {"phone_number_id"},
		"base_url":         {"endpoint"},
		"endpoint":         {"base_url"},
	}
	for _, alias := range aliases[strings.ToLower(key)] {
		for _, m := range candidates {
			if m == nil {
				continue
			}
			if v, ok := m[alias]; ok && !isEmptyValue(v) {
				return v, true
			}
			for mk, mv := range m {
				if strings.EqualFold(mk, alias) && !isEmptyValue(mv) {
					return mv, true
				}
			}
		}
	}

	return nil, false
}

func structToConfigMap(v interface{}) map[string]interface{} {
	out, ok := toConfigValue(reflect.ValueOf(v)).(map[string]interface{})
	if !ok || out == nil {
		return map[string]interface{}{}
	}
	return out
}

func toConfigValue(v reflect.Value) interface{} {
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		m := map[string]interface{}{}
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			key := f.Tag.Get("mapstructure")
			if key == "" {
				key = strings.ToLower(f.Name)
			} else {
				key = strings.Split(key, ",")[0]
			}
			if key == "" || key == "-" {
				continue
			}
			val := toConfigValue(v.Field(i))
			if val == nil {
				continue
			}
			m[key] = val
		}
		return m

	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		m := map[string]interface{}{}
		for _, k := range v.MapKeys() {
			m[fmt.Sprintf("%v", k.Interface())] = toConfigValue(v.MapIndex(k))
		}
		return m

	case reflect.Slice, reflect.Array:
		n := v.Len()
		out := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, toConfigValue(v.Index(i)))
		}
		return out

	default:
		return v.Interface()
	}
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
func (a *App) onPluginMessage(msgJSON []byte) {
	var msg models.Message
	if err := json.Unmarshal(msgJSON, &msg); err != nil {
		log.Printf("plugins: malformed inbound message from plugin: %v", err)
		return
	}
	a.handleInboundMessage(&msg, "")
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
	a.MsgHandler = domainhandlers.NewMessageHandler(
		a.AIProvider,
		a.MsgRouter,
		a.MemoryAdapter,
		a.ToolRegistry,
		a.PermManager,
		a.SessionRepo,
		a.MessageRepo,
		a.UserRepo,
		eventBus,
		a.CtxInjector,
		a.CompactionSvc,
		a.UserChannelRepo,
		a.PairingService,
		cfg.SubAgents.MaxConcurrent,
		a.AudioProvider,
	)
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
