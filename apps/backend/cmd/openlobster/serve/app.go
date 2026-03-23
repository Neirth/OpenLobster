// Package serve wires all openlobster components and manages the
// application lifecycle. Called from the "serve" subcommand.
package serve

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	appmcp "github.com/neirth/openlobster/internal/application/mcp"
	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/neirth/openlobster/internal/application/graphql/subscriptions"
	"github.com/neirth/openlobster/internal/application/registry"
	appcontext "github.com/neirth/openlobster/internal/domain/context"
	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	msgrouter "github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/router"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/filesystem"
	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/neirth/openlobster/internal/infrastructure/secrets"
)

// App holds every component wired during startup. All init* methods populate
// its fields; lifecycle methods (startAndWait) use them.
type App struct {
	// Meta
	Version  string
	PublicFS fs.FS

	// CLI flag overrides (set by Command before calling Run)
	FlagHost    string
	FlagPort    int
	FlagDataDir string

	// Config
	CfgPath    string
	CfgPathAbs string
	Cfg        *config.Config

	// Database (private raw handle, use GormDB via persistence.Database)
	db          *persistence.Database
	TaskRepo    repositories.TaskRepository
	MessageRepo ports.MessageRepositoryPort
	SessionRepo repositories.SessionRepository
	UserRepo    ports.UserRepositoryPort
	ConvRepo    *repositories.ConversationRepository
	DashMsgRepo *repositories.DashboardMessageRepository

	ToolPermRepo  repositories.ToolPermissionRepositoryPort
	MCPServerRepo repositories.MCPServerRepositoryPort
	PairingRepo   ports.PairingRepositoryPort
	UserChannelRepo ports.UserChannelRepositoryPort

	// Infrastructure
	AIProvider    ports.AIProviderPort
	MemoryAdapter ports.MemoryPort

	// Messaging
	ChanReg          *msgrouter.Registry
	MsgRouter        *msgrouter.Router
	MessagingAdapters []ports.MessagingPort

	// Domain services
	EventBus       domainservices.EventBus
	SubManager     *subscriptions.SubscriptionManager
	PairingService *domainservices.PairingService
	PermManager    *permissions.Manager
	ToolRegistry   *mcp.ToolRegistry

	CompactionSvc *domainservices.MessageCompactionService
	SubAgentSvc   *domainservices.SubAgentService
	CtxInjector   appcontext.ContextInjector
	MsgHandler    *domainhandlers.MessageHandler
	SkillsAdapter *filesystem.SkillsAdapter

	SchedulerNotify               func()
	SchedulerUpdateMemoryInterval func(time.Duration)

	// Application layer
	AgentRegistry  *registry.AgentRegistry
	QueryService   *domainservices.DashboardQueryService
	CommandService *domainservices.DashboardCommandService
	Deps           *resolvers.Deps
	HTTPHandler    *graphql.Handler
	ConfigWriter   *dto.ConfigUpdateAdapter

	// MCP
	SecretsProvider secrets.SecretsProvider
	MCPClientSDK    *mcp.MCPClientSDK
	OAuthMgr        *mcp.OAuthManager
	McpConnectPort  *appmcp.ConnectAdapter
	McpOAuthPort    *appmcp.OAuthAdapter

	// HTTP
	Mux        *http.ServeMux
	HTTPServer *http.Server

	// Lifecycle
	Ctx             context.Context
	Cancel          context.CancelFunc
	ChannelStartCtx context.Context
}

// New returns an uninitialised App. Call Run() to start the daemon.
func New(version string, publicFS fs.FS) *App {
	return &App{Version: version, PublicFS: publicFS}
}

// Run initialises all subsystems in order and blocks until a shutdown signal
// is received.
func (a *App) Run() {
	a.initConfig()
	a.initWorkspace()
	a.initDatabase()
	a.initChannels()
	a.initServices()
	a.initGraphQL()
	a.initMCP()
	a.initHTTP()
	a.startAndWait()
}
