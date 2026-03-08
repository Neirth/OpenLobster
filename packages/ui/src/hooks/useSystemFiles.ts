// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from "@tanstack/solid-query";
import type { GraphQLClient } from "graphql-request";
import type { SystemFile } from "../types";

const SYSTEM_FILES_QUERY = `
  query SystemFiles {
    systemFiles {
      name
      path
      content
      lastModified
    }
  }
`;

export function useSystemFiles(client: GraphQLClient) {
  return createQuery(() => ({
    queryKey: ["systemFiles"],
    queryFn: async () => {
      const result = await client.request<{ systemFiles: SystemFile[] }>(
        SYSTEM_FILES_QUERY,
      );
      return result.systemFiles;
    },
    refetchInterval: 30_000,
  }));
}
