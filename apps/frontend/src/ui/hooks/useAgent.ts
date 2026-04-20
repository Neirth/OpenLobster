// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { AGENT_QUERY } from '../graphql/queries/index';
import type { Agent } from '../types/index';

interface AgentQueryResult {
  agent: Agent;
}

/**
 * Fetches the current agent status with a 5-second polling interval.
 *
 * @param client - GraphQL client instance (web: proxied; terminal: direct URL)
 * @returns solid-query result containing the Agent or undefined while loading
 */
export function useAgent(client: GraphQLClient) {
  return createQuery<Agent>(() => ({
    queryKey: ['agent'],
    queryFn: async () => {
      const data = await client.request<AgentQueryResult>(AGENT_QUERY);
      return data.agent;
    },
    refetchInterval: 5_000,
    staleTime: 4_000,
  }));
}
