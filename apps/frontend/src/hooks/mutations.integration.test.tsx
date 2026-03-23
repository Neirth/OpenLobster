// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi } from 'vitest';
import { render } from '@solidjs/testing-library';
import { QueryClient, QueryClientProvider } from '@tanstack/solid-query';
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
 * Integration tests that run each hook inside a QueryClientProvider
 * to exercise the mutationFn closure, verifying it calls
 * client.request with the correct mutation string and variables.
 */

function makeMockClient(result: unknown = {}): GraphQLClient {
  return { request: vi.fn().mockResolvedValue(result) } as unknown as GraphQLClient;
}

function makeQueryClient() {
  return new QueryClient({ defaultOptions: { mutations: { retry: false } } });
}

describe('hooks/mutations — mutationFn integration', () => {
  it('useUpdateConfig mutationFn calls client.request with correct mutation', async () => {
    const client = makeMockClient({ updateConfig: { agentName: 'agent' } });
    const variables = { input: { agentName: 'test-agent' } };
    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useUpdateConfig(client);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await mutateAsync(variables);
    expect(client.request).toHaveBeenCalledWith(UPDATE_CONFIG_MUTATION, variables);
  });

  it('useAddMCPServer mutationFn calls client.request with correct mutation', async () => {
    const client = makeMockClient({ addMCPServer: { id: '1', name: 'srv' } });
    const variables = { name: 'srv', transport: 'stdio', command: 'node srv.js' };
    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useAddMCPServer(client);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await mutateAsync(variables);
    expect(client.request).toHaveBeenCalledWith(ADD_MCP_SERVER_MUTATION, variables);
  });

  it('useRemoveMCPServer mutationFn calls client.request with correct mutation', async () => {
    const client = makeMockClient({ removeMCPServer: { success: true } });
    const variables = { id: 'server-id' };
    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useRemoveMCPServer(client);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await mutateAsync(variables);
    expect(client.request).toHaveBeenCalledWith(REMOVE_MCP_SERVER_MUTATION, variables);
  });

  it('useAddTask mutationFn calls client.request with correct mutation', async () => {
    const client = makeMockClient({ addTask: { id: 'task-1' } });
    const variables = {
      name: 'Daily brief',
      prompt: 'Write a brief',
      schedule: '0 8 * * *',
      channel: 'discord',
      isCyclic: true,
    };
    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useAddTask(client);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await mutateAsync(variables);
    expect(client.request).toHaveBeenCalledWith(ADD_TASK_MUTATION, variables);
  });

  it('useRemoveTask mutationFn calls client.request with correct mutation', async () => {
    const client = makeMockClient({ removeTask: { success: true } });
    const variables = { taskId: 'task-99' };
    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useRemoveTask(client);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await mutateAsync(variables);
    expect(client.request).toHaveBeenCalledWith(REMOVE_TASK_MUTATION, variables);
  });

  it('useUpdateConfig onError handler logs to console.error', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const errorClient = {
      request: vi.fn().mockRejectedValue(new Error('network failure')),
    } as unknown as GraphQLClient;

    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useUpdateConfig(errorClient);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await expect(mutateAsync({ input: {} })).rejects.toThrow('network failure');
    expect(consoleSpy).toHaveBeenCalledWith(
      'updateConfig mutation failed:',
      expect.any(Error),
    );
    consoleSpy.mockRestore();
  });

  it('useAddMCPServer onError handler logs to console.error', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const errorClient = {
      request: vi.fn().mockRejectedValue(new Error('fail')),
    } as unknown as GraphQLClient;

    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useAddMCPServer(errorClient);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await expect(mutateAsync({})).rejects.toThrow('fail');
    expect(consoleSpy).toHaveBeenCalledWith('addMCPServer mutation failed:', expect.any(Error));
    consoleSpy.mockRestore();
  });

  it('useRemoveMCPServer onError handler logs to console.error', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const errorClient = {
      request: vi.fn().mockRejectedValue(new Error('fail')),
    } as unknown as GraphQLClient;

    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useRemoveMCPServer(errorClient);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await expect(mutateAsync({})).rejects.toThrow('fail');
    expect(consoleSpy).toHaveBeenCalledWith('removeMCPServer mutation failed:', expect.any(Error));
    consoleSpy.mockRestore();
  });

  it('useAddTask onError handler logs to console.error', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const errorClient = {
      request: vi.fn().mockRejectedValue(new Error('fail')),
    } as unknown as GraphQLClient;

    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useAddTask(errorClient);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await expect(mutateAsync({})).rejects.toThrow('fail');
    expect(consoleSpy).toHaveBeenCalledWith('addTask mutation failed:', expect.any(Error));
    consoleSpy.mockRestore();
  });

  it('useRemoveTask onError handler logs to console.error', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const errorClient = {
      request: vi.fn().mockRejectedValue(new Error('fail')),
    } as unknown as GraphQLClient;

    let mutateAsync!: (v: Record<string, unknown>) => Promise<unknown>;

    render(() => (
      <QueryClientProvider client={makeQueryClient()}>
        {(() => {
          const m = useRemoveTask(errorClient);
          mutateAsync = (v) => m.mutateAsync(v);
          return <></>;
        })()}
      </QueryClientProvider>
    ));

    await expect(mutateAsync({})).rejects.toThrow('fail');
    expect(consoleSpy).toHaveBeenCalledWith('removeTask mutation failed:', expect.any(Error));
    consoleSpy.mockRestore();
  });
});
