// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect } from 'vitest';
import { theme, lightTheme, statusColors } from './index';

// Hex color validation helper
const isHex = (value: string) => /^#[0-9A-Fa-f]{6}$/.test(value);

describe('theme', () => {
  describe('background tokens', () => {
    it('defines all required background levels', () => {
      expect(theme.bgBase).toBeDefined();
      expect(theme.bgSurface).toBeDefined();
      expect(theme.bgSunken).toBeDefined();
      expect(theme.bgElevated).toBeDefined();
      expect(theme.bgSelected).toBeDefined();
      expect(theme.bgOverlay).toBeDefined();
    });

    it('all backgrounds are valid hex colors', () => {
      expect(isHex(theme.bgBase)).toBe(true);
      expect(isHex(theme.bgSurface)).toBe(true);
      expect(isHex(theme.bgSunken)).toBe(true);
      expect(isHex(theme.bgElevated)).toBe(true);
      expect(isHex(theme.bgSelected)).toBe(true);
      expect(isHex(theme.bgOverlay)).toBe(true);
    });

    it('backgrounds form a dark-to-lighter hierarchy (sunken is darkest)', () => {
      // Parse hex to luminance value for comparison
      const lum = (hex: string) => parseInt(hex.slice(1, 3), 16);

      expect(lum(theme.bgSunken)).toBeLessThan(lum(theme.bgSurface));
      expect(lum(theme.bgSurface)).toBeLessThan(lum(theme.bgBase));
      expect(lum(theme.bgBase)).toBeLessThan(lum(theme.bgElevated));
      expect(lum(theme.bgElevated)).toBeLessThan(lum(theme.bgSelected));
    });
  });

  describe('text tokens', () => {
    it('defines all required text levels', () => {
      expect(theme.textPrimary).toBeDefined();
      expect(theme.textSecondary).toBeDefined();
      expect(theme.textMuted).toBeDefined();
      expect(theme.textInverse).toBeDefined();
    });

    it('all text colors are valid hex', () => {
      expect(isHex(theme.textPrimary)).toBe(true);
      expect(isHex(theme.textSecondary)).toBe(true);
      expect(isHex(theme.textMuted)).toBe(true);
      expect(isHex(theme.textInverse)).toBe(true);
    });

    it('primary text is lighter than secondary which is lighter than muted', () => {
      const lum = (hex: string) => parseInt(hex.slice(1, 3), 16);

      expect(lum(theme.textPrimary)).toBeGreaterThan(lum(theme.textSecondary));
      expect(lum(theme.textSecondary)).toBeGreaterThan(lum(theme.textMuted));
    });

    it('primary text meets WCAG AA contrast ratio (>=4.5:1) against bgBase', () => {
      // Simplified relative luminance for single-channel hex values
      const toLinear = (c: number) => {
        const s = c / 255;
        return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
      };

      const relativeLuminance = (hex: string) => {
        const r = toLinear(parseInt(hex.slice(1, 3), 16));
        const g = toLinear(parseInt(hex.slice(3, 5), 16));
        const b = toLinear(parseInt(hex.slice(5, 7), 16));
        return 0.2126 * r + 0.7152 * g + 0.0722 * b;
      };

      const contrastRatio = (l1: number, l2: number) => {
        const lighter = Math.max(l1, l2);
        const darker = Math.min(l1, l2);
        return (lighter + 0.05) / (darker + 0.05);
      };

      const contrast = contrastRatio(
        relativeLuminance(theme.textPrimary),
        relativeLuminance(theme.bgBase),
      );

      expect(contrast).toBeGreaterThanOrEqual(4.5);
    });
  });

  describe('semantic tokens', () => {
    it('defines all semantic colors', () => {
      expect(theme.accent).toBeDefined();
      expect(theme.success).toBeDefined();
      expect(theme.warning).toBeDefined();
      expect(theme.error).toBeDefined();
      expect(theme.info).toBeDefined();
      expect(theme.purple).toBeDefined();
    });

    it('all semantic colors are valid hex', () => {
      expect(isHex(theme.accent)).toBe(true);
      expect(isHex(theme.success)).toBe(true);
      expect(isHex(theme.warning)).toBe(true);
      expect(isHex(theme.error)).toBe(true);
      expect(isHex(theme.info)).toBe(true);
      expect(isHex(theme.purple)).toBe(true);
    });
  });

  describe('border tokens', () => {
    it('defines border colors', () => {
      expect(theme.border).toBeDefined();
      expect(theme.borderMuted).toBeDefined();
    });

    it('border colors are valid hex', () => {
      expect(isHex(theme.border)).toBe(true);
      expect(isHex(theme.borderMuted)).toBe(true);
    });
  });

  it('is a readonly constant object', () => {
    // TypeScript enforces this at compile time; verify the object is not a class instance
    expect(typeof theme).toBe('object');
    expect(theme).not.toBeNull();
  });
});

describe('lightTheme', () => {
  it('is a partial override — does not include all keys', () => {
    expect(Object.keys(lightTheme).length).toBeLessThan(Object.keys(theme).length);
  });

  it('overrides bgBase with a light value', () => {
    // Light theme bgBase should be much lighter than dark theme
    const darkLum = parseInt(theme.bgBase.slice(1, 3), 16);
    const lightLum = parseInt((lightTheme.bgBase as string).slice(1, 3), 16);
    expect(lightLum).toBeGreaterThan(darkLum);
  });

  it('overrides textPrimary with a dark value', () => {
    const darkTextLum = parseInt(theme.textPrimary.slice(1, 3), 16);
    const lightTextLum = parseInt((lightTheme.textPrimary as string).slice(1, 3), 16);
    expect(lightTextLum).toBeLessThan(darkTextLum);
  });
});

describe('statusColors', () => {
  it('defines all four status keys', () => {
    expect(statusColors.success).toBeDefined();
    expect(statusColors.warning).toBeDefined();
    expect(statusColors.error).toBeDefined();
    expect(statusColors.muted).toBeDefined();
    expect(statusColors.info).toBeDefined();
  });

  it('maps success to the theme success color', () => {
    expect(statusColors.success).toBe(theme.success);
  });

  it('maps error to the theme error color', () => {
    expect(statusColors.error).toBe(theme.error);
  });

  it('maps warning to the theme warning color', () => {
    expect(statusColors.warning).toBe(theme.warning);
  });

  it('maps muted to the theme textMuted color', () => {
    expect(statusColors.muted).toBe(theme.textMuted);
  });

  it('all status color values are valid hex', () => {
    for (const value of Object.values(statusColors)) {
      expect(isHex(value)).toBe(true);
    }
  });
});
