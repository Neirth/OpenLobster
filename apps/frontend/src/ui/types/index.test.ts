// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Tests for the shared domain types.
 *
 * Since these are TypeScript interfaces (no runtime values), the tests verify:
 *   1. That factory/helper functions build correctly-shaped objects.
 *   2. That type guard helpers correctly identify valid vs invalid shapes.
 *   3. Enumeration values are exhaustive at runtime (useful if the backend
 *      sends an unexpected status value).
 *
 * The main value here is documenting the expected shapes and catching
 * accidental breakage when the type file is edited.
 */

import { describe, it, expect } from 'vitest';
import type {
  Agent,
  Metrics,
  Channel,
  Conversation,
  Message,
  Task,
  McpServer,
  MemoryNode,
  MemoryEdge,
  MemoryGraph,
  Skill,
  AppConfig,
  ChannelConfig,
  ConnectionStatus,
  TaskStatus,
  MessageRole,
  AIProvider,
  McpTransport,
} from './index';

// ─── Runtime value sets ───────────────────────────────────────────────────────
// These arrays mirror the union types so we can test exhaustiveness at runtime.

const CONNECTION_STATUSES: ConnectionStatus[] = ['online', 'offline', 'degraded', 'unknown'];
const TASK_STATUSES: TaskStatus[] = ['pending', 'running', 'done', 'failed'];
const MESSAGE_ROLES: MessageRole[] = ['user', 'agent', 'assistant', 'system', 'tool', 'compaction'];
const AI_PROVIDERS: AIProvider[] = ['openai', 'openrouter', 'ollama'];
const MCP_TRANSPORTS: McpTransport[] = ['stdio', 'http', 'sse'];

// ─── Shape factories ──────────────────────────────────────────────────────────

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'agent-1',
    name: 'OpenLobster',
    version: '0.1.0',
    status: 'online',
    uptime: 3600,
    provider: 'openai',
    ...overrides,
  };
}

function makeMetrics(overrides: Partial<Metrics> = {}): Metrics {
  return {
    uptime: 3600,
    messagesReceived: 100,
    messagesSent: 80,
    activeSessions: 3,
    memoryNodes: 50,
    memoryEdges: 30,
    mcpTools: 12,
    tasksPending: 2,
    tasksRunning: 1,
    tasksDone: 15,
    errorsTotal: 0,
    ...overrides,
  };
}

function makeChannel(overrides: Partial<Channel> = {}): Channel {
  return {
    id: 'ch-1',
    name: 'discord',
    type: 'discord',
    status: 'online',
    messagesReceived: 200,
    messagesSent: 150,
    ...overrides,
  };
}

function makeConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: 'conv-1',
    channelId: 'ch-1',
    channelName: 'discord',
    participantId: 'user-42',
    participantName: 'Alice',
    lastMessageAt: '2024-01-01T12:00:00Z',
    unreadCount: 0,
    ...overrides,
  };
}

function makeMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg-1',
    conversationId: 'conv-1',
    role: 'user',
    content: 'Hello',
    createdAt: '2024-01-01T12:00:00Z',
    ...overrides,
  };
}

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: 'task-1',
    prompt: 'Summarize daily news',
    status: 'pending',
    isCyclic: true,
    createdAt: '2024-01-01T08:00:00Z',
    lastRunAt: null,
    nextRunAt: '2024-01-02T08:00:00Z',
    ...overrides,
  };
}

function makeMcpServer(overrides: Partial<McpServer> = {}): McpServer {
  return {
    name: 'filesystem',
    transport: 'stdio',
    status: 'online',
    toolCount: 8,
    ...overrides,
  };
}

function makeMemoryNode(overrides: Partial<MemoryNode> = {}): MemoryNode {
  return {
    id: 'node-1',
    label: 'OpenAI API Key',
    type: 'credential',
    value: 'sk-...',
    createdAt: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeMemoryEdge(overrides: Partial<MemoryEdge> = {}): MemoryEdge {
  return {
    id: 'edge-1',
    sourceId: 'node-1',
    targetId: 'node-2',
    relation: 'used_by',
    ...overrides,
  };
}

function makeSkill(overrides: Partial<Skill> = {}): Skill {
  return {
    name: 'computer-science',
    description: 'Software engineering guidance',
    enabled: true,
    path: '.claude/skills/computer-science',
    ...overrides,
  };
}

function makeAppConfig(overrides: Partial<AppConfig> = {}): AppConfig {
  return {
    agentName: 'OpenLobster',
    systemPrompt: 'You are a helpful agent.',
    provider: 'openai',
    channels: [],
    ...overrides,
  };
}

// ─── Enumeration tests ────────────────────────────────────────────────────────

describe('ConnectionStatus', () => {
  it('has exactly 4 values', () => {
    expect(CONNECTION_STATUSES).toHaveLength(4);
  });

  it('includes online, offline, degraded, unknown', () => {
    expect(CONNECTION_STATUSES).toContain('online');
    expect(CONNECTION_STATUSES).toContain('offline');
    expect(CONNECTION_STATUSES).toContain('degraded');
    expect(CONNECTION_STATUSES).toContain('unknown');
  });
});

describe('TaskStatus', () => {
  it('has exactly 4 values', () => {
    expect(TASK_STATUSES).toHaveLength(4);
  });

  it('includes all lifecycle states', () => {
    expect(TASK_STATUSES).toContain('pending');
    expect(TASK_STATUSES).toContain('running');
    expect(TASK_STATUSES).toContain('done');
    expect(TASK_STATUSES).toContain('failed');
  });
});

describe('MessageRole', () => {
  it('has exactly 6 values', () => {
    expect(MESSAGE_ROLES).toHaveLength(6);
  });

  it('includes user, agent, assistant, system, tool, compaction', () => {
    expect(MESSAGE_ROLES).toContain('user');
    expect(MESSAGE_ROLES).toContain('agent');
    expect(MESSAGE_ROLES).toContain('assistant');
    expect(MESSAGE_ROLES).toContain('system');
    expect(MESSAGE_ROLES).toContain('tool');
    expect(MESSAGE_ROLES).toContain('compaction');
  });
});

describe('AIProvider', () => {
  it('has exactly 3 supported providers', () => {
    expect(AI_PROVIDERS).toHaveLength(3);
  });

  it('includes openai, openrouter, ollama', () => {
    expect(AI_PROVIDERS).toContain('openai');
    expect(AI_PROVIDERS).toContain('openrouter');
    expect(AI_PROVIDERS).toContain('ollama');
  });
});

describe('McpTransport', () => {
  it('has exactly 3 transport types', () => {
    expect(MCP_TRANSPORTS).toHaveLength(3);
  });

  it('includes stdio, http, sse', () => {
    expect(MCP_TRANSPORTS).toContain('stdio');
    expect(MCP_TRANSPORTS).toContain('http');
    expect(MCP_TRANSPORTS).toContain('sse');
  });
});

// ─── Shape tests ──────────────────────────────────────────────────────────────

describe('Agent', () => {
  it('contains all required fields', () => {
    const agent = makeAgent();
    expect(agent.id).toBeTypeOf('string');
    expect(agent.name).toBeTypeOf('string');
    expect(agent.version).toBeTypeOf('string');
    expect(agent.uptime).toBeTypeOf('number');
    expect(CONNECTION_STATUSES).toContain(agent.status);
    expect(AI_PROVIDERS).toContain(agent.provider);
  });

  it('accepts all valid connection statuses', () => {
    for (const status of CONNECTION_STATUSES) {
      const agent = makeAgent({ status });
      expect(agent.status).toBe(status);
    }
  });
});

describe('Metrics', () => {
  it('contains all required numeric fields', () => {
    const metrics = makeMetrics();
    const numericFields: (keyof Metrics)[] = [
      'uptime', 'messagesReceived', 'messagesSent', 'activeSessions',
      'memoryNodes', 'memoryEdges', 'mcpTools', 'tasksPending',
      'tasksRunning', 'tasksDone', 'errorsTotal',
    ];
    for (const field of numericFields) {
      expect(metrics[field]).toBeTypeOf('number');
    }
  });

  it('all counts are non-negative', () => {
    const metrics = makeMetrics();
    for (const value of Object.values(metrics)) {
      expect(value as number).toBeGreaterThanOrEqual(0);
    }
  });
});

describe('Channel', () => {
  it('contains all required fields with correct types', () => {
    const channel = makeChannel();
    expect(channel.id).toBeTypeOf('string');
    expect(channel.name).toBeTypeOf('string');
    expect(channel.type).toBeTypeOf('string');
    expect(channel.messagesReceived).toBeTypeOf('number');
    expect(channel.messagesSent).toBeTypeOf('number');
    expect(CONNECTION_STATUSES).toContain(channel.status);
  });
});

describe('Conversation', () => {
  it('contains all required fields', () => {
    const conv = makeConversation();
    expect(conv.id).toBeTypeOf('string');
    expect(conv.channelId).toBeTypeOf('string');
    expect(conv.channelName).toBeTypeOf('string');
    expect(conv.participantId).toBeTypeOf('string');
    expect(conv.participantName).toBeTypeOf('string');
    expect(conv.lastMessageAt).toBeTypeOf('string');
    expect(conv.unreadCount).toBeTypeOf('number');
  });
});

describe('Message', () => {
  it('contains all required fields', () => {
    const msg = makeMessage();
    expect(msg.id).toBeTypeOf('string');
    expect(msg.conversationId).toBeTypeOf('string');
    expect(msg.content).toBeTypeOf('string');
    expect(msg.createdAt).toBeTypeOf('string');
    expect(MESSAGE_ROLES).toContain(msg.role);
  });

  it('accepts all valid message roles', () => {
    for (const role of MESSAGE_ROLES) {
      const msg = makeMessage({ role });
      expect(msg.role).toBe(role);
    }
  });
});

describe('Task', () => {
  it('contains all required fields', () => {
    const task = makeTask();
    expect(task.id).toBeTypeOf('string');
    expect(task.prompt).toBeTypeOf('string');
    expect(task.isCyclic).toBeTypeOf('boolean');
    expect(task.createdAt).toBeTypeOf('string');
    expect(TASK_STATUSES).toContain(task.status);
  });

  it('allows null for lastRunAt and nextRunAt', () => {
    const task = makeTask({ lastRunAt: null, nextRunAt: null });
    expect(task.lastRunAt).toBeNull();
    expect(task.nextRunAt).toBeNull();
  });

  it('accepts all valid task statuses', () => {
    for (const status of TASK_STATUSES) {
      const task = makeTask({ status });
      expect(task.status).toBe(status);
    }
  });
});

describe('McpServer', () => {
  it('contains all required fields', () => {
    const server = makeMcpServer();
    expect(server.name).toBeTypeOf('string');
    expect(server.toolCount).toBeTypeOf('number');
    expect(CONNECTION_STATUSES).toContain(server.status);
    expect(MCP_TRANSPORTS).toContain(server.transport);
  });

  it('accepts all transport types', () => {
    for (const transport of MCP_TRANSPORTS) {
      const server = makeMcpServer({ transport });
      expect(server.transport).toBe(transport);
    }
  });
});

describe('MemoryNode', () => {
  it('contains all required fields', () => {
    const node = makeMemoryNode();
    expect(node.id).toBeTypeOf('string');
    expect(node.label).toBeTypeOf('string');
    expect(node.type).toBeTypeOf('string');
    expect(node.value).toBeTypeOf('string');
    expect(node.createdAt).toBeTypeOf('string');
  });
});

describe('MemoryEdge', () => {
  it('contains all required fields', () => {
    const edge = makeMemoryEdge();
    expect(edge.id).toBeTypeOf('string');
    expect(edge.sourceId).toBeTypeOf('string');
    expect(edge.targetId).toBeTypeOf('string');
    expect(edge.relation).toBeTypeOf('string');
  });

  it('sourceId and targetId can differ', () => {
    const edge = makeMemoryEdge({ sourceId: 'a', targetId: 'b' });
    expect(edge.sourceId).not.toBe(edge.targetId);
  });
});

describe('MemoryGraph', () => {
  it('contains nodes and edges arrays', () => {
    const graph: MemoryGraph = {
      nodes: [makeMemoryNode()],
      edges: [makeMemoryEdge()],
    };
    expect(Array.isArray(graph.nodes)).toBe(true);
    expect(Array.isArray(graph.edges)).toBe(true);
  });

  it('can be empty', () => {
    const graph: MemoryGraph = { nodes: [], edges: [] };
    expect(graph.nodes).toHaveLength(0);
    expect(graph.edges).toHaveLength(0);
  });
});

describe('Skill', () => {
  it('contains all required fields', () => {
    const skill = makeSkill();
    expect(skill.name).toBeTypeOf('string');
    expect(skill.description).toBeTypeOf('string');
    expect(skill.enabled).toBeTypeOf('boolean');
    expect(skill.path).toBeTypeOf('string');
  });
});

describe('AppConfig', () => {
  it('contains all required fields', () => {
    const config = makeAppConfig();
    expect(config.agentName).toBeTypeOf('string');
    expect(config.systemPrompt).toBeTypeOf('string');
    expect(Array.isArray(config.channels)).toBe(true);
    expect(AI_PROVIDERS).toContain(config.provider);
  });

  it('channels array contains correctly shaped ChannelConfig items', () => {
    const channelConfig: ChannelConfig = {
      channelId: 'ch-1',
      channelName: 'discord',
      enabled: true,
    };
    const config = makeAppConfig({ channels: [channelConfig] });
    expect(config.channels[0].channelId).toBeTypeOf('string');
    expect(config.channels[0].channelName).toBeTypeOf('string');
    expect(config.channels[0].enabled).toBeTypeOf('boolean');
  });
});
