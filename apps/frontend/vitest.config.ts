// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { defineConfig } from "vitest/config";
import solid from "vite-plugin-solid";
import path from "path";

const srcDir = path.resolve(__dirname, "./src");
const uiMockSrc = path.resolve(__dirname, "./tests/ui");

export default defineConfig({
  plugins: [solid()],
  resolve: {
    dedupe: ["graphql", "solid-js", "@solidjs/router", "@tanstack/solid-query"],
    // IMPORTANT: More specific aliases must come before less specific ones.
    alias: [
      // Mock overrides: point specific subpaths to their test-mock counterparts.
      // Only mock what exists in tests/ui — leave @/graphql/config etc. untouched.
      { find: "@/graphql/mutations", replacement: path.resolve(uiMockSrc, "graphql/mutations") },
      { find: "@/graphql/queries", replacement: path.resolve(uiMockSrc, "graphql/queries") },
      { find: "@/hooks", replacement: path.resolve(uiMockSrc, "hooks") },
      { find: "@/theme", replacement: path.resolve(uiMockSrc, "theme") },
      // Legacy @/ui maps to mock root (kept for backward compat)
      { find: "@/ui", replacement: uiMockSrc },
      // Base @/ alias maps to src for everything else
      { find: "@", replacement: srcDir },
    ],
  },
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
    include: ["src/**/*.test.{ts,tsx}", "tests/**/*.test.{ts,tsx}"],
  },
});
