import type { CodegenConfig } from "@graphql-codegen/cli";
import { dirname, join } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));

const config: CodegenConfig = {
  // Schema at monorepo root — shared by backend and frontend.
  schema: [
    join(__dirname, "schema/root.graphql"),
    join(__dirname, "schema/shared.graphql"),
    join(__dirname, "schema/agent.graphql"),
    join(__dirname, "schema/config.graphql"),
    join(__dirname, "schema/conversations.graphql"),
    join(__dirname, "schema/memory.graphql"),
    join(__dirname, "schema/mcp.graphql"),
    join(__dirname, "schema/tasks.graphql"),
    join(__dirname, "schema/skills.graphql"),
    join(__dirname, "schema/tools.graphql"),
    join(__dirname, "schema/subscriptions.graphql"),
    join(__dirname, "schema/plugins.graphql"),
  ],

  documents: [
    join(__dirname, "apps/frontend/src/graphql/**/*.ts"),
  ],

  generates: {
    // Frontend SDK — types + graphql-request SDK
    [join(__dirname, "apps/frontend/src/graphql/generated.ts")]: {
      plugins: [
        "typescript",
        "typescript-operations",
        "typescript-graphql-request",
      ],
      config: {
        scalars: {
          JSON: "Record<string, unknown>",
        },
        avoidOptionals: false,
        maybeValue: "T | null | undefined",
        enumsAsTypes: true,
        useTypeImports: true,
      },
    },
  },
};

export default config;
