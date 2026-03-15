// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Mock hooks for testing and E2E preview.
 *
 * Drop-in replacement for @openlobster/ui/hooks — same exports, same
 * signatures (client argument accepted but ignored). Swap the import at
 * the bundler alias level to use these instead of the real GraphQL hooks.
 *
 * IMPORTANT: These mocks use the SAME types as @openlobster/ui to ensure
 * type compatibility. This forces the application code to work with the
 * real data shapes, making the mocks transparent to the type system.
 */

import type {
  Agent,
  Metrics,
  Channel,
  Conversation,
  Message,
  Task,
  McpServer,
  McpTool,
  MemoryGraph,
  Skill,
  AppConfig,
  SystemFile,
} from "@openlobster/ui/types";

export interface PairingRequestEvent {
  requestID: string;
  code: string;
  channelID: string;
  channelType: string;
  displayName: string;
  timestamp: string;
}

export interface SubscriptionEvent {
  type: string;
  timestamp: string;
  data: unknown;
}

const systemFiles: SystemFile[] = [
  {
    name: "AGENTS.md",
    path: "/etc/openlobster/AGENTS.md",
    content:
      "# Agent Configuration\n\nThis file defines the agent's behavior and capabilities.",
    lastModified: "2026-02-28T10:00:00Z",
  },
  {
    name: "SOUL.md",
    path: "/etc/openlobster/SOUL.md",
    content:
      "# Agent Soul\n\nThis file contains the agent's personality and values.",
    lastModified: "2026-02-28T10:00:00Z",
  },
];

const agent: Agent = {
  id: "agent-01",
  name: "agent-01",
  provider: "openrouter",
  version: "1.0.0",
  status: "online",
  uptime: 3600,
};

const metrics: Metrics = {
  uptime: 3600,
  messagesReceived: 2410,
  messagesSent: 2411,
  activeSessions: 3,
  memoryNodes: 128,
  memoryEdges: 32,
  mcpTools: 10,
  tasksPending: 1,
  tasksRunning: 2,
  tasksDone: 47,
  errorsTotal: 0,
};

const channels: Channel[] = [
  {
    id: "1",
    name: "Discord",
    type: "discord",
    status: "online",
    messagesReceived: 1200,
    messagesSent: 1100,
  },
  {
    id: "2",
    name: "Telegram",
    type: "telegram",
    status: "degraded",
    messagesReceived: 800,
    messagesSent: 750,
  },
  {
    id: "3",
    name: "WhatsApp",
    type: "whatsapp",
    status: "offline",
    messagesReceived: 410,
    messagesSent: 561,
  },
];

const conversations: Conversation[] = [
  {
    id: "1",
    channelId: "1",
    channelName: "Discord",
    participantId: "u1",
    participantName: "John",
    lastMessageAt: "2024-02-26T23:07:00.000Z",
    unreadCount: 2,
    isGroup: false,
  },
  {
    id: "2",
    channelId: "2",
    channelName: "Telegram",
    participantId: "u2",
    participantName: "Jane",
    lastMessageAt: "2024-02-26T23:06:00.000Z",
    unreadCount: 0,
    isGroup: false,
  },
  {
    id: "3",
    channelId: "1",
    channelName: "Discord",
    participantId: "g1",
    participantName: "Sergio",
    groupName: "Sergio, Josu y Horizon",
    lastMessageAt: "2024-02-26T22:00:00.000Z",
    unreadCount: 0,
    isGroup: true,
  },
];

const messages: Message[] = [
  {
    id: "m1",
    conversationId: "1",
    role: "user",
    content: "Hey there",
    createdAt: "2024-02-26T23:05:00.000Z",
  },
  {
    id: "m2",
    conversationId: "1",
    role: "agent",
    content: "Hello back",
    createdAt: "2024-02-26T23:06:00.000Z",
  },
];

const tasks: Task[] = [
  {
    id: "task1",
    prompt: "Morning brief",
    status: "running",
    schedule: "0 8 * * *",
    taskType: "cyclic",
    isCyclic: true,
    createdAt: "2026-01-01T00:00:00Z",
    lastRunAt: "2026-02-27T08:00:00Z",
    nextRunAt: "2026-02-28T08:00:00Z",
  },
  {
    id: "task2",
    prompt: "Health check",
    status: "running",
    schedule: "0 12 * * *",
    taskType: "cyclic",
    isCyclic: true,
    createdAt: "2026-01-01T00:00:00Z",
    lastRunAt: "2026-02-27T12:00:00Z",
    nextRunAt: null,
  },
  {
    id: "task3",
    prompt: "Memory cleanup",
    status: "pending",
    schedule: "",
    taskType: "one-shot",
    isCyclic: false,
    createdAt: "2026-02-20T00:00:00Z",
    lastRunAt: null,
    nextRunAt: "2026-03-02T03:00:00Z",
  },
];

const mcpServers: McpServer[] = [
  { name: "filesystem", transport: "stdio", status: "online", toolCount: 5, url: "http://localhost:8080" },
  { name: "github", transport: "http", status: "degraded", toolCount: 8, url: "https://api.github.com" },
  { name: "broken", transport: "http", status: "offline", toolCount: 0 },
];

const mcpTools: McpTool[] = [
  {
    name: "read_file",
    serverName: "filesystem",
    description: "Read a file from the filesystem",
  },
  {
    name: "write_file",
    serverName: "filesystem",
    description: "Write content to a file",
  },
  {
    name: "list_dir",
    serverName: "filesystem",
    description: "List directory contents",
  },
  {
    name: "delete_file",
    serverName: "filesystem",
    description: "Delete a file from the filesystem",
  },
  { name: "search", serverName: "web-search", description: "Search the web" },
  {
    name: "fetch_url",
    serverName: "web-search",
    description: "Fetch content from a URL",
  },
  {
    name: "get_issue",
    serverName: "github",
    description: "Get a GitHub issue",
  },
  {
    name: "create_issue",
    serverName: "github",
    description: "Create a GitHub issue",
  },
  {
    name: "store_memory",
    serverName: "memory",
    description: "Store a memory node",
  },
  {
    name: "query_memory",
    serverName: "memory",
    description: "Query the knowledge graph",
  },
];

const memory: MemoryGraph = {
  nodes: [
    {
      id: "n1",
      label: "John Doe",
      type: "person",
      value: "Test value",
      createdAt: "2026-01-10T09:00:00Z",
    },
    {
      id: "n2",
      label: "Jane Smith",
      type: "person",
      value: "",
      createdAt: "2026-01-12T11:00:00Z",
    },
  ],
  edges: [],
};

const skills: Skill[] = [
  {
    name: "computer-science",
    path: "",
    enabled: true,
    description: "Software engineering and CS expertise",
  },
  {
    name: "general-engineering",
    path: "",
    enabled: true,
    description: "Universal engineering principles",
  },
  {
    name: "graphics-design",
    path: "",
    enabled: true,
    description: "Visual design and UI/UX guidance",
  },
  {
    name: "web-search",
    path: "",
    enabled: false,
    description: "Search the web for up-to-date information",
  },
];

const config: AppConfig = {
  agentName: "agent-01",
  systemPrompt: "You are a helpful assistant.",
  provider: "openrouter",
  model: "claude-sonnet-4-5",
  memoryBackend: "neo4j",
  secretsBackend: "env_var",
  activeSessions: [
    {
      id: "s1",
      address: "192.168.1.48",
      status: "active",
      channel: "Discord",
      user: "John Doe",
    },
    {
      id: "s2",
      address: "10.0.8.12",
      status: "idle",
      channel: "Telegram",
      user: "Jane Smith",
    },
  ],
  channels: [
    { channelId: "1", channelName: "Discord", enabled: true },
    { channelId: "2", channelName: "Telegram", enabled: true },
    { channelId: "3", channelName: "WhatsApp", enabled: false },
  ],
};

function createMockQuery<T>(data: T) {
  return {
    data,
    isLoading: false,
    error: null,
    isSuccess: true,
    isError: false,
    isPending: false,
    refetch: () => Promise.resolve(),
  };
}

const mockLogs = `2026-03-01 13:48:00 [INFO] Agent started
2026-03-01 13:48:01 [INFO] Connected to Discord channel
2026-03-01 13:48:02 [INFO] Connected to Telegram channel
2026-03-01 13:48:05 [INFO] MCP server initialized
2026-03-01 13:48:10 [INFO] Heartbeat OK`;

export const useAgent = (_client?: unknown) =>
  createMockQuery<Agent | undefined>(agent);
export const useMetrics = (_client?: unknown) =>
  createMockQuery<Metrics | undefined>(metrics);
export const useLogs = (_client?: unknown) =>
  createMockQuery<string | undefined>(mockLogs);
export const useChannels = (_client?: unknown) =>
  createMockQuery<Channel[] | undefined>(channels);
export const useConversations = (_client?: unknown) =>
  createMockQuery<Conversation[] | undefined>(conversations);
export const useMessages = (_client?: unknown, _conversationId?: unknown) =>
  createMockQuery<Message[] | undefined>(messages);
export const useTasks = (_client?: unknown) =>
  createMockQuery<Task[] | undefined>(tasks);
export const useMcpServers = (_client?: unknown) =>
  createMockQuery<McpServer[] | undefined>(mcpServers);
export const useMcpTools = (_client?: unknown) =>
  createMockQuery<McpTool[] | undefined>(mcpTools);

const mcpUsers: Array<{ channelId: string; displayName: string; isAgent?: boolean }> = [
  { channelId: "1", displayName: "John", isAgent: false },
  { channelId: "2", displayName: "Jane", isAgent: false },
];
const toolPermissions: Array<{ toolName: string; mode: string }> = [];

export const useMcpUsers = (_client?: unknown) =>
  createMockQuery<typeof mcpUsers>(mcpUsers);
export const useToolPermissions = (_client?: unknown, _userId?: () => string) =>
  createMockQuery<typeof toolPermissions>(toolPermissions);
export const useMemory = (_client?: unknown) =>
  createMockQuery<MemoryGraph | undefined>(memory);
export const useSkills = (_client?: unknown) =>
  createMockQuery<Skill[] | undefined>(skills);
export const useConfig = (_client?: unknown) =>
  createMockQuery<AppConfig | undefined>(config);
export const useSystemFiles = (_client?: unknown) =>
  createMockQuery<SystemFile[] | undefined>(systemFiles);

export function useSubscriptions(_options?: unknown) {
  return {
    isConnected: () => false,
    error: () => null,
    connect: () => {},
    disconnect: () => {},
    sendResponse: () => {},
  };
}

export function createSubscriptionManager(_client?: unknown) {
  return {
    subscribe(_options?: unknown) {
      return useSubscriptions(_options);
    },
  };
}
