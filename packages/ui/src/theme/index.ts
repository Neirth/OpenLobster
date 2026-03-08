// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * OpenLobster design tokens as TypeScript constants.
 *
 * These values are the canonical source of truth shared between the terminal
 * frontend (@openlobster/terminal) and any non-CSS context. The web frontend
 * uses the CSS Custom Properties in src/styles/tokens.css, which mirror these
 * exact values.
 *
 * Usage in the terminal (OpenTUI):
 *   import { theme } from '@openlobster/ui/theme';
 *   <box backgroundColor={theme.bgBase}>...</box>
 */
export const theme = {
  // Backgrounds — hierarchy by tonality, no borders
  bgBase:     '#141414', // Main content area
  bgSurface:  '#0D0D0D', // Header, footer
  bgSunken:   '#0A0A0A', // Sidebar (darkest chrome)
  bgElevated: '#1E1E1E', // Hover state
  bgSelected: '#252525', // Active / selected item
  bgOverlay:  '#2C2C2C', // Modals, command palette

  // Text
  textPrimary:   '#E0E0E0',
  textSecondary: '#A0A0A0',
  textMuted:     '#606060',
  textInverse:   '#0D0D0D',

  // Semantic
  accent:  '#4F8EF7',
  success: '#4CAF50',
  warning: '#FF9800',
  error:   '#F44336',
  info:    '#29B6F6',
  purple:  '#9C27B0',

  // Borders (use sparingly — prefer background contrast)
  border:      '#1F1F1F',
  borderMuted: '#161616',
} as const;

export type Theme = typeof theme;

/**
 * Light theme overrides (future use).
 * Only the values that differ from the dark theme are listed.
 */
export const lightTheme: Partial<Theme> = {
  bgBase:        '#FAFAFA',
  bgSurface:     '#F0F0F0',
  bgSunken:      '#E8E8E8',
  bgElevated:    '#FFFFFF',
  bgSelected:    '#E3EFFE',
  textPrimary:   '#0D0D0D',
  textSecondary: '#404040',
  textMuted:     '#909090',
  border:        '#D0D0D0',
};

/**
 * Semantic status color map — used by both StatusBadge (web) and
 * StatusBadge (terminal) to resolve a status name to a hex color.
 */
export const statusColors = {
  success: theme.success,
  warning: theme.warning,
  error:   theme.error,
  muted:   theme.textMuted,
  info:    theme.info,
} as const;

export type StatusColor = keyof typeof statusColors;
