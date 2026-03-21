package config

import (
	"os"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/assert"
)

func TestProvidersConfig(t *testing.T) {
	content := `
providers:
  openrouter:
    api_key: "key"
    default_model: "model"
  ollama:
    endpoint: "http://localhost"
    default_model: "llama"
  opencode:
    api_key: "oc-key"
  openai:
    api_key: "oa-key"
`

	tmpFile := t.TempDir() + "/config.yaml"
	createTestFile(t, tmpFile, content)

	cfg, err := Load(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, "key", cfg.Providers.OpenRouter.APIKey)
	assert.Equal(t, "model", cfg.Providers.OpenRouter.DefaultModel)
	assert.Equal(t, "http://localhost", cfg.Providers.Ollama.Endpoint)
	assert.Equal(t, "llama", cfg.Providers.Ollama.DefaultModel)
	assert.Equal(t, "oc-key", cfg.Providers.OpenCode.APIKey)
	assert.Equal(t, "oa-key", cfg.Providers.OpenAI.APIKey)
}

func TestChannelsConfig(t *testing.T) {
	content := `
channels:
  telegram:
    bot_token: "tg-token"
  discord:
    bot_token: "dc-token"
  whatsapp:
    phone_id: "phone123"
  twilio:
    account_sid: "ACxxx"
    auth_token: "auth"
    from_number: "+1234"
    webhook_path: "/webhook"
`

	tmpFile := t.TempDir() + "/config.yaml"
	createTestFile(t, tmpFile, content)

	cfg, err := Load(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, "tg-token", cfg.Channels.Telegram.BotToken)
	assert.Equal(t, "dc-token", cfg.Channels.Discord.BotToken)
	assert.Equal(t, "phone123", cfg.Channels.WhatsApp.PhoneID)
	assert.Equal(t, "ACxxx", cfg.Channels.Twilio.AccountSID)
	assert.Equal(t, "auth", cfg.Channels.Twilio.AuthToken)
	assert.Equal(t, "+1234", cfg.Channels.Twilio.FromNumber)
}

func TestMCPConfig(t *testing.T) {
	content := `
mcp:
  servers:
    - name: "fs"
      type: "stdio"
      command: "npx"
      args: ["-y", "server"]
      env: ["VAR=val"]
    - name: "http"
      type: "http"
      url: "http://server"
`

	tmpFile := t.TempDir() + "/config.yaml"
	createTestFile(t, tmpFile, content)

	cfg, err := Load(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.MCP.Servers, 2)

	assert.Equal(t, "fs", cfg.MCP.Servers[0].Name)
	assert.Equal(t, "stdio", cfg.MCP.Servers[0].Type)
	assert.Equal(t, "npx", cfg.MCP.Servers[0].Command)
	assert.Equal(t, []string{"-y", "server"}, cfg.MCP.Servers[0].Args)
	assert.Equal(t, []string{"VAR=val"}, cfg.MCP.Servers[0].Env)

	assert.Equal(t, "http", cfg.MCP.Servers[1].Name)
	assert.Equal(t, "http", cfg.MCP.Servers[1].Type)
	assert.Equal(t, "http://server", cfg.MCP.Servers[1].URL)
}

func TestMemoryConfig_File(t *testing.T) {
	content := `
memory:
  backend: "file"
  file:
    path: "./memory/data"
`

	tmpFile := t.TempDir() + "/config.yaml"
	createTestFile(t, tmpFile, content)

	cfg, err := Load(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, models.MemoryFile, cfg.Memory.Backend)
	assert.Equal(t, "./memory/data", cfg.Memory.File.Path)
}

func TestMemoryConfig_Neo4j(t *testing.T) {
	content := `
memory:
  backend: "neo4j"
  neo4j:
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "pass"
`

	tmpFile := t.TempDir() + "/config.yaml"
	createTestFile(t, tmpFile, content)

	cfg, err := Load(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, models.MemoryNeo4j, cfg.Memory.Backend)
	assert.Equal(t, "bolt://localhost:7687", cfg.Memory.Neo4j.URI)
}

func TestSubAgentsConfig(t *testing.T) {
	content := `
subagents:
  max_concurrent: 10
  default_timeout: "15m"
`

	tmpFile := t.TempDir() + "/config.yaml"
	createTestFile(t, tmpFile, content)

	cfg, err := Load(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, 10, cfg.SubAgents.MaxConcurrent)
	assert.Equal(t, 15*60*time.Second, cfg.SubAgents.DefaultTimeout)
}

func createTestFile(t *testing.T, path, content string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = f.WriteString(content)
	if err != nil {
		t.Fatal(err, "failed to write test config file")
	}
}

func TestLoadFromEnv(t *testing.T) {
	_, err := LoadFromEnv()
	assert.NoError(t, err)
}

func TestLoadFromEnv_Neo4j(t *testing.T) {
	// Set env vars as Kubernetes would provide them via EnvPrefix + replacer
	os.Setenv("OPENLOBSTER_MEMORY_NEO4J_URI", "bolt://env-host:7687")
	os.Setenv("OPENLOBSTER_MEMORY_NEO4J_USER", "envuser")
	os.Setenv("OPENLOBSTER_MEMORY_NEO4J_PASSWORD", "envpass")
	defer func() {
		os.Unsetenv("OPENLOBSTER_MEMORY_NEO4J_URI")
		os.Unsetenv("OPENLOBSTER_MEMORY_NEO4J_USER")
		os.Unsetenv("OPENLOBSTER_MEMORY_NEO4J_PASSWORD")
	}()

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)
	// Unmarshal should populate the nested Neo4j fields from env via BindEnv
	assert.Equal(t, "bolt://env-host:7687", cfg.Memory.Neo4j.URI)
	assert.Equal(t, "envuser", cfg.Memory.Neo4j.User)
	assert.Equal(t, "envpass", cfg.Memory.Neo4j.Password)
}

// envMappingCase defines one env-var → Config field check. SetValue is the string
// written to the environment; GetValue reads the corresponding field from Config;
// Expected is the value that must be present after LoadFromEnv.
type envMappingCase struct {
	EnvVar    string
	SetValue  string
	GetValue  func(*Config) interface{}
	Expected  interface{}
}

func TestLoadFromEnv_AllKeysMapped(t *testing.T) {
	cases := []envMappingCase{
		{"OPENLOBSTER_AGENT_NAME", "env-agent", func(c *Config) interface{} { return c.Agent.Name }, "env-agent"},
		{"OPENLOBSTER_AGENT_CAPABILITIES_BROWSER", "true", func(c *Config) interface{} { return c.Agent.Capabilities.Browser }, true},
		{"OPENLOBSTER_AGENT_CAPABILITIES_TERMINAL", "true", func(c *Config) interface{} { return c.Agent.Capabilities.Terminal }, true},
		{"OPENLOBSTER_AGENT_CAPABILITIES_SUBAGENTS", "true", func(c *Config) interface{} { return c.Agent.Capabilities.Subagents }, true},
		{"OPENLOBSTER_AGENT_CAPABILITIES_MEMORY", "true", func(c *Config) interface{} { return c.Agent.Capabilities.Memory }, true},
		{"OPENLOBSTER_AGENT_CAPABILITIES_MCP", "true", func(c *Config) interface{} { return c.Agent.Capabilities.MCP }, true},
		{"OPENLOBSTER_AGENT_CAPABILITIES_FILESYSTEM", "true", func(c *Config) interface{} { return c.Agent.Capabilities.Filesystem }, true},
		{"OPENLOBSTER_AGENT_CAPABILITIES_SESSIONS", "true", func(c *Config) interface{} { return c.Agent.Capabilities.Sessions }, true},
		{"OPENLOBSTER_SCHEDULER_INTERVAL", "10s", func(c *Config) interface{} { return c.Scheduler.Interval }, 10 * time.Second},
		{"OPENLOBSTER_SCHEDULER_ENABLED", "true", func(c *Config) interface{} { return c.Scheduler.Enabled }, true},
		{"OPENLOBSTER_SCHEDULER_MEMORY_INTERVAL", "1h", func(c *Config) interface{} { return c.Scheduler.MemoryInterval }, 1 * time.Hour},
		{"OPENLOBSTER_SCHEDULER_MEMORY_ENABLED", "true", func(c *Config) interface{} { return c.Scheduler.MemoryEnabled }, true},
		{"OPENLOBSTER_DATABASE_DRIVER", "postgres", func(c *Config) interface{} { return c.Database.Driver }, "postgres"},
		{"OPENLOBSTER_DATABASE_DSN", "postgres://user:pass@host/db", func(c *Config) interface{} { return c.Database.DSN }, "postgres://user:pass@host/db"},
		{"OPENLOBSTER_DATABASE_MAX_OPEN_CONNS", "11", func(c *Config) interface{} { return c.Database.MaxOpenConns }, 11},
		{"OPENLOBSTER_DATABASE_MAX_IDLE_CONNS", "7", func(c *Config) interface{} { return c.Database.MaxIdleConns }, 7},
		{"OPENLOBSTER_MEMORY_BACKEND", "file", func(c *Config) interface{} { return c.Memory.Backend }, models.MemoryFile},
		{"OPENLOBSTER_MEMORY_FILE_PATH", "/tmp/memory.gml", func(c *Config) interface{} { return c.Memory.File.Path }, "/tmp/memory.gml"},
		{"OPENLOBSTER_MEMORY_NEO4J_URI", "bolt://env-host:7687", func(c *Config) interface{} { return c.Memory.Neo4j.URI }, "bolt://env-host:7687"},
		{"OPENLOBSTER_MEMORY_NEO4J_USER", "envuser-all", func(c *Config) interface{} { return c.Memory.Neo4j.User }, "envuser-all"},
		{"OPENLOBSTER_MEMORY_NEO4J_PASSWORD", "envpass-all", func(c *Config) interface{} { return c.Memory.Neo4j.Password }, "envpass-all"},
		{"OPENLOBSTER_MEMORY_POSTGRES_DSN", "postgres://mem:pass@host/memdb", func(c *Config) interface{} { return c.Memory.Postgres.DSN }, "postgres://mem:pass@host/memdb"},
		{"OPENLOBSTER_PROVIDERS_OPENROUTER_API_KEY", "or-key", func(c *Config) interface{} { return c.Providers.OpenRouter.APIKey }, "or-key"},
		{"OPENLOBSTER_PROVIDERS_OPENROUTER_DEFAULT_MODEL", "or-model", func(c *Config) interface{} { return c.Providers.OpenRouter.DefaultModel }, "or-model"},
		{"OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT", "http://ollama-host", func(c *Config) interface{} { return c.Providers.Ollama.Endpoint }, "http://ollama-host"},
		{"OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL", "llama3-env", func(c *Config) interface{} { return c.Providers.Ollama.DefaultModel }, "llama3-env"},
		{"OPENLOBSTER_PROVIDERS_OPENCODE_API_KEY", "oc-key-env", func(c *Config) interface{} { return c.Providers.OpenCode.APIKey }, "oc-key-env"},
		{"OPENLOBSTER_PROVIDERS_OPENCODE_MODEL", "oc-model-env", func(c *Config) interface{} { return c.Providers.OpenCode.Model }, "oc-model-env"},
		{"OPENLOBSTER_PROVIDERS_OPENAI_API_KEY", "oa-key-env", func(c *Config) interface{} { return c.Providers.OpenAI.APIKey }, "oa-key-env"},
		{"OPENLOBSTER_PROVIDERS_OPENAI_MODEL", "gpt-env", func(c *Config) interface{} { return c.Providers.OpenAI.Model }, "gpt-env"},
		{"OPENLOBSTER_PROVIDERS_OPENAICOMPAT_MODEL", "oac-model-env", func(c *Config) interface{} { return c.Providers.OpenAICompat.Model }, "oac-model-env"},
		{"OPENLOBSTER_PROVIDERS_ANTHROPIC_MODEL", "claude-env", func(c *Config) interface{} { return c.Providers.Anthropic.Model }, "claude-env"},
		{"OPENLOBSTER_CHANNELS_TELEGRAM_ENABLED", "true", func(c *Config) interface{} { return c.Channels.Telegram.Enabled }, true},
		{"OPENLOBSTER_CHANNELS_TELEGRAM_BOT_TOKEN", "tg-env", func(c *Config) interface{} { return c.Channels.Telegram.BotToken }, "tg-env"},
		{"OPENLOBSTER_CHANNELS_DISCORD_ENABLED", "true", func(c *Config) interface{} { return c.Channels.Discord.Enabled }, true},
		{"OPENLOBSTER_CHANNELS_DISCORD_BOT_TOKEN", "dc-env", func(c *Config) interface{} { return c.Channels.Discord.BotToken }, "dc-env"},
		{"OPENLOBSTER_CHANNELS_WHATSAPP_ENABLED", "true", func(c *Config) interface{} { return c.Channels.WhatsApp.Enabled }, true},
		{"OPENLOBSTER_CHANNELS_WHATSAPP_PHONE_ID", "wa-phone-env", func(c *Config) interface{} { return c.Channels.WhatsApp.PhoneID }, "wa-phone-env"},
		{"OPENLOBSTER_CHANNELS_TWILIO_ENABLED", "true", func(c *Config) interface{} { return c.Channels.Twilio.Enabled }, true},
		{"OPENLOBSTER_CHANNELS_TWILIO_ACCOUNT_SID", "ACenv", func(c *Config) interface{} { return c.Channels.Twilio.AccountSID }, "ACenv"},
		{"OPENLOBSTER_CHANNELS_TWILIO_AUTH_TOKEN", "tw-auth-env", func(c *Config) interface{} { return c.Channels.Twilio.AuthToken }, "tw-auth-env"},
		{"OPENLOBSTER_CHANNELS_TWILIO_FROM_NUMBER", "+9999", func(c *Config) interface{} { return c.Channels.Twilio.FromNumber }, "+9999"},
		{"OPENLOBSTER_CHANNELS_TWILIO_WEBHOOK_PATH", "/twilio-env", func(c *Config) interface{} { return c.Channels.Twilio.WebhookPath }, "/twilio-env"},
		{"OPENLOBSTER_CHANNELS_SLACK_ENABLED", "true", func(c *Config) interface{} { return c.Channels.Slack.Enabled }, true},
		{"OPENLOBSTER_GRAPHQL_ENABLED", "true", func(c *Config) interface{} { return c.GraphQL.Enabled }, true},
		{"OPENLOBSTER_GRAPHQL_PORT", "9090", func(c *Config) interface{} { return c.GraphQL.Port }, 9090},
		{"OPENLOBSTER_GRAPHQL_HOST", "127.0.0.2", func(c *Config) interface{} { return c.GraphQL.Host }, "127.0.0.2"},
		{"OPENLOBSTER_GRAPHQL_BASE_URL", "https://graphql-env.local", func(c *Config) interface{} { return c.GraphQL.BaseURL }, "https://graphql-env.local"},
		{"OPENLOBSTER_GRAPHQL_AUTH_ENABLED", "true", func(c *Config) interface{} { return c.GraphQL.AuthEnabled }, true},
		{"OPENLOBSTER_GRAPHQL_AUTH_TOKEN", "auth-env-token", func(c *Config) interface{} { return c.GraphQL.AuthToken }, "auth-env-token"},
		{"OPENLOBSTER_LOGGING_LEVEL", "debug", func(c *Config) interface{} { return c.Logging.Level }, "debug"},
		{"OPENLOBSTER_LOGGING_PATH", "/var/log/env", func(c *Config) interface{} { return c.Logging.Path }, "/var/log/env"},
		{"OPENLOBSTER_PERMISSIONS_DEFAULT_MODE", "always", func(c *Config) interface{} { return c.Permissions.DefaultMode }, "always"},
		{"OPENLOBSTER_SECRETS_BACKEND", "file", func(c *Config) interface{} { return c.Secrets.Backend }, "file"},
		{"OPENLOBSTER_SECRETS_FILE_PATH", "/tmp/secrets-env.json", func(c *Config) interface{} { return c.Secrets.File.Path }, "/tmp/secrets-env.json"},
		{"OPENLOBSTER_WORKSPACE_PATH", "/tmp/workspace-env", func(c *Config) interface{} { return c.Workspace.Path }, "/tmp/workspace-env"},
		{"OPENLOBSTER_WIZARD_COMPLETED", "true", func(c *Config) interface{} { return c.Wizard.Completed }, true},
	}
	casesOpenbao := []envMappingCase{
		{"OPENLOBSTER_SECRETS_OPENBAO_URL", "https://openbao-env.local", func(c *Config) interface{} {
			if c.Secrets.Openbao == nil {
				return ""
			}
			return c.Secrets.Openbao.URL
		}, "https://openbao-env.local"},
		{"OPENLOBSTER_SECRETS_OPENBAO_TOKEN", "openbao-token-env", func(c *Config) interface{} {
			if c.Secrets.Openbao == nil {
				return ""
			}
			return c.Secrets.Openbao.Token
		}, "openbao-token-env"},
	}
	cases = append(cases, casesOpenbao...)

	for _, tc := range cases {
		os.Setenv(tc.EnvVar, tc.SetValue)
	}
	defer func() {
		for _, tc := range cases {
			os.Unsetenv(tc.EnvVar)
		}
	}()

	cfg, err := LoadFromEnv()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	for _, tc := range cases {
		got := tc.GetValue(cfg)
		assert.Equal(t, tc.Expected, got, "env %s → config field", tc.EnvVar)
	}
}

// ─── Validate ────────────────────────────────────────────────────────────────

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./data/db"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: "./mem"}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestValidate_MissingDatabaseDriver(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: "./mem"}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	valErr, ok := err.(*ValidationError)
	assert.True(t, ok)
	assert.Contains(t, valErr.Errors[0], "database.driver")
}

func TestValidate_InvalidDatabaseDriver(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "mongodb"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: "./mem"}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database.driver")
}

func TestValidate_MissingMemoryBackend(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: ""},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory.backend")
}

func TestValidate_InvalidMemoryBackend(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: "redis"},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory.backend")
}

func TestValidate_FileMemoryMissingPath(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: ""}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory.file.path")
}

func TestValidate_Neo4jMemoryMissingFields(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: models.MemoryNeo4j, Neo4j: MemoryNeo4jConfig{}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory.neo4j")
}

func TestValidate_InvalidGraphQLPort(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:   GraphQLConfig{Port: 0},
		Memory:    MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: "./mem"}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "graphql.port")
}

func TestValidate_NoAIProvider(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:  GraphQLConfig{Port: 8080},
		Memory:   MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: "./mem"}},
		Providers: ProvidersConfig{
			OpenAI:     OpenAIConfig{APIKey: "YOUR_API_KEY_HERE"},
			OpenRouter: OpenRouterConfig{APIKey: "YOUR_API_KEY_HERE"},
			Ollama:     OllamaConfig{Endpoint: ""},
			Anthropic:  AnthropicConfig{APIKey: "YOUR_API_KEY_HERE"},
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI provider")
}

func TestValidate_SchedulerIntervalWhenEnabled(t *testing.T) {
	cfg := &Config{
		Database:  DatabaseConfig{Driver: "sqlite3", DSN: "./db"},
		GraphQL:   GraphQLConfig{Port: 8080},
		Memory:    MemoryConfig{Backend: models.MemoryFile, File: MemoryFileConfig{Path: "./mem"}},
		Providers: ProvidersConfig{OpenAI: OpenAIConfig{APIKey: "sk-xxx"}},
		Scheduler: SchedulerConfig{Enabled: true, Interval: 0},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheduler.interval")
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Errors: []string{"err1", "err2"}}
	s := err.Error()
	assert.Contains(t, s, "err1")
	assert.Contains(t, s, "err2")
	assert.Contains(t, s, "invalid")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "openlobster", cfg.Agent.Name)
	assert.Equal(t, "sqlite3", cfg.Database.Driver)
	assert.Equal(t, 8080, cfg.GraphQL.Port)
	assert.Equal(t, models.MemoryFile, cfg.Memory.Backend)
	assert.Equal(t, "YOUR_API_KEY_HERE", cfg.Providers.OpenAI.APIKey)
}

func TestLoad_FileNotFound(t *testing.T) {
	// /etc/hosts is a regular file, so using it as a directory fails even as root.
	_, err := Load("/etc/hosts/subdir/config.yaml")
	assert.Error(t, err)
}

func TestLoad_BootstrapCreatesEncryptedConfig(t *testing.T) {
	os.Unsetenv("OPENLOBSTER_CONFIG_ENCRYPT")
	defer os.Unsetenv("OPENLOBSTER_CONFIG_ENCRYPT")
	path := t.TempDir() + "/newconfig.yaml"
	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.True(t, len(data) >= 6 && string(data[:6]) == "OLENC1",
		"bootstrap must create encrypted config when default, got plain/corrupt")
}

func TestLoad_BootstrapCreatesPlainConfig_WhenEncryptDisabled(t *testing.T) {
	os.Setenv("OPENLOBSTER_CONFIG_ENCRYPT", "0")
	defer os.Unsetenv("OPENLOBSTER_CONFIG_ENCRYPT")
	path := t.TempDir() + "/newconfig_plain.yaml"
	cfg, err := Load(path)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.False(t, len(data) >= 6 && string(data[:6]) == "OLENC1",
		"must not be encrypted when OPENLOBSTER_CONFIG_ENCRYPT=0")
	assert.Contains(t, string(data), "database", "plain YAML should have default keys")
}
