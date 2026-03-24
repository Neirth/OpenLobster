// Copyright (c) OpenLobster contributors. See LICENSE for details.

// TestConfigRoundTrip exercises the full stack for every configuration field:
//
//	updateConfig mutation  →  ConfigUpdateAdapter.Apply  →  viper  →
//	BuildConfigSnapshot  →  AppConfigSnapshotToGenerated  →  config query
//
// A single test file is the authoritative gatekeeper: if a field is missing
// from any layer (GraphQL schema, InputToViperKeyMap, BuildConfigSnapshot,
// AppConfigSnapshotToGenerated), the corresponding assertion fails and the
// developer knows exactly which field and layer to fix.
package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-viper/mapstructure/v2"
	graphqlapp "github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func setupConfigIntegTest(t *testing.T) (handler http.Handler, snapshot *dto.AppConfigSnapshot) {
	t.Helper()
	t.Setenv("OPENLOBSTER_CONFIG_ENCRYPT", "0")
	viper.Reset()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(""), 0600))

	snapshot = &dto.AppConfigSnapshot{}

	adapter := &dto.ConfigUpdateAdapter{
		ConfigPath:    cfgPath,
		ReloadChannel: func(string) {},
		ViperKeys:     dto.InputToViperKeyMap(),
		OnApplied: func(_ bool) {
			var cfg config.Config
			if err := viper.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
				dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
					mapstructure.StringToTimeDurationHookFunc(),
					mapstructure.StringToSliceHookFunc(","),
				)
			}); err != nil {
				return
			}
			newSnap := dto.BuildConfigSnapshot(&cfg, func(c *config.Config) string {
				return c.Agent.Provider
			})
			*snapshot = *newSnap
		},
	}

	deps := graphqlapp.NewTestDeps(graphqlapp.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "IntegTest", Status: "ok"},
	})
	deps.ConfigSnapshot = snapshot
	deps.ConfigWriter = adapter

	handler = graphqlapp.NewGraphQLServer(deps)
	return handler, snapshot
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }

// sendUpdateConfig sends the updateConfig mutation with the given input and
// asserts no GraphQL errors are returned.
func sendUpdateConfig(t *testing.T, handler http.Handler, input map[string]interface{}) {
	t.Helper()
	type mutBody struct {
		Query     string      `json:"query"`
		Variables interface{} `json:"variables"`
	}
	body, err := json.Marshal(mutBody{
		Query: `mutation UpdateConfig($input: UpdateConfigInput!) {
			updateConfig(input: $input) { agentName }
		}`,
		Variables: map[string]interface{}{"input": input},
	})
	require.NoError(t, err)
	resp := gqlPost(t, handler, string(body))
	assert.Nil(t, resp["errors"], "mutation returned errors: %v", resp["errors"])
}

// queryConfig sends the config query and returns the config data map.
func queryConfig(t *testing.T, handler http.Handler) map[string]interface{} {
	t.Helper()
	const q = `{"query": "query { config { agent { name systemPrompt provider model apiKey baseURL ollamaHost ollamaApiKey anthropicApiKey dockerModelRunnerEndpoint dockerModelRunnerModel reasoningLevel } capabilities { browser terminal subagents memory mcp filesystem sessions } database { driver dsn maxOpenConns maxIdleConns } memory { backend filePath neo4j { uri user password } } subagents { maxConcurrent defaultTimeout } graphql { enabled port host baseUrl } logging { level path } secrets { backend file { path } openbao { url token } } scheduler { enabled memoryEnabled memoryInterval } channelSecrets { telegramEnabled telegramToken discordEnabled discordToken slackEnabled slackBotToken slackAppToken whatsAppEnabled whatsAppPhoneId whatsAppApiToken twilioEnabled twilioAccountSid twilioAuthToken twilioFromNumber } wizardCompleted } }"}`
	resp := gqlPost(t, handler, q)
	assert.Nil(t, resp["errors"], "config query returned errors: %v", resp["errors"])
	d := dataOf(t, resp)
	cfg, ok := d["config"].(map[string]interface{})
	require.True(t, ok, "config field missing from response")
	return cfg
}

func str(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func boolean(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func nested(m map[string]interface{}, key string) map[string]interface{} {
	v, _ := m[key].(map[string]interface{})
	return v
}

// ─── openai provider + all generic fields ────────────────────────────────────

func TestConfigRoundTrip_OpenAI(t *testing.T) {
	handler, _ := setupConfigIntegTest(t)

	sendUpdateConfig(t, handler, map[string]interface{}{
		// Agent — provider-specific (openai)
		"agentName":      "IntegBot",
		"provider":       "openai",
		"model":          "gpt-4o-mini",
		"apiKey":         "sk-openai-test",
		"baseURL":        "https://openai.example.com",
		"reasoningLevel": "high",

		// Capabilities (handled as nested map by Apply)
		"capabilities": map[string]interface{}{
			"browser":    true,
			"terminal":   true,
			"subagents":  false,
			"memory":     true,
			"mcp":        false,
			"filesystem": true,
			"sessions":   false,
		},

		// Database
		"databaseDriver":       "sqlite",
		"databaseDSN":          "./integ.db",
		"databaseMaxOpenConns": 5,
		"databaseMaxIdleConns": 2,

		// Memory
		"memoryBackend":       "neo4j",
		"memoryFilePath":      "./mem.gml",
		"memoryNeo4jURI":      "bolt://neo4j:7687",
		"memoryNeo4jUser":     "neo4j",
		"memoryNeo4jPassword": "neo4j-pass",

		// Subagents
		"subagentsMaxConcurrent":  4,
		"subagentsDefaultTimeout": "45s",

		// GraphQL
		"graphqlEnabled": true,
		"graphqlPort":    9090,
		"graphqlHost":    "0.0.0.0",
		"graphqlBaseUrl": "https://app.integ.test",

		// Logging
		"loggingLevel": "debug",
		"loggingPath":  "./integ.log",

		// Secrets
		"secretsBackend":      "openbao",
		"secretsFilePath":     "./integ-secrets.json",
		"secretsOpenbaoURL":   "https://vault.integ.test",
		"secretsOpenbaoToken": "hvs.integ",

		// Scheduler
		"schedulerEnabled":        true,
		"schedulerMemoryEnabled":  false,
		"schedulerMemoryInterval": "10m",

		// Channel — Telegram
		"channelTelegramEnabled": true,
		"channelTelegramToken":   "tg-integ-token",
		// Channel — Discord
		"channelDiscordEnabled": false,
		"channelDiscordToken":   "dc-integ-token",
		// Channel — Slack
		"channelSlackEnabled":  true,
		"channelSlackBotToken": "xoxb-integ",
		"channelSlackAppToken": "xapp-integ",
		// Channel — WhatsApp
		"channelWhatsAppEnabled":  true,
		"channelWhatsAppPhoneId":  "+34600000099",
		"channelWhatsAppApiToken": "wa-integ",
		// Channel — Twilio
		"channelTwilioEnabled":    false,
		"channelTwilioAccountSid": "AC-integ",
		"channelTwilioAuthToken":  "tw-integ",
		"channelTwilioFromNumber": "+15550099",

		// Wizard
		"wizardCompleted": true,
	})

	cfg := queryConfig(t, handler)
	agent := nested(cfg, "agent")
	caps := nested(cfg, "capabilities")
	db := nested(cfg, "database")
	mem := nested(cfg, "memory")
	neo4j := nested(mem, "neo4j")
	sub := nested(cfg, "subagents")
	gql := nested(cfg, "graphql")
	log := nested(cfg, "logging")
	sec := nested(cfg, "secrets")
	secFile := nested(sec, "file")
	secBao := nested(sec, "openbao")
	sched := nested(cfg, "scheduler")
	ch := nested(cfg, "channelSecrets")

	// ── agent ──
	assert.Equal(t, "IntegBot", str(agent, "name"), "agent.name")
	assert.Equal(t, "openai", str(agent, "provider"), "agent.provider")
	assert.Equal(t, "gpt-4o-mini", str(agent, "model"), "agent.model")
	assert.Equal(t, "sk-openai-test", str(agent, "apiKey"), "agent.apiKey")
	assert.Equal(t, "high", str(agent, "reasoningLevel"), "agent.reasoningLevel")

	// ── capabilities ──
	assert.True(t, boolean(caps, "browser"), "capabilities.browser")
	assert.True(t, boolean(caps, "terminal"), "capabilities.terminal")
	assert.False(t, boolean(caps, "subagents"), "capabilities.subagents")
	assert.True(t, boolean(caps, "memory"), "capabilities.memory")
	assert.False(t, boolean(caps, "mcp"), "capabilities.mcp")
	assert.True(t, boolean(caps, "filesystem"), "capabilities.filesystem")
	assert.False(t, boolean(caps, "sessions"), "capabilities.sessions")

	// ── database ──
	assert.Equal(t, "sqlite", str(db, "driver"), "database.driver")
	assert.Equal(t, "./integ.db", str(db, "dsn"), "database.dsn")
	// maxOpenConns and maxIdleConns come back as float64 from JSON
	assert.EqualValues(t, 5, db["maxOpenConns"], "database.maxOpenConns")
	assert.EqualValues(t, 2, db["maxIdleConns"], "database.maxIdleConns")

	// ── memory ──
	assert.Equal(t, "neo4j", str(mem, "backend"), "memory.backend")
	assert.Equal(t, "./mem.gml", str(mem, "filePath"), "memory.filePath")
	assert.Equal(t, "bolt://neo4j:7687", str(neo4j, "uri"), "memory.neo4j.uri")
	assert.Equal(t, "neo4j", str(neo4j, "user"), "memory.neo4j.user")
	assert.Equal(t, "neo4j-pass", str(neo4j, "password"), "memory.neo4j.password")

	// ── subagents ──
	assert.EqualValues(t, 4, sub["maxConcurrent"], "subagents.maxConcurrent")
	assert.NotEmpty(t, str(sub, "defaultTimeout"), "subagents.defaultTimeout must not be empty")

	// ── graphql ──
	assert.True(t, boolean(gql, "enabled"), "graphql.enabled")
	assert.EqualValues(t, 9090, gql["port"], "graphql.port")
	assert.Equal(t, "0.0.0.0", str(gql, "host"), "graphql.host")
	assert.Equal(t, "https://app.integ.test", str(gql, "baseUrl"), "graphql.baseUrl")

	// ── logging ──
	assert.Equal(t, "debug", str(log, "level"), "logging.level")
	assert.Equal(t, "./integ.log", str(log, "path"), "logging.path")

	// ── secrets ──
	assert.Equal(t, "openbao", str(sec, "backend"), "secrets.backend")
	assert.Equal(t, "./integ-secrets.json", str(secFile, "path"), "secrets.file.path")
	assert.Equal(t, "https://vault.integ.test", str(secBao, "url"), "secrets.openbao.url")
	assert.Equal(t, "hvs.integ", str(secBao, "token"), "secrets.openbao.token")

	// ── scheduler ──
	assert.True(t, boolean(sched, "enabled"), "scheduler.enabled")
	assert.False(t, boolean(sched, "memoryEnabled"), "scheduler.memoryEnabled")
	assert.NotEmpty(t, str(sched, "memoryInterval"), "scheduler.memoryInterval must not be empty")

	// ── channels ──
	assert.True(t, boolean(ch, "telegramEnabled"), "channelSecrets.telegramEnabled")
	assert.Equal(t, "tg-integ-token", str(ch, "telegramToken"), "channelSecrets.telegramToken")
	assert.False(t, boolean(ch, "discordEnabled"), "channelSecrets.discordEnabled")
	assert.Equal(t, "dc-integ-token", str(ch, "discordToken"), "channelSecrets.discordToken")
	assert.True(t, boolean(ch, "slackEnabled"), "channelSecrets.slackEnabled")
	assert.Equal(t, "xoxb-integ", str(ch, "slackBotToken"), "channelSecrets.slackBotToken")
	assert.Equal(t, "xapp-integ", str(ch, "slackAppToken"), "channelSecrets.slackAppToken")
	assert.True(t, boolean(ch, "whatsAppEnabled"), "channelSecrets.whatsAppEnabled")
	assert.Equal(t, "+34600000099", str(ch, "whatsAppPhoneId"), "channelSecrets.whatsAppPhoneId")
	assert.Equal(t, "wa-integ", str(ch, "whatsAppApiToken"), "channelSecrets.whatsAppApiToken")
	assert.False(t, boolean(ch, "twilioEnabled"), "channelSecrets.twilioEnabled")
	assert.Equal(t, "AC-integ", str(ch, "twilioAccountSid"), "channelSecrets.twilioAccountSid")
	assert.Equal(t, "tw-integ", str(ch, "twilioAuthToken"), "channelSecrets.twilioAuthToken")
	assert.Equal(t, "+15550099", str(ch, "twilioFromNumber"), "channelSecrets.twilioFromNumber")

	// ── wizard ──
	assert.True(t, boolean(cfg, "wizardCompleted"), "wizardCompleted")
}

// ─── anthropic provider ───────────────────────────────────────────────────────

func TestConfigRoundTrip_Anthropic(t *testing.T) {
	handler, _ := setupConfigIntegTest(t)

	sendUpdateConfig(t, handler, map[string]interface{}{
		"provider":        "anthropic",
		"model":           "claude-sonnet-4-6",
		"anthropicApiKey": "sk-ant-integ",
	})

	cfg := queryConfig(t, handler)
	agent := nested(cfg, "agent")

	assert.Equal(t, "anthropic", str(agent, "provider"), "agent.provider")
	assert.Equal(t, "claude-sonnet-4-6", str(agent, "model"), "agent.model")
	assert.Equal(t, "sk-ant-integ", str(agent, "anthropicApiKey"), "agent.anthropicApiKey")
}

// ─── ollama provider ──────────────────────────────────────────────────────────

func TestConfigRoundTrip_Ollama(t *testing.T) {
	handler, _ := setupConfigIntegTest(t)

	sendUpdateConfig(t, handler, map[string]interface{}{
		"provider":     "ollama",
		"model":        "llama3",
		"ollamaHost":   "http://ollama.integ:11434",
		"ollamaApiKey": "olk-integ",
	})

	cfg := queryConfig(t, handler)
	agent := nested(cfg, "agent")

	assert.Equal(t, "ollama", str(agent, "provider"), "agent.provider")
	assert.Equal(t, "llama3", str(agent, "model"), "agent.model")
	assert.Equal(t, "http://ollama.integ:11434", str(agent, "ollamaHost"), "agent.ollamaHost")
	assert.Equal(t, "olk-integ", str(agent, "ollamaApiKey"), "agent.ollamaApiKey")
}

// ─── docker-model-runner provider ────────────────────────────────────────────

func TestConfigRoundTrip_DockerModelRunner(t *testing.T) {
	handler, _ := setupConfigIntegTest(t)

	sendUpdateConfig(t, handler, map[string]interface{}{
		"provider":                  "docker-model-runner",
		"dockerModelRunnerEndpoint": "http://dmr.integ:12434",
		"model":                     "ai/mistral-v2",
	})

	cfg := queryConfig(t, handler)
	agent := nested(cfg, "agent")

	assert.Equal(t, "docker-model-runner", str(agent, "provider"), "agent.provider")
	assert.Equal(t, "http://dmr.integ:12434", str(agent, "dockerModelRunnerEndpoint"), "agent.dockerModelRunnerEndpoint")
	assert.Equal(t, "ai/mistral-v2", str(agent, "dockerModelRunnerModel"), "agent.dockerModelRunnerModel")
}
