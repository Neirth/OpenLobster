// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { CONVERSATIONS_QUERY } from '../graphql/queries/index';
import type { Conversation } from '../types/index';

interface ConversationsQueryResult {
  conversations: Conversation[];
}

/**
 * Fetches all active conversations.
 * Real-time updates come from WebSocket subscriptions, not polling.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing Conversation[] or undefined while loading
 */
export function useConversations(client: GraphQLClient) {
  return createQuery<Conversation[]>(() => ({
    queryKey: ['conversations'],
    queryFn: async () => {
      const data = await client.request<ConversationsQueryResult>(CONVERSATIONS_QUERY);
      return data.conversations;
    },
  }));
}
