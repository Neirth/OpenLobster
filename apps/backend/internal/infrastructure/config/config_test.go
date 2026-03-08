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
	// Path under /nonexistent cannot be created → bootstrap fails
	_, err := Load("/nonexistent/path/config.yaml")
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
