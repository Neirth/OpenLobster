// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { MCP_SERVERS_QUERY, MCP_TOOLS_QUERY } from '../graphql/queries/index';
import type { McpServer, McpTool } from '../types/index';

interface McpServersQueryResult {
  mcpServers: McpServer[];
}

interface McpToolsQueryResult {
  mcpTools: McpTool[];
}

/**
 * Fetches the list of connected MCP servers with a 5-second polling interval.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing McpServer[] or undefined while loading
 */
export function useMcpServers(client: GraphQLClient) {
  return createQuery<McpServer[]>(() => ({
    queryKey: ['mcpServers'],
    queryFn: async () => {
      const data = await client.request<McpServersQueryResult>(MCP_SERVERS_QUERY);
      return data.mcpServers;
    },
    refetchInterval: 5_000,
  }));
}

/**
 * Fetches all tools exposed by connected MCP servers with a 5-second polling interval.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing McpTool[] or undefined while loading
 */
export function useMcpTools(client: GraphQLClient) {
  return createQuery<McpTool[]>(() => ({
    queryKey: ['mcpTools'],
    queryFn: async () => {
      const data = await client.request<McpToolsQueryResult>(MCP_TOOLS_QUERY);
      return data.mcpTools;
    },
    refetchInterval: 5_000,
  }));
}
