// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { CHANNELS_QUERY } from '../graphql/queries/index';
import type { Channel } from '../types/index';

interface ChannelsQueryResult {
  channels: Channel[];
}

/**
 * Fetches the list of messaging channels with a 5-second polling interval.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing Channel[] or undefined while loading
 */
export function useChannels(client: GraphQLClient) {
  return createQuery<Channel[]>(() => ({
    queryKey: ['channels'],
    queryFn: async () => {
      const data = await client.request<ChannelsQueryResult>(CHANNELS_QUERY);
      return data.channels;
    },
    refetchInterval: 5_000,
  }));
}
