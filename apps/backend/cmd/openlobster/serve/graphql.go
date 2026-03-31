package serve

import (
	"log"

	appmcp "github.com/neirth/openlobster/internal/application/mcp"
	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/neirth/openlobster/internal/application/registry"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/repositories"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/filesystem"
	"github.com/neirth/openlobster/internal/infrastructure/config"
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
		OnApplied: func(providerTouched bool) {
			reloaded, err := config.Load(a.CfgPathAbs)
			if err != nil {
				log.Printf("config: failed to reload after save: %v", err)
				return
			}
			providerName := a.activeProviderName()
			a.Deps.ConfigSnapshot = dto.BuildConfigSnapshot(reloaded, func(_ *config.Config) string { return providerName })
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
			if a.SchedulerUpdateMemoryInterval != nil && reloaded.Scheduler.MemoryInterval != cfg.Scheduler.MemoryInterval {
				a.SchedulerUpdateMemoryInterval(reloaded.Scheduler.MemoryInterval)
				log.Printf("config: scheduler memory interval updated to %s", reloaded.Scheduler.MemoryInterval)
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
	if a.PluginRegistry != nil {
		aiPlugins := a.PluginRegistry.GetByType("ai")
		if len(aiPlugins) > 0 {
			return aiPlugins[0].Name()
		}
	}
	return "none"
}
