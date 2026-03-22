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
	aifactory "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/factory"
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
	provider := aifactory.ProviderName(cfg)
	channels := buildInitialChannels(cfg)
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

	configSnapshot := dto.BuildConfigSnapshot(cfg, aifactory.ProviderName)
	subAgentAdapter := dto.NewSubAgentAdapter(a.SubAgentSvc)

	a.Deps = &resolvers.Deps{
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
			a.Deps.ConfigSnapshot = dto.BuildConfigSnapshot(reloaded, aifactory.ProviderName)
			if cur := a.AgentRegistry.GetAgent(); cur != nil {
				name := reloaded.Agent.Name
				if name == "" {
					name = "OpenLobster"
				}
				updated := *cur
				updated.Name = name
				updated.Provider = aifactory.ProviderName(reloaded)
				updated.AIProvider = aifactory.ProviderName(reloaded)
				a.AgentRegistry.UpdateAgent(&updated)
			}
			if providerTouched {
				newProvider := aifactory.BuildFromConfig(reloaded)
				a.MsgHandler.SetAIProvider(newProvider)
				a.CompactionSvc.SetAIProvider(newProvider)
				a.Deps.AIProvider = newProvider
				log.Printf("config: soft reboot — AI provider reloaded")
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

// buildInitialChannels returns the ChannelStatus list for channels that have
// real (non-placeholder) credentials configured at startup.
func buildInitialChannels(cfg *config.Config) []dto.ChannelStatus {
	isReal := func(s string) bool {
		return s != "" &&
			s != "YOUR_BOT_TOKEN_HERE" &&
			s != "YOUR_ACCOUNT_SID" &&
			s != "YOUR_ACCOUNT_SID_HERE" &&
			s != "YOUR_AUTH_TOKEN" &&
			s != "YOUR_API_KEY_HERE"
	}
	var ch []dto.ChannelStatus
	if cfg.Channels.Discord.Enabled && isReal(cfg.Channels.Discord.BotToken) {
		ch = append(ch, dto.ChannelStatus{
			ID: "discord", Name: "Discord", Type: "discord", Status: "online",
			Enabled: true,
			Capabilities: dto.ChannelCapabilities{
				HasVoiceMessage: true, HasTextStream: true, HasMediaSupport: true,
			},
		})
	}
	if cfg.Channels.Telegram.Enabled && isReal(cfg.Channels.Telegram.BotToken) {
		ch = append(ch, dto.ChannelStatus{
			ID: "telegram", Name: "Telegram", Type: "telegram", Status: "online",
			Enabled: true,
			Capabilities: dto.ChannelCapabilities{
				HasVoiceMessage: true, HasTextStream: true, HasMediaSupport: true,
			},
		})
	}
	return ch
}
