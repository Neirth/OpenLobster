// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * GraphQL endpoint configuration.
 * Extracted into a separate file to avoid circular dependencies between authStore and client.
 */

function getGraphqlEndpoint(): string {
  if (import.meta.env.VITE_GRAPHQL_ENDPOINT) {
    return import.meta.env.VITE_GRAPHQL_ENDPOINT;
  }
  if (typeof window !== 'undefined' && window.location?.origin) {
    return `${window.location.origin}/graphql`;
  }
  return '/graphql';
}

export const GRAPHQL_ENDPOINT = getGraphqlEndpoint();
