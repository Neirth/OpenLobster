// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Singleton GraphQL client for the web frontend.
 *
 * Requests go to the same host as the app (base domain). In dev, Vite proxies
 * /graphql to the backend; in production, the reverse proxy does the same.
 * Override with VITE_GRAPHQL_ENDPOINT when needed.
 *
 * The client reads the access token from sessionStorage on every request
 * so that token changes are picked up immediately. When the backend returns
 * a 401, `needsAuth` is set to true and the AccessTokenModal is shown.
 *
 * @module graphql/client
 */

import { createGraphqlClient } from '@openlobster/ui/graphql';
import { getStoredToken, setNeedsAuth } from '../stores/authStore';

/** GraphQL endpoint: same-origin /graphql so it works on any domain (e.g. https://agent.hoki-ghoul.ts.net). */
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

const _client = createGraphqlClient(GRAPHQL_ENDPOINT, getStoredToken);

// Wrap request() to surface 401 errors as the auth modal trigger.
// Be defensive: in test environments the mocked factory may return an object
// without a function on `request` at module evaluation time. Guard the bind
// to avoid a runtime TypeError.


export const client = new Proxy(_client, {
  get(target, prop, receiver) {
    if (prop === 'request') {
      return async (...args: unknown[]) => {
        let fn = (target as { request?: (...args: unknown[]) => Promise<unknown> }).request;
        if (typeof fn !== 'function') {
          try {
            const real = createGraphqlClient(GRAPHQL_ENDPOINT, getStoredToken);
            try { Object.assign(target as object, real); } catch {
              // Ignore property assignment errors (likely in test mocks)
            }
            fn = (target as { request?: (...args: unknown[]) => Promise<unknown> }).request;
          } catch {
            throw new Error('GraphQL client has no request method');
          }
        }
        try {
          return await fn!.apply(target, args);
        } catch (err: unknown) {
          const status = (err as { response?: { status?: number } })?.response?.status;
          if (status === 401) {
            setNeedsAuth(true);
          }
          throw err;
        }
      };
    }
    // Forward other properties directly.
    return Reflect.get(target, prop, receiver);
  },
});
