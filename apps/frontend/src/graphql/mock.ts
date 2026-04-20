// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Mock GraphQL layer for development and visual auditing.
 * Intercepts fetch requests to GRAPHQL_ENDPOINT and returns pre-defined responses.
 */

const MOCK_CONFIG = {
  agent: {
    name: "OpenLobster",
    systemPrompt: "You are a helpful assistant.",
    provider: "ollama",
    model: "llama3.2:latest",
    apiKey: "",
    baseURL: "",
    ollamaHost: "http://localhost:11434",
    ollamaApiKey: "",
    anthropicApiKey: "",
    dockerModelRunnerEndpoint: "http://localhost:12434/engines/v1",
    dockerModelRunnerModel: "llama3.2",
    reasoningLevel: "medium",
  },
  capabilities: {
    browser: false,
    terminal: false,
    subagents: true,
    memory: true,
    mcp: true,
    filesystem: true,
    sessions: true,
  },
  database: {
    driver: "sqlite",
    dsn: "./data/openlobster.db",
    maxOpenConns: 0,
    maxIdleConns: 0,
  },
  memory: {
    backend: "file",
    filePath: "./data/memory.gml",
    neo4j: {
      uri: "",
      user: "",
      password: "",
    },
  },
  subagents: {
    maxConcurrent: 5,
    defaultTimeout: "300s",
  },
  graphql: {
    enabled: true,
    port: 8080,
    host: "127.0.0.1",
    baseUrl: "",
  },
  logging: {
    level: "info",
    path: "./logs",
  },
  secrets: {
    backend: "file",
    file: { path: "./data/secrets.json" },
    openbao: { url: "", token: "" },
  },
  scheduler: {
    enabled: true,
    memoryEnabled: true,
    memoryInterval: "4h",
  },
  activeSessions: [],
  channels: [
    { channelId: "telegram", channelName: "Telegram", enabled: false },
    { channelId: "discord", channelName: "Discord", enabled: false },
  ],
  channelSecrets: {
    telegramEnabled: false,
    telegramToken: "",
    discordEnabled: false,
    discordToken: "",
    whatsAppEnabled: false,
    whatsAppPhoneId: "",
    whatsAppApiToken: "",
    twilioEnabled: false,
    twilioAccountSid: "",
    twilioAuthToken: "",
    twilioFromNumber: "",
    slackEnabled: false,
    slackBotToken: "",
    slackAppToken: "",
  },
  pluginDefaults: {
    ai: "openlobster-ai-ollama",
    memory: "openlobster-memory-gml",
    secrets: "openlobster-secrets-json",
    audio: "",
  },
  a2aEnabled: true,
  webEnabled: true,
  wizardCompleted: true,
};

const MOCK_SYSTEM_FILES = [
  { name: "AGENTS.md", content: "# AGENTS.md\n\nConfiguration for multi-agent workflows." },
  { name: "SOUL.md", content: "# SOUL.md\n\nThe personality core of the agent." },
  { name: "IDENTITY.md", content: "# IDENTITY.md\n\nBasic identity and traits." },
  { name: "BOOTSTRAP.md", content: "# BOOTSTRAP.md\n\nInitial startup instructions." },
  { name: "MEMORY.md", content: "# MEMORY.md\n\nContextual memory guidelines." },
];

const MOCK_PLUGINS = [
  {
    id: "openlobster-ai-ollama",
    name: "Ollama AI",
    version: "1.0.0",
    description: "Built-in Ollama provider for local inference.",
    pluginType: "ai",
    schemaJson: JSON.stringify({
      properties: {
        host: { type: "string", title: "Ollama Host", default: "http://localhost:11434" },
        model: { type: "string", title: "Default Model", default: "llama3.2" }
      }
    }),
    configJson: JSON.stringify({ host: "http://localhost:11434", model: "llama3.2" }),
    enabled: true,
    available: true,
    lastError: null,
    builtin: true,
  },
  {
    id: "openlobster-memory-gml",
    name: "GML Memory",
    version: "1.0.0",
    description: "Graph Markup Language file-based memory plugin.",
    pluginType: "memory",
    schemaJson: JSON.stringify({
      properties: {
        path: { type: "string", title: "Memory Path", default: "./data/memory.gml" }
      }
    }),
    configJson: JSON.stringify({ path: "./data/memory.gml" }),
    enabled: true,
    available: true,
    lastError: null,
    builtin: true,
  },
  {
    id: "openlobster-secrets-json",
    name: "JSON Secrets",
    version: "1.0.0",
    description: "Simple encrypted JSON file secrets storage.",
    pluginType: "secrets",
    schemaJson: JSON.stringify({
      properties: {
        path: { type: "string", title: "Secrets Path", default: "./data/secrets.json" }
      }
    }),
    configJson: JSON.stringify({ path: "./data/secrets.json" }),
    enabled: true,
    available: true,
    lastError: null,
    builtin: true,
  }
];

export function setupMockGraphql() {
  const originalFetch = window.fetch;
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    
    if (url.includes("/graphql") && init?.method === "POST") {
      const body = JSON.parse(init.body as string);
      const query = (body.query || "").trim();
      const variables = body.variables || {};

      console.warn("[Mock GraphQL] Intercepted:", { query: query.substring(0, 100) + "...", variables });

      if (query.includes("query GetConfig")) {
        return new Response(JSON.stringify({ data: { config: MOCK_CONFIG } }), { status: 200 });
      }
      
      if (query.includes("query GetSystemFiles")) {
        return new Response(JSON.stringify({ data: { systemFiles: MOCK_SYSTEM_FILES } }), { status: 200 });
      }

      if (query.includes("query GetPlugins")) {
        return new Response(JSON.stringify({ data: { plugins: MOCK_PLUGINS } }), { status: 200 });
      }

      if (query.includes("mutation UpdateConfig")) {
        return new Response(JSON.stringify({ data: { updateConfig: { ...MOCK_CONFIG, ...variables.input } } }), { status: 200 });
      }

      if (query.includes("mutation WriteSystemFile")) {
        return new Response(JSON.stringify({ data: { writeSystemFile: { success: true } } }), { status: 200 });
      }

      if (query.includes("mutation UpdatePluginConfig")) {
        return new Response(JSON.stringify({ data: { updatePluginConfig: { success: true } } }), { status: 200 });
      }
    }
    
    return originalFetch(input, init);
  };
}
