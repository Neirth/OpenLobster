// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * @/ui-tests — Mock implementations for testing
 *
 * This package provides mock implementations of the @/ui hooks
 * for unit testing. It re-exports the same types and GraphQL queries/mutations
 * as the real package, but returns mock data instead of making network requests.
 *
 * Configured in vitest.config.ts:
 *   alias: { '@/ui': uiTestsSrc }
 *
 * This allows tests to import from '@/hooks' and receive
 * mock implementations automatically.
 */

export * from "../../src/types/index.js";
export * from "./theme/index.js";
export * from "./hooks/index.js";
export * from "./graphql/mutations/index.js";
