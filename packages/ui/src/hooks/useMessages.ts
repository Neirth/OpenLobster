// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { Accessor } from 'solid-js';
import type { GraphQLClient } from 'graphql-request';
import { MESSAGES_QUERY } from '../graphql/queries/index';
import type { Message } from '../types/index';

interface MessagesQueryResult {
  messages: Message[];
}

/**
 * Fetches messages for a given conversation.
 * Real-time updates come from WebSocket subscriptions, not polling.
 *
 * @param client - GraphQL client instance
 * @param conversationId - Reactive accessor returning the active conversation ID.
 *                        Pass an empty string to disable fetching.
 * @returns solid-query result containing Message[] or undefined while loading
 */
export function useMessages(client: GraphQLClient, conversationId: Accessor<string>) {
  return createQuery<Message[]>(() => ({
    queryKey: ['messages', conversationId()],
    queryFn: async () => {
      const data = await client.request<MessagesQueryResult>(MESSAGES_QUERY, {
        conversationId: conversationId(),
      });
      return data.messages;
    },
    enabled: conversationId().length > 0,
  }));
}
