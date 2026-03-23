// Copyright (c) OpenLobster contributors. See LICENSE for details.
import { defineConfig } from "vitest/config";
import solid from "vite-plugin-solid";
import { resolve } from "path";

const uiTestsHooks = resolve(__dirname, "../../packages/ui-tests/src/hooks");

export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: {
      // Swap only the hooks from @openlobster/ui to ui-tests
      // Keep types, graphql, and theme coming from the real package
      "@openlobster/ui/hooks": uiTestsHooks,
    },
  },
  test: {
    environment: "happy-dom",
    setupFiles: ["src/test-setup.ts"],
    globals: true,
    include: [
      "src/**/*.test.ts",
      "src/**/*.test.tsx",
      "tests/**/*.test.ts",
      "tests/**/*.test.tsx",
    ],
    exclude: ["node_modules", "dist"],
    coverage: {
      provider: "v8",
      reporter: ["text", "html", "json"],
      all: true,
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.d.ts", "src/**/index.ts", "src/**/index.tsx"],
    },
  },
});
