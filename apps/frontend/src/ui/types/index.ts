// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Shared domain types for the OpenLobster GraphQL API.
 *
 * Both the web frontend (apps/frontend) and the terminal frontend
 * (apps/terminal) consume the same GraphQL server. These TypeScript
 * interfaces are the single source of truth for the shape of that API.
 *
 * Keep these types in sync with the GraphQL schema in apps/backend.
 * Do not add presentation logic here — types only.
 */

// ─── Enumerations ────────────────────────────────────────────────────────────

/** Operational status of a persistent entity (channel, MCP server, agent). */
export type ConnectionStatus = "online" | "offline" | "degraded" | "unknown" | "unauthorized";

/** Lifecycle state of a scheduled task. */
export type TaskStatus = "pending" | "running" | "done" | "failed";

/** Role of a message sender within a conversation. */
export type MessageRole = "user" | "agent" | "assistant" | "system" | "tool" | "compaction";

/** Supported AI provider identifiers. */
export type AIProvider = "openai" | "openrouter" | "ollama";

// ─── Agent ───────────────────────────────────────────────────────────────────

/** Top-level agent status returned by the `agent` query. */
export interface Agent {
  id: string;
  name: string;
  version: string;
  status: ConnectionStatus;
  uptime: number;
  provider: AIProvider;
}

// ─── Metrics ─────────────────────────────────────────────────────────────────

/** Aggregate runtime metrics returned by the `metrics` query. */
export interface Metrics {
  uptime: number;
  messagesReceived: number;
  messagesSent: number;
  activeSessions: number;
  memoryNodes: number;
  memoryEdges: number;
  mcpTools: number;
  tasksPending: number;
  tasksRunning: number;
  tasksDone: number;
  errorsTotal: number;
}

// ─── Channels ────────────────────────────────────────────────────────────────

/** A messaging channel (Discord, Telegram, WhatsApp, Twilio, etc.). */
export interface Channel {
  id: string;
  name: string;
  type: string;
  status: ConnectionStatus;
  messagesReceived: number;
  messagesSent: number;
}

// ─── Conversations ───────────────────────────────────────────────────────────

/** A conversation thread within a channel. */
export interface Conversation {
  id: string;
  channelId: string;
  channelName: string;
  /** Optional human-readable group title for group conversations. */
  groupName?: string;
  isGroup: boolean;
  participantId: string;
  participantName: string;
  lastMessageAt: string;
  unreadCount: number;
}

/** An attachment included in a message (image, audio, document, etc.). */
export interface MessageAttachment {
  type: string;
  url?: string;
  filename?: string;
  mimeType?: string;
}

/** A single message within a conversation. */
export interface Message {
  id: string;
  conversationId: string;
  role: MessageRole;
  content: string;
  createdAt: string;
  attachments?: MessageAttachment[];
}

// ─── Tasks (Cron) ────────────────────────────────────────────────────────────

/** A scheduled or one-off task managed by the cron subsystem. */
export interface Task {
  id: string;
  prompt: string;
  status: TaskStatus;
  /** Cron expression, ISO 8601 datetime, or empty string (immediate). */
  schedule: string;
  /** "one-shot" | "cyclic" — derived from schedule, stored in DB. */
  taskType: 'one-shot' | 'cyclic';
  /** Derived from taskType: true when taskType === 'cyclic'. */
  isCyclic: boolean;
  /** Whether the scheduler will execute this task. */
  enabled: boolean;
  createdAt: string;
  lastRunAt: string | null;
  nextRunAt: string | null;
}

// ─── MCP Servers ─────────────────────────────────────────────────────────────

/** Transport type of an MCP server connection. Only Streamable HTTP is supported. */
export type McpTransport = "http";

/** A connected or configured MCP (Model Context Protocol) server. */
export interface McpServer {
  name: string;
  transport: McpTransport;
  status: ConnectionStatus;
  toolCount: number;
  /** Endpoint URL of the server; used by the UI to derive the favicon origin. */
  url?: string;
}

/** A single tool exposed by an MCP server. */
export interface McpTool {
  name: string;
  serverName: string;
  description: string;
}

// ─── Memory ──────────────────────────────────────────────────────────────────

/** A node in the agent memory graph. */
export interface MemoryNode {
  id: string;
  label: string;
  type: string;
  value: string;
  createdAt: string;
  properties?: Record<string, string>;
}

/** A directed edge between two memory nodes. */
export interface MemoryEdge {
  id: string;
  sourceId: string;
  targetId: string;
  relation: string;
}

/** The full memory graph returned by the `memory` query. */
export interface MemoryGraph {
  nodes: MemoryNode[];
  edges: MemoryEdge[];
}

// ─── Skills ──────────────────────────────────────────────────────────────────

/** A Claude skill available to the agent. */
export interface Skill {
  name: string;
  description: string;
  enabled: boolean;
  path: string;
}

// ─── Configuration ───────────────────────────────────────────────────────────

/** Active session connected to the agent. */
export interface ActiveSession {
  id: string;
  address: string;
  status: string;
  channel: string;
  user: string;
}

/** Agent configuration returned by the `config` query. */
export interface AppConfig {
  agentName?: string;
  systemPrompt?: string;
  provider?: AIProvider;
  model?: string;
  memoryBackend?: string;
  secretsBackend?: string;
  agent?: {
    name: string;
    systemPrompt: string;
    provider: AIProvider | string;
    model: string;
    apiKey: string;
    baseURL: string;
    ollamaHost: string;
  };
  capabilities?: {
    browser: boolean;
    terminal: boolean;
    subagents: boolean;
    memory: boolean;
    mcp: boolean;
    audio: boolean;
    filesystem: boolean;
    sessions: boolean;
  };
  database?: {
    driver: string;
    dsn: string;
    maxOpenConns: number;
    maxIdleConns: number;
  };
  memory?: {
    backend: string;
    filePath: string;
    neo4j?: {
      uri: string;
      user: string;
      password: string;
    };
    postgres?: {
      dsn: string;
    };
  };
  subagents?: {
    maxConcurrent: number;
    defaultTimeout: string;
  };
  graphql?: {
    enabled: boolean;
    port: number;
    host: string;
  };
  logging?: {
    level: string;
    path: string;
  };
  secrets?: {
    backend: string;
    file?: {
      path: string;
    };
    openbao?: {
      url: string;
      token: string;
    };
  };
  scheduler?: {
    enabled: boolean;
    memoryEnabled: boolean;
    memoryInterval: string;
  };
  activeSessions: ActiveSession[];
  channels: ChannelConfig[];
}

/** Per-channel enable/disable toggle within the agent config. */
export interface ChannelConfig {
  channelId: string;
  channelName: string;
  enabled: boolean;
}

// ─── System Files ───────────────────────────────────────────────────────────

/** System files that configure agent behavior. */
export interface SystemFile {
  name: string;
  path: string;
  content: string;
  lastModified: string;
}

// ─── Tool Permissions ────────────────────────────────────────────────────────

/** Permission mode for a specific tool. Absence of a row implies 'deny'. */
export type ToolPermissionMode = 'allow' | 'deny';

/** An explicit permission entry for one user + tool combination. */
export interface ToolPermission {
  toolName: string;
  mode: ToolPermissionMode;
}

/** A user (conversation participant) that appears in the permission manager. */
export interface McpUser {
  channelId: string;
  displayName: string;
  /** True for the reserved loopback/agent system user. */
  isAgent?: boolean;
}
