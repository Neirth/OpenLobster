import type { CodegenConfig } from "@graphql-codegen/cli";

const config: CodegenConfig = {
  // Schema at monorepo root — shared by backend (gqlgen) and frontend codegen.
  schema: [
    "./schema/root.graphql",
    "./schema/shared.graphql",
    "./schema/agent.graphql",
    "./schema/config.graphql",
    "./schema/conversations.graphql",
    "./schema/memory.graphql",
    "./schema/mcp.graphql",
    "./schema/tasks.graphql",
    "./schema/skills.graphql",
    "./schema/tools.graphql",
    "./schema/subscriptions.graphql",
  ],

  documents: [
    // All .graphql operation files in the monorepo
    "packages/ui/src/graphql/**/*.ts",
    "apps/frontend/src/graphql/**/*.ts",
  ],

  generates: {
    // Shared UI package — types + graphql-request SDK
    "packages/ui/src/graphql/generated.ts": {
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
        // Use `string` for ID scalars (matches current usage in codebase)
        useTypeImports: true,
      },
    },
  },

  hooks: {
    // Keep the generated file header clean
    afterAllFileWrite: [],
  },
};

export default config;
