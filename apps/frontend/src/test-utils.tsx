// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { render } from "@solidjs/testing-library";
import type { JSXElement } from "solid-js";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

export type RenderResult = ReturnType<typeof render>;

export function renderWithQueryClient(ui: () => JSXElement): RenderResult {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(() => (
    <QueryClientProvider client={queryClient}>{ui()}</QueryClientProvider>
  ));
}
