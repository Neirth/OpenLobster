package dto

// DTOs para el API GraphQL del dashboard. Capa de presentación.

type AgentSnapshot struct {
	ID            string
	Name          string
	Version       string
	Status        string
	Uptime        int64
	Provider      string
	AIProvider    string
	MemoryBackend string
	ToolsCount    int
	TasksCount    int
	Channels      []ChannelStatus
}

type ChannelStatus struct {
	ID           string
	Name         string
	Type         string
	Status       string
	Enabled      bool
	Capabilities ChannelCapabilities
}

type ChannelCapabilities struct {
	HasVoiceMessage bool
	HasCallStream   bool
	HasTextStream   bool
	HasMediaSupport bool
}

type TaskSnapshot struct {
	ID        string
	Prompt    string
	Status    string
	Schedule  string
	TaskType  string
	Enabled   bool
	CreatedAt string
	LastRunAt string
	NextRunAt string
	IsCyclic  bool
}

type ConversationSnapshot struct {
	ID              string
	ChannelID       string
	ChannelType     string
	ChannelName     string
	GroupName       string
	IsGroup         bool
	ParticipantID   string
	ParticipantName string
	LastMessageAt   string
	UnreadCount     int
}

type AttachmentSnapshot struct {
	Type     string
	URL      string
	Filename string
	MIMEType string
}

type MessageSnapshot struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	CreatedAt      string
	Attachments    []AttachmentSnapshot
}

type ToolSnapshot struct {
	Name        string
	Description string
	Source      string
	ServerName  string // Set for MCP tools (e.g. from "serverName:toolName"); empty for internal
}

type MCPSnapshot struct {
	Name   string
	Type   string
	Status string
	URL    string
	Tools  []ToolSnapshot
}

type SubAgentSnapshot struct {
	ID     string
	Name   string
	Status string
	Task   string
}

type MCPServerRecord struct {
	Name      string
	URL       string
	Status    string // "online", "unknown", etc.; populated by resolver
	ToolCount int    // number of tools exposed by this server; populated by resolver
}

type ToolPermissionRecord struct {
	UserID   string
	ToolName string
	Mode     string
}

type SkillSnapshot struct {
	Name        string
	Description string
	Enabled     bool
	Path        string
}

type GraphNodeSnapshot struct {
	ID         string
	Label      string
	Type       string
	Value      string
	Properties map[string]string
}

type GraphEdgeSnapshot struct {
	Source string
	Target string
	Label  string
}

type GraphSnapshot struct {
	Nodes []GraphNodeSnapshot
	Edges []GraphEdgeSnapshot
}

type MetricsSnapshot struct {
	Uptime           int64
	MessagesReceived int64
	MessagesSent     int64
	ActiveSessions   int64
	MemoryNodes      int64
	MemoryEdges      int64
	McpTools         int64
	TasksPending     int64
	TasksRunning     int64
	TasksDone        int64
	ErrorsTotal      int64
}

type StatusSnapshot struct {
	Agent     *AgentSnapshot
	Health    *HeartbeatSnapshot
	Channels  []ChannelStatus
	Tools     []ToolSnapshot
	SubAgents []SubAgentSnapshot
	Tasks     []TaskSnapshot
	Mcps      []MCPSnapshot
}

type HeartbeatSnapshot struct {
	Status    string
	LastCheck int64
}

type SendMessageResult struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	CreatedAt      string
}

type PairingSnapshot struct {
	Code   string
	Status string
}

type UserSnapshot struct {
	ID          string
	DisplayName string
}

type SystemFileSnapshot struct {
	Name         string
	Path         string
	Content      string
	LastModified string
}

type AppConfigSnapshot struct {
	Agent           *AgentConfigSnapshot
	Capabilities    *CapabilitiesSnapshot
	Database        *DatabaseConfigSnapshot
	Memory          *MemoryConfigSnapshot
	Subagents       *SubagentsConfigSnapshot
	GraphQL         *GraphQLConfigSnapshot
	Logging         *LoggingConfigSnapshot
	Secrets         *SecretsConfigSnapshot
	Scheduler       *SchedulerConfigSnapshot
	ActiveSessions  []ActiveSessionSnapshot
	Channels        []ChannelConfigSnapshot
	ChannelSecrets  *ChannelSecretsSnapshot
	WizardCompleted bool
}

type AgentConfigSnapshot struct {
	Name                      string
	SystemPrompt              string
	Provider                  string
	Model                     string
	APIKey                    string
	BaseURL                   string
	OllamaHost                string
	OllamaApiKey              string
	AnthropicApiKey           string
	DockerModelRunnerEndpoint string
	DockerModelRunnerModel    string
}

type CapabilitiesSnapshot struct {
	Browser    bool
	Terminal   bool
	Subagents  bool
	Memory     bool
	MCP        bool
	Filesystem bool
	Sessions   bool
}

type DatabaseConfigSnapshot struct {
	Driver       string
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

type MemoryConfigSnapshot struct {
	Backend  string
	FilePath string
	Neo4j    *Neo4jConfigSnapshot
	Postgres *PostgresConfigSnapshot
}

type Neo4jConfigSnapshot struct {
	URI      string
	User     string
	Password string
}

type PostgresConfigSnapshot struct {
	DSN string
}

type SubagentsConfigSnapshot struct {
	MaxConcurrent  int
	DefaultTimeout string
}

type GraphQLConfigSnapshot struct {
	Enabled bool
	Port    int
	Host    string
	BaseURL string
}

type LoggingConfigSnapshot struct {
	Level string
	Path  string
}

type SecretsConfigSnapshot struct {
	Backend string
	File    *FileSecretsSnapshot
	Openbao *OpenbaoSecretsSnapshot
}

type FileSecretsSnapshot struct {
	Path string
}

type OpenbaoSecretsSnapshot struct {
	URL   string
	Token string
}

type SchedulerConfigSnapshot struct {
	Enabled        bool
	MemoryEnabled  bool
	MemoryInterval string
}

type ActiveSessionSnapshot struct {
	ID      string
	Address string
	Status  string
	Channel string
	User    string
}

type ChannelConfigSnapshot struct {
	ChannelID   string
	ChannelName string
	Enabled     bool
}

type ChannelSecretsSnapshot struct {
	TelegramEnabled  bool
	TelegramToken    string
	DiscordEnabled   bool
	DiscordToken     string
	WhatsAppEnabled  bool
	WhatsAppPhoneId  string
	WhatsAppApiToken string
	TwilioEnabled    bool
	TwilioAccountSid string
	TwilioAuthToken  string
	TwilioFromNumber string
	SlackEnabled     bool
	SlackBotToken    string
	SlackAppToken    string
}
