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
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const _originalRequest = _client.request.bind(_client) as (...args: any[]) => Promise<any>;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(_client as any).request = async (...args: any[]) => {
	try {
		return await _originalRequest(...args);
	} catch (err: unknown) {
		const status = (err as { response?: { status?: number } })?.response?.status;
		if (status === 401) {
			setNeedsAuth(true);
		}
		throw err;
	}
};

export const client = _client;
