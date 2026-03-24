package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/spf13/viper"
)

// bindEnvForAllKeys binds Viper keys to environment variable names using the
// OPENLOBSTER_ prefix and converting dots to underscores. This ensures that
// nested keys (e.g. memory.neo4j.uri) are bound to env vars like
// OPENLOBSTER_MEMORY_NEO4J_URI so that viper.Unmarshal picks them up.
func bindEnvForAllKeys() {
	const prefix = "OPENLOBSTER_"
	// Bind any keys already present in viper (from config file or defaults).
	for _, k := range viper.AllKeys() {
		envKey := prefix + strings.ToUpper(strings.ReplaceAll(k, ".", "_"))
		_ = viper.BindEnv(k, envKey)
	}
	// Do not bind hard-coded keys here; bindEnvFromOS handles env-only cases
	// and viper.AllKeys covers keys present in config files or defaults.
}

// bindEnvFromOS scans the process environment for OPENLOBSTER_* variables
// and binds corresponding viper keys (reverse mapping: OPENLOBSTER_FOO_BAR -> foo.bar).
func bindEnvFromOS() {
	const prefix = "OPENLOBSTER_"
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(name, prefix), "_", "."))

		_ = viper.BindEnv(key, name)
	}
}

// placeholders are values that indicate a field has not been configured.
var placeholders = []string{
	"YOUR_API_KEY_HERE",
	"YOUR_BOT_TOKEN_HERE",
	"YOUR_ACCOUNT_SID",
	"YOUR_AUTH_TOKEN",
}

// isPlaceholder returns true if s is empty or a known placeholder value.
func isPlaceholder(s string) bool {
	if s == "" {
		return true
	}
	for _, p := range placeholders {
		if strings.EqualFold(s, p) {
			return true
		}
	}
	return false
}

// ValidationError accumulates configuration errors found during Validate.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("configuration is invalid:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// Validate checks the configuration for missing or invalid required fields.
// It returns a *ValidationError listing all problems found, or nil if the
// configuration is valid.
func (c *Config) Validate() error {
	var errs []string

	// Database: driver must be a known value; DSN must not be empty.
	switch c.Database.Driver {
	case "sqlite3", "sqlite":
		if c.Database.DSN == "" {
			errs = append(errs, "database.dsn is required for sqlite (e.g. ./data/openlobster.db)")
		}
	case "postgres", "pgx", "mysql":
		if c.Database.DSN == "" {
			errs = append(errs, fmt.Sprintf("database.dsn is required for %s (connection string)", c.Database.Driver))
		}
	case "":
		errs = append(errs, "database.driver is required; supported values: sqlite, postgres, mysql")
	default:
		errs = append(errs, fmt.Sprintf("database.driver %q is not supported; supported values: sqlite, postgres, mysql", c.Database.Driver))
	}

	// GraphQL: port must be in valid range.
	if c.GraphQL.Port < 1 || c.GraphQL.Port > 65535 {
		errs = append(errs, fmt.Sprintf("graphql.port must be between 1 and 65535, got %d", c.GraphQL.Port))
	}

	// Memory backend: validate required fields per backend type.
	switch c.Memory.Backend {
	case models.MemoryFile:
		if c.Memory.File.Path == "" {
			errs = append(errs, "memory.file.path is required when memory.backend is \"file\"")
		}
	case models.MemoryNeo4j:
		if c.Memory.Neo4j.URI == "" {
			errs = append(errs, "memory.neo4j.uri is required when memory.backend is \"neo4j\"")
		}
		if c.Memory.Neo4j.User == "" {
			errs = append(errs, "memory.neo4j.user is required when memory.backend is \"neo4j\"")
		}
		if c.Memory.Neo4j.Password == "" {
			errs = append(errs, "memory.neo4j.password is required when memory.backend is \"neo4j\"")
		}
	case "":
		errs = append(errs, "memory.backend is required (\"file\" or \"neo4j\")")
	default:
		errs = append(errs, fmt.Sprintf("memory.backend %q is not supported; use \"file\" or \"neo4j\"", c.Memory.Backend))
	}

	// AI provider: at least one must be configured.
	hasOpenAI := !isPlaceholder(c.Providers.OpenAI.APIKey)
	hasOpenRouter := !isPlaceholder(c.Providers.OpenRouter.APIKey)
	hasOllama := c.Providers.Ollama.Endpoint != ""
	hasOpenAICompat := !isPlaceholder(c.Providers.OpenAICompat.APIKey) && c.Providers.OpenAICompat.BaseURL != ""
	hasAnthropic := !isPlaceholder(c.Providers.Anthropic.APIKey)
	hasDockerModelRunner := c.Providers.DockerModelRunner.Endpoint != ""
	hasOpenCode := !isPlaceholder(c.Providers.OpenCode.APIKey)
	if !hasOpenAI && !hasOpenRouter && !hasOllama && !hasOpenAICompat && !hasAnthropic && !hasDockerModelRunner && !hasOpenCode {
		errs = append(errs, "at least one AI provider must be configured: providers.openai.api_key, providers.openrouter.api_key, providers.ollama.endpoint, providers.openaicompat (api_key + base_url), providers.anthropic.api_key, providers.docker_model_runner.endpoint, or providers.opencode.api_key")
	}

	// Scheduler: interval must be positive when enabled.
	if c.Scheduler.Enabled && c.Scheduler.Interval <= 0 {
		errs = append(errs, "scheduler.interval must be a positive duration when scheduler.enabled is true")
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ResolvePaths converts all relative paths in the configuration into absolute
// paths, anchored at the BaseDir. This must be called before components are
// initialized if the process intends to chdir() at runtime.
func (c *Config) ResolvePaths() {
	makeAbs := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(c.BaseDir, p)
	}

	// SQLite DSN is special: it might have prefixes like "file:".
	// We only touch it if it's a simple path.
	if (c.Database.Driver == "sqlite" || c.Database.Driver == "sqlite3") &&
		!strings.Contains(c.Database.DSN, ":") {
		c.Database.DSN = makeAbs(c.Database.DSN)
	}

	c.Memory.File.Path = makeAbs(c.Memory.File.Path)
	c.Logging.Path = makeAbs(c.Logging.Path)
	c.Secrets.File.Path = makeAbs(c.Secrets.File.Path)
	c.Workspace.Path = makeAbs(c.Workspace.Path)
}

type Config struct {
	// BaseDir is the root directory for all runtime data (data/, logs/,
	// workspace/). Configurable via base_dir in YAML, OPENLOBSTER_BASE_DIR
	// env var, or the --data-dir CLI flag. Defaults to $HOME/.openlobster.
	BaseDir     string            `mapstructure:"base_dir"`
	Agent       AgentConfig       `mapstructure:"agent"`
	Scheduler   SchedulerConfig   `mapstructure:"scheduler"` 
	Database    DatabaseConfig    `mapstructure:"database"`
	Providers   ProvidersConfig   `mapstructure:"providers"`
	Channels    ChannelsConfig    `mapstructure:"channels"`
	Memory      MemoryConfig      `mapstructure:"memory"`
	MCP         MCPConfig         `mapstructure:"mcp"`
	SubAgents   SubAgentsConfig   `mapstructure:"subagents"`
	GraphQL     GraphQLConfig     `mapstructure:"graphql"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Permissions PermissionsConfig `mapstructure:"permissions"`
	Secrets     SecretsConfig     `mapstructure:"secrets"`
	Workspace   WorkspaceConfig   `mapstructure:"workspace"`
	Wizard      WizardConfig      `mapstructure:"wizard"`
}


// WizardConfig holds first-boot wizard state (server-side).
type WizardConfig struct {
	Completed bool `mapstructure:"completed"`
}

type AgentConfig struct {
	Name           string             `mapstructure:"name"`
	SystemPrompt   string             `mapstructure:"system_prompt"`
	Provider       string             `mapstructure:"provider"`
	ReasoningLevel string             `mapstructure:"reasoning_level"` // none, low, medium, high
	Capabilities   CapabilitiesConfig `mapstructure:"capabilities"`
}

type CapabilitiesConfig struct {
	Browser    bool `mapstructure:"browser"`
	Terminal   bool `mapstructure:"terminal"`
	Subagents  bool `mapstructure:"subagents"`
	Memory     bool `mapstructure:"memory"`
	MCP        bool `mapstructure:"mcp"`
	Filesystem bool `mapstructure:"filesystem"`
	Sessions   bool `mapstructure:"sessions"`
}

// SchedulerConfig holds task scheduler settings (formerly HeartbeatConfig).
type SchedulerConfig struct {
	Interval       time.Duration `mapstructure:"interval"`
	Enabled        bool          `mapstructure:"enabled"`
	MemoryInterval time.Duration `mapstructure:"memory_interval"`
	MemoryEnabled  bool          `mapstructure:"memory_enabled"`
}

type DatabaseConfig struct {
	Driver       string `mapstructure:"driver"`
	DSN          string `mapstructure:"dsn"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type MemoryConfig struct {
	Backend  models.MemoryBackendType `mapstructure:"backend"`
	File     MemoryFileConfig         `mapstructure:"file"`
	Neo4j    MemoryNeo4jConfig        `mapstructure:"neo4j"`
	Postgres PostgresMemoryConfig     `mapstructure:"postgres"`
}

type MemoryFileConfig struct {
	Path string `mapstructure:"path"`
}

type MemoryNeo4jConfig struct {
	URI      string `mapstructure:"uri"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type PostgresMemoryConfig struct {
	DSN string `mapstructure:"dsn"`
}

type ProvidersConfig struct {
	OpenRouter        OpenRouterConfig        `mapstructure:"openrouter"`
	Ollama            OllamaConfig            `mapstructure:"ollama"`
	OpenCode          OpenCodeConfig          `mapstructure:"opencode"`
	OpenAI            OpenAIConfig            `mapstructure:"openai"`
	OpenAICompat      OpenAICompatConfig      `mapstructure:"openaicompat"`
	Anthropic         AnthropicConfig         `mapstructure:"anthropic"`
	DockerModelRunner DockerModelRunnerConfig `mapstructure:"docker_model_runner"`
}

type OpenRouterConfig struct {
	APIKey        string `mapstructure:"api_key"`
	DefaultModel  string `mapstructure:"default_model"`
	// ContextWindow overrides the context window used for message chunking.
	// Required because OpenRouter does not expose this via API.
	ContextWindow int    `mapstructure:"context_window"`
}

type OllamaConfig struct {
	Endpoint     string `mapstructure:"endpoint"`
	DefaultModel string `mapstructure:"default_model"`
	APIKey       string `mapstructure:"api_key"`
	// ContextWindow overrides the value auto-detected from /api/show.
	ContextWindow int `mapstructure:"context_window"`
}

// OpenCodeConfig holds settings for the OpenCode Zen AI gateway.
// See https://opencode.ai/docs/zen/ for supported models.
type OpenCodeConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
	// ContextWindow overrides the context window used for message chunking.
	// Required because the OpenAI API does not expose this per-model.
	ContextWindow int `mapstructure:"context_window"`
}

// OpenAICompatConfig holds settings for a generic OpenAI-compatible provider.
type OpenAICompatConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
	// ContextWindow must be set explicitly since compatible providers vary widely.
	ContextWindow int `mapstructure:"context_window"`
}

// AnthropicConfig holds settings for the Anthropic Messages API.
type AnthropicConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
	// ContextWindow overrides the value auto-detected from the Anthropic models API.
	ContextWindow int `mapstructure:"context_window"`
}

// DockerModelRunnerConfig holds settings for Docker Desktop's Model Runner.
type DockerModelRunnerConfig struct {
	Endpoint      string `mapstructure:"endpoint"`
	DefaultModel  string `mapstructure:"default_model"`
	// ContextWindow overrides the context window used for message chunking.
	ContextWindow int    `mapstructure:"context_window"`
}

type ChannelsConfig struct {
	Telegram TelegramConfig `mapstructure:"telegram"`
	Discord  DiscordConfig  `mapstructure:"discord"`
	WhatsApp WhatsAppConfig `mapstructure:"whatsapp"`
	Twilio   TwilioConfig   `mapstructure:"twilio"`
	Slack    SlackConfig    `mapstructure:"slack"`
}

type TelegramConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
}

type DiscordConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
}

type WhatsAppConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	PhoneID  string `mapstructure:"phone_id"`
	APIToken string `mapstructure:"api_token"`
}

type TwilioConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	AccountSID  string `mapstructure:"account_sid"`
	AuthToken   string `mapstructure:"auth_token"`
	FromNumber  string `mapstructure:"from_number"`
	WebhookPath string `mapstructure:"webhook_path"`
}

// SlackConfig holds settings for the Slack Socket Mode adapter.
// BotToken is the Bot User OAuth Token (xoxb-…).
// AppToken is the App-Level Token (xapp-…) required for Socket Mode.
type SlackConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	AppToken string `mapstructure:"app_token"`
}

type MCPConfig struct {
	Servers []MCPServerConfig `mapstructure:"servers"`
}

type MCPServerConfig struct {
	Name    string   `mapstructure:"name"`
	Type    string   `mapstructure:"type"`
	Command string   `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
	Env     []string `mapstructure:"env"`
	URL     string   `mapstructure:"url"`
}

type SubAgentsConfig struct {
	MaxConcurrent  int           `mapstructure:"max_concurrent"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
}

type GraphQLConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Host    string `mapstructure:"host"`
	// BaseURL is the public URL of the server (e.g. https://openlobster.example.com).
	// Used for OAuth redirect_uri and other callbacks. If empty, derived from host:port.
	BaseURL string `mapstructure:"base_url"`
	// AuthEnabled gates the dashboard/API behind a token. Enabled by default.
	AuthEnabled bool `mapstructure:"auth_enabled"`
	// AuthToken is the bearer token required to access the GraphQL API when
	// AuthEnabled is true. The environment variable OPENLOBSTER_GRAPHQL_AUTH_TOKEN takes
	// precedence over this value at runtime.
	AuthToken string `mapstructure:"auth_token"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
	Path  string `mapstructure:"path"`
}

type PermissionsConfig struct {
	DefaultMode     string                    `mapstructure:"default_mode"`
	ToolPermissions map[string]ToolPermConfig `mapstructure:"tool_permissions"`
}

type ToolPermConfig struct {
	Mode string `mapstructure:"mode"`
	User string `mapstructure:"user"`
}

type SecretsConfig struct {
	Backend string                `mapstructure:"backend"`
	File    SecretsFileConfig     `mapstructure:"file"`
	Openbao *OpenbaoSecretsConfig `mapstructure:"openbao"`
}

type SecretsFileConfig struct {
	Path string `mapstructure:"path"`
}

type OpenbaoSecretsConfig struct {
	URL   string `mapstructure:"url"`
	Token string `mapstructure:"token"`
}

type WorkspaceConfig struct {
	Path string `mapstructure:"path"`
}

func setDefaults() {
	home, _ := os.UserHomeDir()
	viper.SetDefault("base_dir", filepath.Join(home, ".openlobster"))
	viper.SetDefault("scheduler.interval", "30s")
	viper.SetDefault("scheduler.enabled", true)
	viper.SetDefault("scheduler.memory_interval", "4h")
	viper.SetDefault("scheduler.memory_enabled", true)
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dsn", "./data/persistence.db")
	// Pool settings default to 0 (use driver's defaults) but are present so
	// they can be overridden cleanly via environment variables.
	viper.SetDefault("database.max_open_conns", 0)
	viper.SetDefault("database.max_idle_conns", 0)
	viper.SetDefault("memory.backend", "file")
	viper.SetDefault("memory.file.path", "./data/memory.gml")
	viper.SetDefault("memory.neo4j.uri", "")
	viper.SetDefault("memory.neo4j.user", "")
	viper.SetDefault("memory.neo4j.password", "")
	viper.SetDefault("secrets.backend", "file")
	viper.SetDefault("secrets.file.path", "./data/secrets.json")
	viper.SetDefault("secrets.openbao.url", "")
	viper.SetDefault("secrets.openbao.token", "")
	viper.SetDefault("workspace.path", "./workspace")
	viper.SetDefault("graphql.enabled", true)
	viper.SetDefault("graphql.port", 8080)
	viper.SetDefault("graphql.host", "0.0.0.0")
	viper.SetDefault("graphql.base_url", "")
	viper.SetDefault("graphql.auth_enabled", true)
	viper.SetDefault("graphql.auth_token", "")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.path", "./logs")
	viper.SetDefault("subagents.max_concurrent", 3)
	viper.SetDefault("subagents.default_timeout", "5m")
	viper.SetDefault("permissions.default_mode", "deny")
	viper.SetDefault("permissions.tool_permissions.read_file.mode", "ask")
	viper.SetDefault("permissions.tool_permissions.write_file.mode", "ask")
	viper.SetDefault("permissions.tool_permissions.edit_file.mode", "ask")
	viper.SetDefault("permissions.tool_permissions.list_content.mode", "always")
	viper.SetDefault("permissions.tool_permissions.terminal_exec.mode", "ask")
	viper.SetDefault("permissions.tool_permissions.send_message.mode", "always")
	// Default AI provider: Ollama (configurable from frontend)
	viper.SetDefault("providers.ollama.endpoint", "http://localhost:11434")
	viper.SetDefault("providers.ollama.default_model", "")
	viper.SetDefault("providers.ollama.api_key", "")
	viper.SetDefault("providers.openrouter.api_key", "")
	viper.SetDefault("providers.openrouter.default_model", "")
	viper.SetDefault("providers.opencode.api_key", "")
	viper.SetDefault("providers.opencode.model", "")
	viper.SetDefault("providers.openai.api_key", "")
	viper.SetDefault("providers.openai.model", "")
	viper.SetDefault("providers.openai.base_url", "")
	viper.SetDefault("providers.openaicompat.api_key", "")
	viper.SetDefault("providers.openaicompat.model", "")
	viper.SetDefault("providers.openaicompat.base_url", "")
	viper.SetDefault("providers.anthropic.api_key", "")
	viper.SetDefault("providers.anthropic.model", "")
	viper.SetDefault("providers.docker_model_runner.endpoint", "http://host.docker.internal:12434/engines/v1")
	viper.SetDefault("providers.docker_model_runner.default_model", "")
	viper.SetDefault("channels.telegram.enabled", false)
	viper.SetDefault("channels.telegram.bot_token", "")
	viper.SetDefault("channels.discord.enabled", false)
	viper.SetDefault("channels.discord.bot_token", "")
	viper.SetDefault("channels.whatsapp.enabled", false)
	viper.SetDefault("channels.whatsapp.phone_id", "")
	viper.SetDefault("channels.whatsapp.api_token", "")
	viper.SetDefault("channels.twilio.enabled", false)
	viper.SetDefault("channels.twilio.account_sid", "")
	viper.SetDefault("channels.twilio.auth_token", "")
	viper.SetDefault("channels.twilio.from_number", "")
	viper.SetDefault("channels.twilio.webhook_path", "")
	viper.SetDefault("channels.slack.enabled", false)
	viper.SetDefault("channels.slack.bot_token", "")
	viper.SetDefault("channels.slack.app_token", "")
	// Default agent name (shown in navbar)
	viper.SetDefault("agent.name", "OpenLobster")
	viper.SetDefault("agent.reasoning_level", "medium")
	// Default capabilities: all enabled except browser and terminal (opt-in)
	viper.SetDefault("agent.capabilities.browser", false)
	viper.SetDefault("agent.capabilities.terminal", false)
	viper.SetDefault("agent.capabilities.subagents", true)
	viper.SetDefault("agent.capabilities.memory", true)
	viper.SetDefault("agent.capabilities.mcp", true)
	viper.SetDefault("agent.capabilities.filesystem", true)
	viper.SetDefault("agent.capabilities.sessions", true)
	viper.SetDefault("wizard.completed", false)
}

// bootstrapEncryptedConfig creates a default config at path if the file does not exist.
// Encrypted if OPENLOBSTER_CONFIG_ENCRYPT is 1 (default), plain YAML if 0.
func bootstrapEncryptedConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // file exists, nothing to do
	} else if !os.IsNotExist(err) {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigFile(absPath)
	v.SetConfigType("yaml")
	v.SetDefault("scheduler.interval", "30s")
	v.SetDefault("scheduler.enabled", true)
	v.SetDefault("scheduler.memory_interval", "4h")
	v.SetDefault("scheduler.memory_enabled", true)
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "./data/openlobster.db")
	v.SetDefault("memory.backend", "file")
	v.SetDefault("memory.file.path", "./data/memory.gml")
	v.SetDefault("memory.neo4j.uri", "")
	v.SetDefault("memory.neo4j.user", "")
	v.SetDefault("memory.neo4j.password", "")
	v.SetDefault("secrets.backend", "file")
	v.SetDefault("secrets.file.path", "./data/secrets.json")
	v.SetDefault("workspace.path", "./workspace")
	v.SetDefault("graphql.enabled", true)
	v.SetDefault("graphql.port", 8080)
	v.SetDefault("graphql.host", "0.0.0.0")
	v.SetDefault("graphql.base_url", "")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.path", "./logs")
	v.SetDefault("subagents.max_concurrent", 3)
	v.SetDefault("subagents.default_timeout", "5m")
	v.SetDefault("permissions.default_mode", "deny")
	v.SetDefault("providers.ollama.endpoint", "http://localhost:11434")
	v.SetDefault("agent.name", "OpenLobster")
	v.SetDefault("agent.capabilities.browser", false)
	v.SetDefault("agent.capabilities.terminal", false)
	v.SetDefault("agent.capabilities.subagents", true)
	v.SetDefault("agent.capabilities.memory", true)
	v.SetDefault("agent.capabilities.mcp", true)
	v.SetDefault("agent.capabilities.filesystem", true)
	v.SetDefault("agent.capabilities.sessions", true)
	v.SetDefault("wizard.completed", false)
	return WriteEncryptedConfigFromViper(v, absPath)
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	setDefaults()

	// Env vars with OPENLOBSTER_ prefix override file config (e.g. OPENLOBSTER_DATABASE_DSN).
	viper.SetEnvPrefix("OPENLOBSTER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := bootstrapEncryptedConfig(path); err != nil {
		return nil, fmt.Errorf("bootstrap config: %w", err)
	}

	data, err := ReadConfigBytes(path)
	if err != nil {
		return nil, err
	}
	if err := viper.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, err
	}

	// Bind environment variables dynamically: first bind keys present in the
	// config (and defaults), then bind any OPENLOBSTER_* env vars found in
	// the process environment. This covers both file+env and env-only cases.
	bindEnvForAllKeys()
	bindEnvFromOS()

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func LoadFromEnv() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("OPENLOBSTER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Apply same defaults as Load() so env vars override them and nested keys exist.
	setDefaults()

	// Bind existing keys (from defaults) to their corresponding env vars, then
	// bind any OPENLOBSTER_* env vars present so Unmarshal can populate nested
	// keys from the environment when no config file is used. This mirrors Load().
	bindEnvForAllKeys()
	bindEnvFromOS()

	var cfg Config
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Agent: AgentConfig{
			Name:         "openlobster",
			SystemPrompt: "You are openlobster, an autonomous messaging agent.",
		},
		Scheduler: SchedulerConfig{
			Interval:       30 * time.Second,
			Enabled:        true,
			MemoryInterval: 4 * time.Hour,
			MemoryEnabled:  true,
		},
		Database: DatabaseConfig{
			Driver: "sqlite3",
			DSN:    "./data/openlobster.db",
		},
		GraphQL: GraphQLConfig{
			Enabled: true,
			Port:    8080,
			Host:    "127.0.0.1",
		},
		Memory: MemoryConfig{
			Backend: "file",
			File: MemoryFileConfig{
				Path: "./data/memory.gml",
			},
		},
		Providers: ProvidersConfig{
			OpenAI: OpenAIConfig{
				APIKey: "YOUR_API_KEY_HERE",
			},
			OpenRouter: OpenRouterConfig{
				APIKey:       "YOUR_API_KEY_HERE",
				DefaultModel: "openai/gpt-4o",
			},
			Ollama: OllamaConfig{
				Endpoint:     "http://localhost:11434",
				DefaultModel: "llama3",
			},
			Anthropic: AnthropicConfig{
				APIKey: "YOUR_API_KEY_HERE",
				Model:  "claude-sonnet-4-6",
			},
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				BotToken: "YOUR_BOT_TOKEN_HERE",
			},
			Discord: DiscordConfig{
				BotToken: "YOUR_BOT_TOKEN_HERE",
			},
			WhatsApp: WhatsAppConfig{
				PhoneID:  "",
				APIToken: "",
			},
			Twilio: TwilioConfig{
				AccountSID: "YOUR_ACCOUNT_SID",
				AuthToken:  "YOUR_AUTH_TOKEN",
				FromNumber: "+1234567890",
			},
		},
		SubAgents: SubAgentsConfig{
			MaxConcurrent:  3,
			DefaultTimeout: 5 * time.Minute,
		},
	}
}
