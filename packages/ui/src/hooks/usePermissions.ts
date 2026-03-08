// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { MCP_USERS_QUERY, TOOL_PERMISSIONS_QUERY } from '../graphql/queries/index';
import type { McpUser, ToolPermission } from '../types/index';

interface McpUsersQueryResult {
  mcpUsers: McpUser[];
}

interface ToolPermissionsQueryResult {
  toolPermissions: ToolPermission[];
}

/**
 * Fetches the list of users that have had at least one conversation, deduplicated
 * by channelId. These are the identifiers used in the tool permission system.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing McpUser[] or undefined while loading
 */
export function useMcpUsers(client: GraphQLClient) {
  return createQuery<McpUser[]>(() => ({
    queryKey: ['mcpUsers'],
    queryFn: async () => {
      const data = await client.request<McpUsersQueryResult>(MCP_USERS_QUERY);
      return data.mcpUsers ?? [];
    },
    refetchInterval: 30_000,
  }));
}

/**
 * Fetches all explicit tool permission entries for a specific user. Tools not
 * present in the result are denied by default.
 *
 * @param client - GraphQL client instance
 * @param userId - The channelId that identifies the user
 * @returns solid-query result containing ToolPermission[] or undefined while loading
 */
export function useToolPermissions(client: GraphQLClient, userId: () => string) {
  return createQuery<ToolPermission[]>(() => ({
    queryKey: ['toolPermissions', userId()],
    queryFn: async () => {
      if (!userId()) return [];
      const data = await client.request<ToolPermissionsQueryResult>(
        TOOL_PERMISSIONS_QUERY,
        { userId: userId() },
      );
      return data.toolPermissions ?? [];
    },
    enabled: () => !!userId(),
    refetchInterval: 30_000,
  }));
}
