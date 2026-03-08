// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Tests for the shared GraphQL query strings.
 *
 * Each test verifies:
 *   1. The export is a non-empty string.
 *   2. It starts with the correct GraphQL operation keyword.
 *   3. It selects the fields documented in the shared types.
 *   4. Variable declarations match the documented arguments.
 *
 * These tests do NOT execute network requests. They guard against
 * accidental edits that truncate or corrupt the query strings.
 */

import { describe, it, expect } from 'vitest';
import {
  AGENT_QUERY,
  METRICS_QUERY,
  CHANNELS_QUERY,
  CONVERSATIONS_QUERY,
  MESSAGES_QUERY,
  TASKS_QUERY,
  MCP_SERVERS_QUERY,
  MCP_TOOLS_QUERY,
  MEMORY_QUERY,
  SKILLS_QUERY,
  CONFIG_QUERY,
} from './index';

// Helper: assert a string is a non-empty query/mutation document
const isQueryString = (s: string) =>
  typeof s === 'string' && s.trim().length > 0;

// Helper: check that all field names are present in the string
const hasFields = (query: string, fields: string[]) =>
  fields.every((f) => query.includes(f));

describe('AGENT_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(AGENT_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(AGENT_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all Agent fields', () => {
    expect(hasFields(AGENT_QUERY, ['id', 'name', 'version', 'status', 'uptime', 'provider'])).toBe(true);
  });
});

describe('METRICS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(METRICS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(METRICS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all Metrics fields', () => {
    const fields = [
      'uptime', 'messagesReceived', 'messagesSent', 'activeSessions',
      'memoryNodes', 'memoryEdges', 'mcpTools', 'tasksPending',
      'tasksRunning', 'tasksDone', 'errorsTotal',
    ];
    expect(hasFields(METRICS_QUERY, fields)).toBe(true);
  });
});

describe('CHANNELS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(CHANNELS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(CHANNELS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all Channel fields', () => {
    expect(hasFields(CHANNELS_QUERY, ['id', 'name', 'type', 'status', 'messagesReceived', 'messagesSent'])).toBe(true);
  });
});

describe('CONVERSATIONS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(CONVERSATIONS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(CONVERSATIONS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all Conversation fields', () => {
    const fields = ['id', 'channelId', 'channelName', 'participantId', 'participantName', 'lastMessageAt', 'unreadCount'];
    expect(hasFields(CONVERSATIONS_QUERY, fields)).toBe(true);
  });
});

describe('MESSAGES_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(MESSAGES_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(MESSAGES_QUERY.trim()).toMatch(/^query /);
  });

  it('declares the $conversationId variable', () => {
    expect(MESSAGES_QUERY).toContain('$conversationId');
    expect(MESSAGES_QUERY).toContain('String!');
  });

  it('selects all Message fields', () => {
    expect(hasFields(MESSAGES_QUERY, ['id', 'conversationId', 'role', 'content', 'createdAt'])).toBe(true);
  });
});

describe('TASKS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(TASKS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(TASKS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all Task fields', () => {
    const fields = ['id', 'prompt', 'status', 'isCyclic', 'createdAt', 'lastRunAt', 'nextRunAt'];
    expect(hasFields(TASKS_QUERY, fields)).toBe(true);
  });
});

describe('MCP_SERVERS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(MCP_SERVERS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(MCP_SERVERS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all McpServer fields', () => {
    expect(hasFields(MCP_SERVERS_QUERY, ['name', 'transport', 'status', 'toolCount'])).toBe(true);
  });
});

describe('MCP_TOOLS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(MCP_TOOLS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(MCP_TOOLS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all McpTool fields', () => {
    expect(hasFields(MCP_TOOLS_QUERY, ['name', 'serverName', 'description'])).toBe(true);
  });
});

describe('MEMORY_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(MEMORY_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(MEMORY_QUERY.trim()).toMatch(/^query /);
  });

  it('selects node fields', () => {
    expect(hasFields(MEMORY_QUERY, ['nodes', 'id', 'label', 'type', 'value', 'createdAt'])).toBe(true);
  });

  it('selects edge fields', () => {
    expect(hasFields(MEMORY_QUERY, ['edges', 'sourceId', 'targetId', 'relation'])).toBe(true);
  });
});

describe('SKILLS_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(SKILLS_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(SKILLS_QUERY.trim()).toMatch(/^query /);
  });

  it('selects all Skill fields', () => {
    expect(hasFields(SKILLS_QUERY, ['name', 'description', 'enabled', 'path'])).toBe(true);
  });
});

describe('CONFIG_QUERY', () => {
  it('is a non-empty string', () => {
    expect(isQueryString(CONFIG_QUERY)).toBe(true);
  });

  it('is a query operation', () => {
    expect(CONFIG_QUERY.trim()).toMatch(/^query /);
  });

  it('selects nested AppConfig fields', () => {
    expect(hasFields(CONFIG_QUERY, ['agent', 'name', 'systemPrompt', 'provider'])).toBe(true);
    expect(hasFields(CONFIG_QUERY, ['scheduler', 'enabled', 'memoryInterval'])).toBe(true);
    expect(hasFields(CONFIG_QUERY, ['database', 'driver', 'dsn'])).toBe(true);
    expect(hasFields(CONFIG_QUERY, ['memory', 'backend', 'neo4j'])).toBe(true);
  });

  it('selects nested ChannelConfig fields', () => {
    expect(hasFields(CONFIG_QUERY, ['channels', 'channelId', 'channelName', 'enabled'])).toBe(true);
  });
});

describe('query string integrity', () => {
  const allQueries = [
    AGENT_QUERY, METRICS_QUERY, CHANNELS_QUERY, CONVERSATIONS_QUERY,
    MESSAGES_QUERY, TASKS_QUERY, MCP_SERVERS_QUERY, MCP_TOOLS_QUERY,
    MEMORY_QUERY, SKILLS_QUERY, CONFIG_QUERY,
  ];

  it('all query strings have balanced braces', () => {
    for (const query of allQueries) {
      const opens = (query.match(/\{/g) ?? []).length;
      const closes = (query.match(/\}/g) ?? []).length;
      expect(opens).toBe(closes);
    }
  });

  it('all query strings are unique', () => {
    const unique = new Set(allQueries);
    expect(unique.size).toBe(allQueries.length);
  });
});
