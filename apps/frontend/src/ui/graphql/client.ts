// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Factory for the shared GraphQL client used by both frontends.
 *
 * The web frontend calls createGraphqlClient('/graphql') so the Vite proxy
 * forwards requests to the backend at :8080.
 *
 * The terminal frontend calls createGraphqlClient(process.env.GRAPHQL_URL)
 * which defaults to 'http://localhost:8080/graphql'.
 *
 * @param url      - Full GraphQL endpoint URL
 * @param getToken - Optional callback that returns the current bearer token.
 *                   Called on every request so token changes are picked up
 *                   without recreating the client.
 * @returns Configured GraphQLClient instance
 */

import { GraphQLClient } from 'graphql-request';

export function createGraphqlClient(
  url: string,
  getToken?: () => string | null,
): GraphQLClient {
  return new GraphQLClient(url, {
    headers: () => {
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      const token = getToken?.();
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }
      return headers;
    },
  });
}
