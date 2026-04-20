// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Tests for the shared headless data hooks.
 *
 * Strategy: hooks are thin wrappers around createQuery. We verify:
 *   1. The correct queryKey is used.
 *   2. queryFn calls the right GraphQL operation with the right variables.
 *   3. refetchInterval and staleTime are set to the documented values.
 *   4. enabled is false when a required argument is empty (useMessages).
 *
 * createQuery itself is not instantiated (it requires a QueryClientProvider).
 * Instead, we extract the options factory passed to createQuery and test it directly.
 */

import { describe, it, expect, vi } from 'vitest';
import type { GraphQLClient } from 'graphql-request';
import {
  AGENT_QUERY,
  CHANNELS_QUERY,
  TASKS_QUERY,
  MCP_SERVERS_QUERY,
  MCP_TOOLS_QUERY,
  CONVERSATIONS_QUERY,
  MESSAGES_QUERY,
  MEMORY_QUERY,
  SKILLS_QUERY,
  CONFIG_QUERY,
} from '../graphql/queries/index';

// ─── Mock @tanstack/solid-query ───────────────────────────────────────────────
// Capture the options object passed to createQuery without actually running it.
// Each createQuery call stores its factory result so we can inspect it.

type QueryOptions = Record<string, unknown>;
let capturedOptions: QueryOptions | null = null;

vi.mock('@tanstack/solid-query', () => ({
  createQuery: (optionsFn: () => QueryOptions) => {
    capturedOptions = optionsFn();
    return capturedOptions;
  },
}));

// ─── Helpers ──────────────────────────────────────────────────────────────────

function makeClient(response: unknown): GraphQLClient {
  return {
    request: vi.fn().mockResolvedValue(response),
  } as unknown as GraphQLClient;
}

function getOptions(): QueryOptions {
  if (!capturedOptions) throw new Error('createQuery was not called');
  return capturedOptions;
}

// Reset captured options before each test
beforeEach(() => {
  capturedOptions = null;
});

// ─── useAgent ─────────────────────────────────────────────────────────────────

describe('useAgent', () => {
  it('uses [agent] as queryKey', async () => {
    const { useAgent } = await import('./useAgent');
    useAgent(makeClient({ agent: {} }));
    expect(getOptions().queryKey).toEqual(['agent']);
  });

  it('queryFn calls AGENT_QUERY', async () => {
    const { useAgent } = await import('./useAgent');
    const client = makeClient({ agent: { id: '1', name: 'bot' } });
    useAgent(client);
    const opts = getOptions();
    await (opts.queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(AGENT_QUERY);
  });

  it('polls every 5 seconds', async () => {
    const { useAgent } = await import('./useAgent');
    useAgent(makeClient({ agent: {} }));
    expect(getOptions().refetchInterval).toBe(5_000);
  });

  it('staleTime is 4 seconds', async () => {
    const { useAgent } = await import('./useAgent');
    useAgent(makeClient({ agent: {} }));
    expect(getOptions().staleTime).toBe(4_000);
  });
});

/** Build a minimal GraphQLClient mock that also exposes the url property. */
function makeMetricsClient(): GraphQLClient {
  return { request: vi.fn(), url: 'http://localhost:8080/graphql' } as unknown as GraphQLClient;
}

// ─── useMetrics ───────────────────────────────────────────────────────────────

describe('useMetrics', () => {
  it('uses [metrics] as queryKey', async () => {
    const { useMetrics } = await import('./useMetrics');
    useMetrics(makeMetricsClient());
    expect(getOptions().queryKey).toEqual(['metrics']);
  });

  it('queryFn fetches /metrics endpoint via fetch (not GraphQL)', async () => {
    const { useMetrics } = await import('./useMetrics');
    const prometheusText = [
      '# HELP openlobster_uptime_seconds Agent uptime in seconds.',
      '# TYPE openlobster_uptime_seconds gauge',
      'openlobster_uptime_seconds 42',
      '# HELP openlobster_messages_received_total Total messages received.',
      '# TYPE openlobster_messages_received_total counter',
      'openlobster_messages_received_total 5',
      '# HELP openlobster_messages_sent_total Total messages sent.',
      '# TYPE openlobster_messages_sent_total counter',
      'openlobster_messages_sent_total 3',
      '# HELP openlobster_active_sessions Active sessions.',
      '# TYPE openlobster_active_sessions gauge',
      'openlobster_active_sessions 2',
      '# HELP openlobster_memory_nodes Memory nodes.',
      '# TYPE openlobster_memory_nodes gauge',
      'openlobster_memory_nodes 10',
      '# HELP openlobster_memory_edges Memory edges.',
      '# TYPE openlobster_memory_edges gauge',
      'openlobster_memory_edges 20',
      '# HELP openlobster_mcp_tools MCP tools.',
      '# TYPE openlobster_mcp_tools gauge',
      'openlobster_mcp_tools 4',
      '# HELP openlobster_tasks_pending Tasks pending.',
      '# TYPE openlobster_tasks_pending gauge',
      'openlobster_tasks_pending 1',
      '# HELP openlobster_tasks_running Tasks running.',
      '# TYPE openlobster_tasks_running gauge',
      'openlobster_tasks_running 0',
      '# HELP openlobster_tasks_done_total Tasks done.',
      '# TYPE openlobster_tasks_done_total gauge',
      'openlobster_tasks_done_total 7',
      '# HELP openlobster_errors_total Errors.',
      '# TYPE openlobster_errors_total counter',
      'openlobster_errors_total 0',
    ].join('\n');

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(prometheusText),
    }));

    const client = makeMetricsClient();
    useMetrics(client);
    const result = await (getOptions().queryFn as () => Promise<unknown>)() as Record<string, number>;

    expect(result.uptime).toBe(42);
    expect(result.messagesReceived).toBe(5);
    expect(result.messagesSent).toBe(3);
    expect(result.activeSessions).toBe(2);
    expect(result.memoryNodes).toBe(10);
    expect(result.memoryEdges).toBe(20);
    expect(result.mcpTools).toBe(4);
    expect(result.tasksPending).toBe(1);
    expect(result.tasksDone).toBe(7);
    expect(result.errorsTotal).toBe(0);
    // GraphQL client.request should NOT be called
    expect((client.request as ReturnType<typeof vi.fn>).mock.calls.length).toBe(0);

    vi.unstubAllGlobals();
  });

  it('polls every 5 seconds when visible', async () => {
    const { useMetrics } = await import('./useMetrics');
    const orig = typeof document !== 'undefined' && Object.getOwnPropertyDescriptor(document, 'visibilityState');
    if (typeof document !== 'undefined') {
      Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
    }
    useMetrics(makeMetricsClient());
    const ri = getOptions().refetchInterval;
    const result = typeof ri === 'function' ? (ri as (q: unknown) => number)({}) : ri;
    expect(result).toBe(5_000);
    if (orig) Object.defineProperty(document, 'visibilityState', orig);
  });
});

// ─── useChannels ──────────────────────────────────────────────────────────────

describe('useChannels', () => {
  it('uses [channels] as queryKey', async () => {
    const { useChannels } = await import('./useChannels');
    useChannels(makeClient({ channels: [] }));
    expect(getOptions().queryKey).toEqual(['channels']);
  });

  it('queryFn calls CHANNELS_QUERY', async () => {
    const { useChannels } = await import('./useChannels');
    const client = makeClient({ channels: [] });
    useChannels(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(CHANNELS_QUERY);
  });

  it('polls every 5 seconds', async () => {
    const { useChannels } = await import('./useChannels');
    useChannels(makeClient({ channels: [] }));
    expect(getOptions().refetchInterval).toBe(5_000);
  });
});

// ─── useTasks ─────────────────────────────────────────────────────────────────

describe('useTasks', () => {
  it('uses [tasks] as queryKey', async () => {
    const { useTasks } = await import('./useTasks');
    useTasks(makeClient({ tasks: [] }));
    expect(getOptions().queryKey).toEqual(['tasks']);
  });

  it('queryFn calls TASKS_QUERY', async () => {
    const { useTasks } = await import('./useTasks');
    const client = makeClient({ tasks: [] });
    useTasks(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(TASKS_QUERY);
  });

  it('polls every 3 seconds', async () => {
    const { useTasks } = await import('./useTasks');
    useTasks(makeClient({ tasks: [] }));
    expect(getOptions().refetchInterval).toBe(3_000);
  });
});

// ─── useMcpServers ────────────────────────────────────────────────────────────

describe('useMcpServers', () => {
  it('uses [mcpServers] as queryKey', async () => {
    const { useMcpServers } = await import('./useMcps');
    useMcpServers(makeClient({ mcpServers: [] }));
    expect(getOptions().queryKey).toEqual(['mcpServers']);
  });

  it('queryFn calls MCP_SERVERS_QUERY', async () => {
    const { useMcpServers } = await import('./useMcps');
    const client = makeClient({ mcpServers: [] });
    useMcpServers(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(MCP_SERVERS_QUERY);
  });

  it('polls every 5 seconds', async () => {
    const { useMcpServers } = await import('./useMcps');
    useMcpServers(makeClient({ mcpServers: [] }));
    expect(getOptions().refetchInterval).toBe(5_000);
  });
});

// ─── useMcpTools ──────────────────────────────────────────────────────────────

describe('useMcpTools', () => {
  it('uses [mcpTools] as queryKey', async () => {
    const { useMcpTools } = await import('./useMcps');
    useMcpTools(makeClient({ mcpTools: [] }));
    expect(getOptions().queryKey).toEqual(['mcpTools']);
  });

  it('queryFn calls MCP_TOOLS_QUERY', async () => {
    const { useMcpTools } = await import('./useMcps');
    const client = makeClient({ mcpTools: [] });
    useMcpTools(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(MCP_TOOLS_QUERY);
  });
});

// ─── useConversations ─────────────────────────────────────────────────────────

describe('useConversations', () => {
  it('uses [conversations] as queryKey', async () => {
    const { useConversations } = await import('./useConversations');
    useConversations(makeClient({ conversations: [] }));
    expect(getOptions().queryKey).toEqual(['conversations']);
  });

  it('queryFn calls CONVERSATIONS_QUERY', async () => {
    const { useConversations } = await import('./useConversations');
    const client = makeClient({ conversations: [] });
    useConversations(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(CONVERSATIONS_QUERY);
  });
});

// ─── useMessages ──────────────────────────────────────────────────────────────

describe('useMessages', () => {
  it('uses [messages, conversationId] as queryKey', async () => {
    const { useMessages } = await import('./useMessages');
    useMessages(makeClient({ messages: [] }), () => 'conv-1');
    expect(getOptions().queryKey).toEqual(['messages', 'conv-1']);
  });

  it('queryFn calls MESSAGES_QUERY with the conversationId variable', async () => {
    const { useMessages } = await import('./useMessages');
    const client = makeClient({ messages: [] });
    useMessages(client, () => 'conv-42');
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(MESSAGES_QUERY, { conversationId: 'conv-42' });
  });

  it('is enabled when conversationId is non-empty', async () => {
    const { useMessages } = await import('./useMessages');
    useMessages(makeClient({ messages: [] }), () => 'conv-1');
    expect(getOptions().enabled).toBe(true);
  });

  it('is disabled when conversationId is empty', async () => {
    const { useMessages } = await import('./useMessages');
    useMessages(makeClient({ messages: [] }), () => '');
    expect(getOptions().enabled).toBe(false);
  });
});

// ─── useMemory ────────────────────────────────────────────────────────────────

describe('useMemory', () => {
  it('uses [memory] as queryKey', async () => {
    const { useMemory } = await import('./useMemory');
    useMemory(makeClient({ memory: { nodes: [], edges: [] } }));
    expect(getOptions().queryKey).toEqual(['memory']);
  });

  it('queryFn calls MEMORY_QUERY', async () => {
    const { useMemory } = await import('./useMemory');
    const client = makeClient({ memory: { nodes: [], edges: [] } });
    useMemory(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(MEMORY_QUERY);
  });

  it('polls every 10 seconds', async () => {
    const { useMemory } = await import('./useMemory');
    useMemory(makeClient({ memory: { nodes: [], edges: [] } }));
    expect(getOptions().refetchInterval).toBe(10_000);
  });
});

// ─── useSkills ────────────────────────────────────────────────────────────────

describe('useSkills', () => {
  it('uses [skills] as queryKey', async () => {
    const { useSkills } = await import('./useSkills');
    useSkills(makeClient({ skills: [] }));
    expect(getOptions().queryKey).toEqual(['skills']);
  });

  it('queryFn calls SKILLS_QUERY', async () => {
    const { useSkills } = await import('./useSkills');
    const client = makeClient({ skills: [] });
    useSkills(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(SKILLS_QUERY);
  });

  it('polls every 30 seconds', async () => {
    const { useSkills } = await import('./useSkills');
    useSkills(makeClient({ skills: [] }));
    expect(getOptions().refetchInterval).toBe(30_000);
  });
});

// ─── useConfig ────────────────────────────────────────────────────────────────

describe('useConfig', () => {
  it('uses [config] as queryKey', async () => {
    const { useConfig } = await import('./useConfig');
    useConfig(makeClient({ config: {} }));
    expect(getOptions().queryKey).toEqual(['config']);
  });

  it('queryFn calls CONFIG_QUERY', async () => {
    const { useConfig } = await import('./useConfig');
    const client = makeClient({ config: { agentName: 'bot' } });
    useConfig(client);
    await (getOptions().queryFn as () => Promise<unknown>)();
    expect(client.request).toHaveBeenCalledWith(CONFIG_QUERY);
  });

  it('polls every 30 seconds', async () => {
    const { useConfig } = await import('./useConfig');
    useConfig(makeClient({ config: {} }));
    expect(getOptions().refetchInterval).toBe(30_000);
  });
});

// ─── Barrel integrity ─────────────────────────────────────────────────────────

describe('hooks barrel (index.ts)', () => {
  it('exports all 11 hooks', async () => {
    const hooks = await import('./index');
    const expectedExports = [
      'useAgent', 'useMetrics', 'useChannels', 'useTasks',
      'useMcpServers', 'useMcpTools', 'useConversations',
      'useMessages', 'useMemory', 'useSkills', 'useConfig',
    ];
    for (const name of expectedExports) {
      expect(typeof (hooks as Record<string, unknown>)[name]).toBe('function');
    }
  });
});
