// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * JSON Schema for OpenLobster configuration
 * Defines all configuration fields with types, validation, and dependencies
 */
export interface ConfigSchema {
  $schema: string;
  type: string;
  properties: Record<string, SchemaProperty>;
  required?: string[];
}

/** Condition shape used in JSON Schema dependency rules. */
export interface SchemaConditionNode {
  const?: unknown;
  properties?: Record<string, SchemaConditionNode>;
}

export interface SchemaCondition {
  properties?: Record<string, SchemaConditionNode>;
  oneOf?: SchemaCondition[];
}

export interface SchemaProperty {
  type: string;
  title: string;
  description: string;
  default?: unknown;
  properties?: Record<string, SchemaProperty>;
  enum?: string[];
  minLength?: number;
  maxLength?: number;
  minimum?: number;
  maximum?: number;
  pattern?: string;
  format?: string;
  placeholder?: string;
  dependencies?: Record<string, SchemaCondition>;
  oneOf?: SchemaCondition[];
  allOf?: SchemaCondition[];
  if?: SchemaCondition;
  then?: SchemaCondition;
  else?: SchemaCondition;
}

/**
 * Configuration schema with conditional rendering and validation rules
 */
export const configSchema: ConfigSchema = {
  $schema: "http://json-schema.org/draft-07/schema#",
  type: "object",
  properties: {
    // ========== GENERAL CONFIGURATION ==========
    agentName: {
      type: "string",
      title: "Agent Name",
      description: "Display name for this agent instance",
      default: "agent-01",
      minLength: 1,
      maxLength: 50,
    },
    reasoningLevel: {
      type: "string",
      title: "Reasoning Level",
      description: "Controls the depth of internal reasoning. Note: Higher levels may increase latency and token usage.",
      enum: ["none", "low", "medium", "high"],
      default: "medium",
    },

    // ========== AGENT CAPABILITIES ==========
    capabilities: {
      type: "object",
      title: "Agent Capabilities",
      description: "Enable or disable core agent features.",
      properties: {
        browser: { type: "boolean", title: "Browser", description: "Enable browser automation", default: false },
        terminal: { type: "boolean", title: "Terminal", description: "Enable terminal execution", default: false },
        subagents: { type: "boolean", title: "Subagents", description: "Enable spawning subagents", default: true },
        memory: { type: "boolean", title: "Memory", description: "Enable long-term memory", default: true },
        mcp: { type: "boolean", title: "MCP", description: "Enable MCP servers", default: true },
        filesystem: { type: "boolean", title: "Filesystem", description: "Enable filesystem access", default: true },
        sessions: { type: "boolean", title: "Sessions", description: "Enable session interaction", default: true },
      },
    },

    // ========== DATABASE CONFIGURATION ==========
    databaseDriver: {
      type: "string",
      title: "Database Driver",
      description: "Database driver to use",
      enum: ["sqlite", "postgres", "mysql"],
      default: "sqlite",
    },
    databaseDSN: {
      type: "string",
      title: "Database DSN",
      description: "Database connection string",
      default: "./data/openlobster.db",
    },
    databaseMaxOpenConns: {
      type: "integer",
      title: "Max Open Connections",
      description: "Maximum open database connections (0 = unlimited)",
      default: 0,
      minimum: 0,
    },
    databaseMaxIdleConns: {
      type: "integer",
      title: "Max Idle Connections",
      description: "Maximum idle database connections (0 = unlimited)",
      default: 0,
      minimum: 0,
    },

    // ========== SUBAGENTS CONFIGURATION ==========
    subagentsMaxConcurrent: {
      type: "integer",
      title: "Max Concurrent Subagents",
      description: "Maximum number of concurrent subagents",
      default: 3,
      minimum: 1,
      maximum: 10,
    },
    subagentsDefaultTimeout: {
      type: "string",
      title: "Default Timeout",
      description: "Default timeout for subagent tasks (e.g., 5m)",
      default: "5m",
      pattern: "^\\d+[smh]$",
    },

    // ========== GRAPHQL CONFIGURATION ==========
    graphqlEnabled: { type: "boolean", title: "GraphQL Enabled", description: "Enable GraphQL API", default: true },
    webEnabled: { type: "boolean", title: "Web Frontend Enabled", description: "Enable built-in web frontend", default: true },
    graphqlPort: { type: "integer", title: "GraphQL Port", description: "Port for GraphQL server", default: 8080, minimum: 1024, maximum: 65535 },
    graphqlHost: { type: "string", title: "GraphQL Host", description: "Host for GraphQL server", default: "127.0.0.1" },
    graphqlBaseUrl: { type: "string", title: "Server Base URL", description: "Public URL of the server", default: "" },
    a2aEnabled: { type: "boolean", title: "A2A Enabled", description: "Enable A2A protocol endpoints", default: false },

    // ========== LOGGING CONFIGURATION ==========
    loggingLevel: { type: "string", title: "Log Level", description: "Logging verbosity", enum: ["debug", "info", "warn", "error"], default: "info" },
    loggingPath: { type: "string", title: "Log Path", description: "Directory for log files", default: "./logs" },

    // ========== SCHEDULER CONFIGURATION ==========
    schedulerEnabled: { type: "boolean", title: "Scheduler Enabled", description: "Enable task scheduler", default: true },
    schedulerMemoryEnabled: { type: "boolean", title: "Memory Consolidation", description: "Enable periodic memory consolidation", default: true },
    schedulerMemoryInterval: { type: "string", title: "Consolidation Interval", description: "How often to run memory consolidation", default: "4h" },
  },
};

/**
 * Maps schema field keys to i18n keys. Used by SchemaField to render translated labels.
 */
export function getSchemaFieldI18nKey(field: string, forDescription = false): string {
  const base = `settings.field.${field.replace(/\./g, "_")}`;
  return forDescription ? `${base}Desc` : base;
}

/**
 * Group configuration for organizing settings in the UI
 */
export const configGroups = [
  {
    id: "general",
    title: "GENERAL CONFIGURATION",
    fields: ["agentName", "reasoningLevel"],
  },
  {
    id: "capabilities",
    title: "AGENT CAPABILITIES",
    fields: ["capabilities"],
  },
  {
    id: "database",
    title: "DATABASE CONFIGURATION",
    fields: ["databaseDriver", "databaseDSN", "databaseMaxOpenConns", "databaseMaxIdleConns"],
  },
  {
    id: "subagents",
    title: "SUBAGENTS CONFIGURATION",
    fields: ["subagentsMaxConcurrent", "subagentsDefaultTimeout"],
  },
  {
    id: "graphql",
    title: "GRAPHQL CONFIGURATION",
    fields: ["a2aEnabled", "webEnabled", "graphqlEnabled", "graphqlPort", "graphqlHost", "graphqlBaseUrl"],
  },
  {
    id: "logging",
    title: "LOGGING CONFIGURATION",
    fields: ["loggingLevel", "loggingPath"],
  },
  {
    id: "scheduler",
    title: "SCHEDULER CONFIGURATION",
    fields: ["schedulerEnabled", "schedulerMemoryEnabled", "schedulerMemoryInterval"],
  },
];
