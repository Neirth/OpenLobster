// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Tests for the shared GraphQL mutation strings.
 *
 * Same approach as the query tests: verify string shape, operation keyword,
 * variable declarations, and selected return fields. No network calls.
 */

import { describe, it, expect } from 'vitest';
import {
  SEND_MESSAGE_MUTATION,
  ADD_TASK_MUTATION,
  COMPLETE_TASK_MUTATION,
  REMOVE_TASK_MUTATION,
  CONNECT_MCP_MUTATION,
  DISCONNECT_MCP_MUTATION,
  ADD_MEMORY_NODE_MUTATION,
  ENABLE_SKILL_MUTATION,
  DISABLE_SKILL_MUTATION,
  UPDATE_CONFIG_MUTATION,
} from './index';

const isMutationString = (s: string) =>
  typeof s === 'string' && s.trim().length > 0;

const hasFields = (mutation: string, fields: string[]) =>
  fields.every((f) => mutation.includes(f));

describe('SEND_MESSAGE_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(SEND_MESSAGE_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(SEND_MESSAGE_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares required variables', () => {
    expect(SEND_MESSAGE_MUTATION).toContain('$conversationId');
    expect(SEND_MESSAGE_MUTATION).toContain('$content');
  });

  it('returns Message fields', () => {
    expect(hasFields(SEND_MESSAGE_MUTATION, ['id', 'conversationId', 'role', 'content', 'createdAt'])).toBe(true);
  });
});

describe('ADD_TASK_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(ADD_TASK_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(ADD_TASK_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $prompt and $schedule variables', () => {
    expect(ADD_TASK_MUTATION).toContain('$prompt');
    expect(ADD_TASK_MUTATION).toContain('$schedule');
  });

  it('returns Task fields', () => {
    const fields = ['id', 'prompt', 'status', 'isCyclic', 'createdAt', 'lastRunAt', 'nextRunAt'];
    expect(hasFields(ADD_TASK_MUTATION, fields)).toBe(true);
  });
});

describe('COMPLETE_TASK_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(COMPLETE_TASK_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(COMPLETE_TASK_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $taskId variable', () => {
    expect(COMPLETE_TASK_MUTATION).toContain('$taskId');
    expect(COMPLETE_TASK_MUTATION).toContain('String!');
  });
});

describe('REMOVE_TASK_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(REMOVE_TASK_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(REMOVE_TASK_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $taskId variable', () => {
    expect(REMOVE_TASK_MUTATION).toContain('$taskId');
  });
});

describe('CONNECT_MCP_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(CONNECT_MCP_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(CONNECT_MCP_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $name and $transport variables', () => {
    expect(CONNECT_MCP_MUTATION).toContain('$name');
    expect(CONNECT_MCP_MUTATION).toContain('$transport');
  });

  it('$url is required (has exclamation mark)', () => {
    expect(CONNECT_MCP_MUTATION).toMatch(/\$url:\s*String!/);
  });

  it('returns McpServer fields', () => {
    expect(hasFields(CONNECT_MCP_MUTATION, ['name', 'transport', 'status', 'toolCount'])).toBe(true);
  });
});

describe('DISCONNECT_MCP_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(DISCONNECT_MCP_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(DISCONNECT_MCP_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $name variable', () => {
    expect(DISCONNECT_MCP_MUTATION).toContain('$name');
  });
});

describe('ADD_MEMORY_NODE_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(ADD_MEMORY_NODE_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(ADD_MEMORY_NODE_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $label, $type, $value variables', () => {
    expect(ADD_MEMORY_NODE_MUTATION).toContain('$label');
    expect(ADD_MEMORY_NODE_MUTATION).toContain('$type');
    expect(ADD_MEMORY_NODE_MUTATION).toContain('$value');
  });

  it('returns MemoryNode fields', () => {
    expect(hasFields(ADD_MEMORY_NODE_MUTATION, ['id', 'label', 'type', 'value', 'createdAt'])).toBe(true);
  });
});

describe('ENABLE_SKILL_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(ENABLE_SKILL_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(ENABLE_SKILL_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $name variable', () => {
    expect(ENABLE_SKILL_MUTATION).toContain('$name');
  });
});

describe('DISABLE_SKILL_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(DISABLE_SKILL_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(DISABLE_SKILL_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $name variable', () => {
    expect(DISABLE_SKILL_MUTATION).toContain('$name');
  });

  it('is distinct from ENABLE_SKILL_MUTATION', () => {
    expect(DISABLE_SKILL_MUTATION).not.toBe(ENABLE_SKILL_MUTATION);
  });
});

describe('UPDATE_CONFIG_MUTATION', () => {
  it('is a non-empty string', () => {
    expect(isMutationString(UPDATE_CONFIG_MUTATION)).toBe(true);
  });

  it('is a mutation operation', () => {
    expect(UPDATE_CONFIG_MUTATION.trim()).toMatch(/^mutation /);
  });

  it('declares $input variable of type UpdateConfigInput!', () => {
    expect(UPDATE_CONFIG_MUTATION).toContain('$input');
    expect(UPDATE_CONFIG_MUTATION).toContain('UpdateConfigInput!');
  });

  it('returns AppConfig fields', () => {
    expect(hasFields(UPDATE_CONFIG_MUTATION, ['agentName', 'systemPrompt', 'provider'])).toBe(true);
  });

  it('returns nested ChannelConfig fields', () => {
    expect(hasFields(UPDATE_CONFIG_MUTATION, ['channels', 'channelId', 'channelName', 'enabled'])).toBe(true);
  });
});

describe('mutation string integrity', () => {
  const allMutations = [
    SEND_MESSAGE_MUTATION, ADD_TASK_MUTATION, COMPLETE_TASK_MUTATION,
    REMOVE_TASK_MUTATION, CONNECT_MCP_MUTATION, DISCONNECT_MCP_MUTATION,
    ADD_MEMORY_NODE_MUTATION, ENABLE_SKILL_MUTATION, DISABLE_SKILL_MUTATION,
    UPDATE_CONFIG_MUTATION,
  ];

  it('all mutation strings have balanced braces', () => {
    for (const mutation of allMutations) {
      const opens = (mutation.match(/\{/g) ?? []).length;
      const closes = (mutation.match(/\}/g) ?? []).length;
      expect(opens).toBe(closes);
    }
  });

  it('all mutation strings are unique', () => {
    const unique = new Set(allMutations);
    expect(unique.size).toBe(allMutations.length);
  });
});
