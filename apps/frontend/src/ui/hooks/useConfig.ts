// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { CONFIG_QUERY } from '../graphql/queries/index';
import type { AppConfig } from '../types/index';

interface ConfigQueryResult {
  config: AppConfig;
}

/**
 * Fetches the agent configuration with a 30-second polling interval.
 * Config changes are rare and typically user-initiated.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing AppConfig or undefined while loading
 */
export function useConfig(client: GraphQLClient) {
  return createQuery<AppConfig>(() => ({
    queryKey: ['config'],
    queryFn: async () => {
      const data = await client.request<ConfigQueryResult>(CONFIG_QUERY);
      return data.config;
    },
    refetchInterval: 30_000,
  }));
}
