package serve

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"

	"github.com/neirth/openlobster/internal/application/graphql/subscriptions"
	appcontext "github.com/neirth/openlobster/internal/domain/context"
	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/repositories"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	inframc "github.com/neirth/openlobster/internal/infrastructure/adapters/mcp"
	browser "github.com/neirth/openlobster/internal/infrastructure/adapters/browser/chromedp"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/filesystem"
	pluginadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/plugin"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/terminal"
)

// initPlugins loads all WASM plugins from cfg.Plugins.Dir, registers them in
// PluginRegistry, and wires AI/memory/messaging adapters derived from plugins
// into the application.  Must be called before initServices().
func (a *App) initPlugins() {
	a.PluginRegistry = pluginadapter.NewRegistry()

	// Initialise ChanReg/MsgRouter here so messaging plugins can register.
	a.initChannels()

	ctx := context.Background()
	plugins, err := pluginadapter.LoadPlugins(ctx, a.Cfg.Plugins.Dir, a.onPluginMessage)
	if err != nil {
		log.Printf("plugins: failed to load: %v", err)
		return
	}

	for _, p := range plugins {
		a.PluginRegistry.Register(p)
		cfg := a.Cfg.Plugins.Settings[p.ID()]

		switch p.Type() {
		case "ai":
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
			wrapper := pluginadapter.NewMessagingWrapper(p, channelType, cfg)
			a.ChanReg.Set(channelType, wrapper)
			a.MessagingAdapters = append(a.MessagingAdapters, wrapper)
			log.Printf("plugins: messaging channel → %s (%s)", channelType, p.Name())

		case "memory":
			if a.MemoryAdapter == nil {
				a.MemoryAdapter = pluginadapter.NewMemoryWrapper(p, cfg)
				log.Printf("plugins: memory backend → %s", p.Name())
			}
		}
	}

	if a.AIProvider == nil {
		log.Println("warn: no AI plugin loaded — agent will not respond to messages")
	}
	if a.MemoryAdapter == nil {
		log.Println("warn: no memory plugin loaded — using nil memory backend")
	}
}

// onPluginMessage is the callback invoked by messaging plugins (via
// host_emit_message) to deliver inbound messages to the message handler.
func (a *App) onPluginMessage(msgJSON []byte) {
	if a.MsgHandler == nil {
		return
	}
	var msg models.Message
	if err := json.Unmarshal(msgJSON, &msg); err != nil {
		log.Printf("plugins: malformed inbound message from plugin: %v", err)
		return
	}
	ct := ""
	if msg.Metadata != nil {
		if v, ok := msg.Metadata["channel_type"].(string); ok {
			ct = v
		}
	}
	if msg.Content == "" && len(msg.Attachments) == 0 && msg.Audio == nil {
		return
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
