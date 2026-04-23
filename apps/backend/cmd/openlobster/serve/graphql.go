package serve

import (
	"context"
	"log"

	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	appmcp "github.com/neirth/openlobster/internal/application/mcp"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/repositories"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/filesystem"
	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
)

// initGraphQL wires the agent registry, GraphQL deps struct, config writer
// and the graphql.Handler that serves the dashboard API.
func (a *App) initGraphQL() {
	cfg := a.Cfg

	a.AgentRegistry = registry.NewAgentRegistry()

	agentName := cfg.Agent.Name
	if agentName == "" {
		agentName = "OpenLobster"
	}
	provider := a.activeProviderName()
	channels := a.rebuildActiveChannels()
	a.AgentRegistry.UpdateAgent(&dto.AgentSnapshot{
		ID: "openlobster", Name: agentName, Version: a.Version, Status: "running",
		Provider: provider, Channels: channels, AIProvider: provider,
		MemoryBackend: string(cfg.Memory.Backend),
	})
	a.AgentRegistry.UpdateAgentChannels(channels)
	appmcp.SyncToolsToRegistry(a.ToolRegistry, a.AgentRegistry)

	logging.Debugf("DEBUG: GRAPHQL: Wiring Dashboard Services...")
	logging.Debugf("DEBUG: GRAPHQL: MemoryAdapter present? %v", a.MemoryAdapter != nil)
	
	queryService := domainservices.NewDashboardQueryService(
		a.TaskRepo, a.MemoryAdapter, a.MemoryAdapter, nil, nil,
	)
	commandService := domainservices.NewDashboardCommandService(
		a.TaskRepo, a.MemoryAdapter, a.MemoryAdapter,
	)
	commandService.SetTaskNotifier(func() {
		if a.SchedulerNotify != nil {
			a.SchedulerNotify()
		}
	})
	a.QueryService = queryService
	a.CommandService = commandService

	configSnapshot := dto.BuildConfigSnapshot(cfg, func(_ *config.Config) string { return a.activeProviderName() })
	subAgentAdapter := dto.NewSubAgentAdapter(a.SubAgentSvc)

	a.Deps = &resolvers.Deps{
		PluginRegistry:  a.PluginRegistry,
		AgentRegistry:   a.AgentRegistry,
		QuerySvc:        queryService,
		CommandSvc:      commandService,
		TaskRepo:        a.TaskRepo,
		MemoryRepo:      a.MemoryAdapter,
		MsgRepo:         &dto.MsgRepoAdapter{Repo: a.DashMsgRepo},
		ConvPort:        &dto.ConversationPortAdapter{Repo: a.ConvRepo},
		SkillsPort:      a.SkillsAdapter,
		SysFilesPort:    filesystem.NewSystemFilesAdapter(cfg.Workspace.Path),
		ToolPermRepo:    &dto.ToolPermAdapter{Repo: a.ToolPermRepo},
		ToolNamesSource: &appmcp.ToolNamesAdapter{Reg: a.ToolRegistry},
		MCPServerRepo:   &dto.MCPServerAdapter{Repo: a.MCPServerRepo},
		SubAgentSvc:     subAgentAdapter,
		PairingPort: &dto.PairingPortAdapter{
			Svc:             a.PairingService,
			UserRepo:        a.UserRepo,
			UserChannelRepo: a.UserChannelRepo,
			ChannelRepo:     repositories.NewChannelRepository(a.db.GormDB()),
			MessageSender:   a.ChanReg,
			EventBus:        a.EventBus,
		},
		UserRepo:          &dto.UserRepoAdapter{Repo: a.UserRepo},
		UserChannelRepo:   a.UserChannelRepo,
		MessageSender:     a.ChanReg,
		MessageDispatcher: a.MsgHandler,
		EventBus:          &dto.EventBusAdapter{Eb: a.EventBus},
		AIProvider:        a.AIProvider,
		ConfigSnapshot:    configSnapshot,
		ConfigPath:        a.CfgPath,
		ReloadPlugins: func(_ context.Context) error {
			a.SyncMessagingRuntime()
			if a.AgentRegistry != nil {
				a.AgentRegistry.UpdateAgentChannels(a.rebuildActiveChannels())
			}
			return nil
		},
	}

	a.MsgHandler.SetCapabilitiesChecker(func(cap string) bool {
		if a.Deps.ConfigSnapshot == nil || a.Deps.ConfigSnapshot.Capabilities == nil {
			return true
		}
		switch cap {
		case "browser":
			return a.Deps.ConfigSnapshot.Capabilities.Browser
		case "terminal":
			return a.Deps.ConfigSnapshot.Capabilities.Terminal
		case "subagents":
			return a.Deps.ConfigSnapshot.Capabilities.Subagents
		case "memory":
			return a.Deps.ConfigSnapshot.Capabilities.Memory
		case "mcp":
			return a.Deps.ConfigSnapshot.Capabilities.MCP
		case "filesystem":
			return a.Deps.ConfigSnapshot.Capabilities.Filesystem
		case "sessions":
			return a.Deps.ConfigSnapshot.Capabilities.Sessions
		default:
			return true
		}
	})

	// Keep subagent tool visibility aligned with the main agent.
	a.SubAgentSvc.SetCapabilitiesChecker(func(cap string) bool {
		if a.Deps.ConfigSnapshot == nil || a.Deps.ConfigSnapshot.Capabilities == nil {
			return true
		}
		switch cap {
		case "browser":
			return a.Deps.ConfigSnapshot.Capabilities.Browser
		case "terminal":
			return a.Deps.ConfigSnapshot.Capabilities.Terminal
		case "subagents":
			return a.Deps.ConfigSnapshot.Capabilities.Subagents
		case "memory":
			return a.Deps.ConfigSnapshot.Capabilities.Memory
		case "mcp":
			return a.Deps.ConfigSnapshot.Capabilities.MCP
		case "filesystem":
			return a.Deps.ConfigSnapshot.Capabilities.Filesystem
		case "sessions":
			return a.Deps.ConfigSnapshot.Capabilities.Sessions
		default:
			return true
		}
	})

	a.ConfigWriter = &dto.ConfigUpdateAdapter{
		ConfigPath:    a.CfgPathAbs,
		ReloadChannel: a.reloadChannel,
		ViperKeys:     dto.InputToViperKeyMap(),
		OnApplied: func(touched bool) {
			reloaded, err := config.Load(a.CfgPathAbs)
			if err != nil {
				log.Printf("config: failed to reload after save: %v", err)
				return
			}
			a.Cfg = reloaded
			
			// Diagnostics: what did we actually load?
			providerName := a.activeProviderName()
			log.Printf("config: reloaded core settings (agent=%s provider=%s)", reloaded.Agent.Name, providerName)
			if reloaded.Channels.Telegram.Enabled {
				log.Printf("config: reloaded telegram channel enabled, has_token=%v", reloaded.Channels.Telegram.BotToken != "")
			}

			a.Deps.ConfigSnapshot = dto.BuildConfigSnapshot(reloaded, func(_ *config.Config) string { return providerName })
			
			// Always rebuild messaging runtime when config changes to pick up enabled/disabled channels
			if a.Deps.ReloadPlugins != nil {
				if err := a.Deps.ReloadPlugins(context.Background()); err != nil {
					log.Printf("config: runtime plugin/channel reconciliation failed: %v", err)
				}
			}

			// Update agent registry with fresh config data
			if cur := a.AgentRegistry.GetAgent(); cur != nil {
				name := reloaded.Agent.Name
				if name == "" {
					name = "OpenLobster"
				}
				updated := *cur
				updated.Name = name
				updated.Provider = providerName
				updated.AIProvider = providerName
				a.AgentRegistry.UpdateAgent(&updated)
			}

			if touched {
				log.Printf("config: critical path touched, performing runtime reconciliation")
				
				// Propagation: Update config on live wrappers
				if c, ok := a.AIProvider.(ports.Configurable); ok {
					if wp, ok := a.AIProvider.(interface{ Plugin() ports.PluginPort }); ok {
						c.UpdateConfig(liveConfigForPluginFromApp(a, wp.Plugin()))
					}
				}
				if c, ok := a.MemoryAdapter.(ports.Configurable); ok {
					if wp, ok := a.MemoryAdapter.(interface{ Plugin() ports.PluginPort }); ok {
						c.UpdateConfig(liveConfigForPluginFromApp(a, wp.Plugin()))
					}
				}
				if c, ok := a.SecretsProvider.(ports.Configurable); ok {
					if wp, ok := a.SecretsProvider.(interface{ Plugin() ports.PluginPort }); ok {
						c.UpdateConfig(liveConfigForPluginFromApp(a, wp.Plugin()))
					}
				}
			}
		},
	}
	a.Deps.ConfigWriter = a.ConfigWriter
	a.Deps.SkillsPort = a.SkillsAdapter

	a.HTTPHandler = graphql.NewHandler(a.Deps)
}

// activeProviderName returns the human-readable name of the current AI provider.
// When backed by a plugin, it uses the plugin's Name(); otherwise "none".
func (a *App) activeProviderName() string {
	if a.Cfg != nil && a.Cfg.Agent.Provider != "" {
		return a.Cfg.Agent.Provider
	}
	if a.PluginRegistry != nil {
		aiPlugins := a.PluginRegistry.GetByType("ai")
		if len(aiPlugins) > 0 {
			return aiPlugins[0].Name()
		}
	}
	return "none"
}
