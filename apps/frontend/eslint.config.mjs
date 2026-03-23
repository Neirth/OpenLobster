// Copyright (c) OpenLobster contributors. See LICENSE for details.

import js from '@eslint/js';
import tsPlugin from '@typescript-eslint/eslint-plugin';
import tsParser from '@typescript-eslint/parser';
import solidPlugin from 'eslint-plugin-solid';

/** Browser + Web API globals shared across src files. */
const browserGlobals = {
  // Core
  console: 'readonly',
  process: 'readonly',
  global: 'readonly',
  // DOM
  document: 'readonly',
  window: 'readonly',
  navigator: 'readonly',
  location: 'readonly',
  history: 'readonly',
  localStorage: 'readonly',
  sessionStorage: 'readonly',
  // HTML elements
  HTMLElement: 'readonly',
  HTMLInputElement: 'readonly',
  HTMLImageElement: 'readonly',
  HTMLButtonElement: 'readonly',
  HTMLDivElement: 'readonly',
  HTMLAnchorElement: 'readonly',
  HTMLTextAreaElement: 'readonly',
  HTMLSelectElement: 'readonly',
  HTMLFormElement: 'readonly',
  HTMLUListElement: 'readonly',
  File: 'readonly',
  Storage: 'readonly',
  NodeListOf: 'readonly',
  // Events
  Event: 'readonly',
  KeyboardEvent: 'readonly',
  MouseEvent: 'readonly',
  MessageEvent: 'readonly',
  CustomEvent: 'readonly',
  // Node types
  Node: 'readonly',
  NodeList: 'readonly',
  // Web APIs
  fetch: 'readonly',
  URL: 'readonly',
  URLSearchParams: 'readonly',
  RequestInfo: 'readonly',
  RequestInit: 'readonly',
  Response: 'readonly',
  Request: 'readonly',
  Headers: 'readonly',
  FileReader: 'readonly',
  AbortController: 'readonly',
  CSS: 'readonly',
  MutationObserver: 'readonly',
  IntersectionObserver: 'readonly',
  ResizeObserver: 'readonly',
  WebSocket: 'readonly',
  // Timers
  setTimeout: 'readonly',
  clearTimeout: 'readonly',
  setInterval: 'readonly',
  clearInterval: 'readonly',
  requestAnimationFrame: 'readonly',
  cancelAnimationFrame: 'readonly',
  // Misc
  crypto: 'readonly',
  performance: 'readonly',
  structuredClone: 'readonly',
};

/** Vitest globals for test files. */
const vitestGlobals = {
  describe: 'readonly',
  it: 'readonly',
  test: 'readonly',
  expect: 'readonly',
  beforeEach: 'readonly',
  afterEach: 'readonly',
  beforeAll: 'readonly',
  afterAll: 'readonly',
  vi: 'readonly',
};

export default [
  {
    ignores: ['dist/**', 'node_modules/**', '.turbo/**'],
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
        ecmaFeatures: { jsx: true },
      },
      globals: browserGlobals,
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
      solid: solidPlugin,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...tsPlugin.configs.recommended.rules,
      ...solidPlugin.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      'no-debugger': 'error',
    },
  },
  {
    files: ['src/**/*.test.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
        ecmaFeatures: { jsx: true },
      },
      globals: { ...browserGlobals, ...vitestGlobals },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...tsPlugin.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      'no-undef': 'error',
    },
  },
  {
    files: ['tests/**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
        ecmaFeatures: { jsx: true },
      },
      globals: { ...browserGlobals, ...vitestGlobals },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...tsPlugin.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
    },
  },
  {
    files: ['tests/e2e/**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
      globals: {
        ...vitestGlobals,
        __dirname: 'readonly',
        __filename: 'readonly',
      },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...tsPlugin.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': 'off',
    },
  },
];
