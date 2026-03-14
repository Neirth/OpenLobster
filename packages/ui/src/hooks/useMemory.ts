// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { MEMORY_QUERY } from '../graphql/queries/index';
import type { MemoryGraph } from '../types/index';

interface MemoryQueryResult {
  memory: MemoryGraph;
}

/**
 * Fetches the full memory graph (nodes + edges) with a 10-second polling interval.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing MemoryGraph or undefined while loading
 */
export function useMemory(client: GraphQLClient) {
  return createQuery<MemoryGraph>(() => ({
    queryKey: ['memory'],
    queryFn: async () => {
      const data = await client.request<MemoryQueryResult>(MEMORY_QUERY);
      // Backend may return null if there is an error or an empty graph; normalise to an empty structure.
      const graph = data.memory;
      return graph ?? { nodes: [], edges: [] };
    },
    refetchInterval: 10_000,
  }));
}
