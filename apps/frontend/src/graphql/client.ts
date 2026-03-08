// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Singleton GraphQL client for the web frontend.
 *
 * Requests to '/graphql' are proxied by Vite dev server (and by the
 * production reverse proxy) to the backend at :8080.
 *
 * The client reads the access token from sessionStorage on every request
 * so that token changes are picked up immediately. When the backend returns
 * a 401, `needsAuth` is set to true and the AccessTokenModal is shown.
 *
 * @module graphql/client
 */

import { createGraphqlClient } from '@openlobster/ui/graphql';
import { getStoredToken, setNeedsAuth } from '../stores/authStore';

// En dev, usar /graphql para que Vite proxy envíe al backend (evita CORS y problemas de conexión).
// En prod, usar la URL configurada o el backend por defecto.
export const GRAPHQL_ENDPOINT =
	import.meta.env.VITE_GRAPHQL_ENDPOINT ??
	(import.meta.env.DEV ? '/graphql' : 'http://127.0.0.1:8080/graphql');

const _client = createGraphqlClient(GRAPHQL_ENDPOINT, getStoredToken);

// Wrap request() to surface 401 errors as the auth modal trigger.
const _originalRequest = _client.request.bind(_client);
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
