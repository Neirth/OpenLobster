// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { render } from "solid-js/web";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { initTheme } from "./stores/themeStore";
import Root from "./App";

initTheme();

const root = document.getElementById("app");

if (import.meta.env.DEV && !(root instanceof HTMLElement)) {
  throw new Error(
    'Root element with id "app" not found. Check your index.html.',
  );
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 0,
      gcTime: 1000 * 60 * 5, // 5 minutes
      refetchOnMount: "always",
      refetchOnWindowFocus: false,
      refetchOnReconnect: true,
      retry: false,
    },
  },
});

if (typeof window !== "undefined") {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (window as any).queryClient = queryClient;
}

render(
  () => (
    <QueryClientProvider client={queryClient}>
      <Root />
    </QueryClientProvider>
  ),
  root!,
);
