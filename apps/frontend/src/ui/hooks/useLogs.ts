// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from "@tanstack/solid-query";
import type { GraphQLClient } from "graphql-request";

export interface LogsData {
  logs: string;
}

/**
 * Fetches recent logs from the /logs endpoint.
 *
 * The logs URL is derived from the GraphQL client URL by replacing the
 * path segment: /graphql -> /logs.
 *
 * Only polls when the page is visible to avoid wasteful background requests.
 *
 * @param client - GraphQL client instance (used to derive the base URL)
 * @param getToken - Optional callback that returns the bearer token for /logs auth
 * @returns solid-query result containing string or undefined while loading
 */
export function useLogs(
  client: GraphQLClient,
  getToken?: () => string | null,
) {
  const logsUrl = (client as unknown as { url: string }).url.replace(
    /\/graphql$/,
    "/logs",
  );

  return createQuery<string>(() => ({
    queryKey: ["logs"],
    queryFn: async () => {
      const headers: Record<string, string> = {};
      const token = getToken?.();
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }
      const res = await fetch(logsUrl, { headers });
      if (!res.ok) throw new Error(`logs fetch failed: ${res.status}`);
      return res.text();
    },
    refetchInterval: (_query) => {
      if (
        typeof document !== "undefined" &&
        document.visibilityState === "hidden"
      ) {
        return false;
      }
      return 10_000;
    },
    enabled:
      typeof document !== "undefined"
        ? document.visibilityState === "visible"
        : true,
  }));
}
