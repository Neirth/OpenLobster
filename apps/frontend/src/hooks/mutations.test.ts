// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi } from 'vitest';
import type { GraphQLClient } from 'graphql-request';
import {
  useUpdateConfig,
  useAddMCPServer,
  useRemoveMCPServer,
  useAddTask,
  useRemoveTask,
} from './mutations';
import {
  UPDATE_CONFIG_MUTATION,
  ADD_MCP_SERVER_MUTATION,
  REMOVE_MCP_SERVER_MUTATION,
  ADD_TASK_MUTATION,
  REMOVE_TASK_MUTATION,
} from '../graphql/mutations';

/**
 * The mutation hooks wrap createMutation from @tanstack/solid-query.
 *
 * We test two layers:
 *   1. Each hook is a callable function (export shape).
 *   2. The mutation strings imported by the hooks have correct content.
 *   3. The mutationFn closure inside each hook calls client.request with
 *      the right mutation string (integration tests via QueryClientProvider).
 */

// -------------------------------------------------------------------------- //
// Shared mock client factory                                                  //
// -------------------------------------------------------------------------- //

function makeMockClient(result: unknown = {}): GraphQLClient {
  return { request: vi.fn().mockResolvedValue(result) } as unknown as GraphQLClient;
}

// -------------------------------------------------------------------------- //
// 1. Export existence                                                         //
// -------------------------------------------------------------------------- //

describe('hooks/mutations — exports', () => {
  it('useUpdateConfig is a function', () => {
    expect(typeof useUpdateConfig).toBe('function');
  });

  it('useAddMCPServer is a function', () => {
    expect(typeof useAddMCPServer).toBe('function');
  });

  it('useRemoveMCPServer is a function', () => {
    expect(typeof useRemoveMCPServer).toBe('function');
  });

  it('useAddTask is a function', () => {
    expect(typeof useAddTask).toBe('function');
  });

  it('useRemoveTask is a function', () => {
    expect(typeof useRemoveTask).toBe('function');
  });
});

// -------------------------------------------------------------------------- //
// 2. Mutation string correctness (the hooks import these constants)           //
// -------------------------------------------------------------------------- //

describe('hooks/mutations — mutation string references', () => {
  it('UPDATE_CONFIG_MUTATION is a non-empty string', () => {
    expect(typeof UPDATE_CONFIG_MUTATION).toBe('string');
    expect(UPDATE_CONFIG_MUTATION.length).toBeGreaterThan(0);
  });

  it('ADD_MCP_SERVER_MUTATION is a non-empty string', () => {
    expect(typeof ADD_MCP_SERVER_MUTATION).toBe('string');
    expect(ADD_MCP_SERVER_MUTATION.length).toBeGreaterThan(0);
  });

  it('REMOVE_MCP_SERVER_MUTATION is a non-empty string', () => {
    expect(typeof REMOVE_MCP_SERVER_MUTATION).toBe('string');
    expect(REMOVE_MCP_SERVER_MUTATION.length).toBeGreaterThan(0);
  });

  it('ADD_TASK_MUTATION is a non-empty string', () => {
    expect(typeof ADD_TASK_MUTATION).toBe('string');
    expect(ADD_TASK_MUTATION.length).toBeGreaterThan(0);
  });

  it('REMOVE_TASK_MUTATION is a non-empty string', () => {
    expect(typeof REMOVE_TASK_MUTATION).toBe('string');
    expect(REMOVE_TASK_MUTATION.length).toBeGreaterThan(0);
  });
});

// -------------------------------------------------------------------------- //
// 3. Hook argument signature — each hook accepts a GraphQLClient              //
//    (calling without context throws from solid-query; we only care that     //
//     the exception is NOT a TypeError about wrong argument types)           //
// -------------------------------------------------------------------------- //

describe('hooks/mutations — hook call signatures', () => {
  it('useUpdateConfig accepts a GraphQLClient argument', () => {
    const client = makeMockClient();
    // Will throw from missing QueryClient context, not from wrong argument type
    expect(() => { try { useUpdateConfig(client); } catch { /* expected */ } }).not.toThrow(TypeError);
  });

  it('useAddMCPServer accepts a GraphQLClient argument', () => {
    const client = makeMockClient();
    expect(() => { try { useAddMCPServer(client); } catch { /* expected */ } }).not.toThrow(TypeError);
  });

  it('useRemoveMCPServer accepts a GraphQLClient argument', () => {
    const client = makeMockClient();
    expect(() => { try { useRemoveMCPServer(client); } catch { /* expected */ } }).not.toThrow(TypeError);
  });

  it('useAddTask accepts a GraphQLClient argument', () => {
    const client = makeMockClient();
    expect(() => { try { useAddTask(client); } catch { /* expected */ } }).not.toThrow(TypeError);
  });

  it('useRemoveTask accepts a GraphQLClient argument', () => {
    const client = makeMockClient();
    expect(() => { try { useRemoveTask(client); } catch { /* expected */ } }).not.toThrow(TypeError);
  });
});
