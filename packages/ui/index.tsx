// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * @openlobster/ui — Shared UI package
 *
 * This package provides the shared foundation used by both frontends:
 *
 *   - theme     Design tokens as TypeScript constants (for the terminal)
 *   - types     Domain types matching the GraphQL API
 *   - graphql   Shared query/mutation strings and GraphQL client factory
 *   - hooks     Headless data hooks (useAgent, useMetrics, useChannels, etc.)
 *               Require @tanstack/solid-query to be installed in the host app.
 *
 * CSS design tokens (for the web frontend) are in src/styles/ and must be
 * imported directly by path:
 *   import '@openlobster/ui/styles/tokens.css'
 *   import '@openlobster/ui/styles/reset.css'
 *   import '@openlobster/ui/styles/global.css'
 *
 * Platform-specific components (HTML elements for web, OpenTUI elements for
 * terminal) live in their respective apps, NOT in this package.
 */

export * from './src/theme/index';
export * from './src/types/index';
export * from './src/graphql/index';
export * from './src/hooks/index';
