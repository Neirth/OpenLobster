// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * @openlobster/ui-tests — Mock implementations for testing
 *
 * This package provides mock implementations of the @openlobster/ui hooks
 * for unit testing. It re-exports the same types and GraphQL queries/mutations
 * as the real package, but returns mock data instead of making network requests.
 *
 * Configured in vitest.config.ts:
 *   alias: { '@openlobster/ui': uiTestsSrc }
 *
 * This allows tests to import from '@openlobster/ui/hooks' and receive
 * mock implementations automatically.
 */

export * from "@openlobster/ui/types";
export * from "@openlobster/ui/graphql";
export * from "@openlobster/ui/theme";
export * from "./hooks/index.js";
export * from "./graphql/mutations/index.js";
