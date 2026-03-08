// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { render } from "solid-js/web";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import Root from "./App";

const root = document.getElementById("app");

if (import.meta.env.DEV && !(root instanceof HTMLElement)) {
  throw new Error(
    'Root element with id "app" not found. Check your index.html.',
  );
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnMount: "always",
      refetchOnWindowFocus: false,
      refetchOnReconnect: true,
      retry: 1,
    },
  },
});

render(
  () => (
    <QueryClientProvider client={queryClient}>
      <Root />
    </QueryClientProvider>
  ),
  root!,
);
