import { describe, it, expect, beforeEach, vi } from 'vitest';

// Mock router and query client to avoid importing runtime JSX modules
vi.mock('@solidjs/router', () => ({
  Router: () => null,
  Route: () => null,
  useLocation: () => ({ pathname: '/' }),
}));

vi.mock('@tanstack/solid-query', () => ({
  useQueryClient: () => ({ invalidateQueries: () => {} }),
}));

// These tests import the module after stubbing globals to exercise
// the module-level `detectBrowserLocale` behavior and the exported
// `recheckConfig` function.

describe('App locale and recheckConfig', () => {
  beforeEach(() => {
    // reset module cache so imports re-run module-level code
    vi.resetModules();
    // ensure fetch is stubbed / restored between tests
    vi.restoreAllMocks();
  });

  it('detects zh locale from navigator.language', async () => {
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: { language: 'zh-CN' },
    });

    const mod = await import('./App');
    // exported `locale` is a signal getter
    const { locale } = mod as any;
    expect(locale()).toBe('zh');
  });

  it('detects es locale from navigator.language', async () => {
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: { language: 'es-ES' },
    });

    const mod = await import('./App');
    const { locale } = mod as any;
    expect(locale()).toBe('es');
  });

  it('falls back to en for unknown language', async () => {
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: { language: 'fr-FR' },
    });

    const mod = await import('./App');
    const { locale } = mod as any;
    expect(locale()).toBe('en');
  });

  it('falls back to en when navigator is undefined', async () => {
    // remove navigator if present
    Object.defineProperty(globalThis, 'navigator', {
      configurable: true,
      value: undefined,
    });

    const mod = await import('./App');
    const { locale } = mod as any;
    expect(locale()).toBe('en');
  });

  it('recheckConfig sets showWizard true on fetch error', async () => {
    // Ensure fetch throws
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('network'))));

    const mod = await import('./App');
    const { recheckConfig, setShowWizard, configLoaded, showWizard } = mod as any;

    // reset showWizard to false then call recheckConfig to see it flip back to true on error
    setShowWizard(false);
    expect(showWizard()).toBe(false);

    await recheckConfig();

    expect(configLoaded()).toBe(true);
    expect(showWizard()).toBe(true);

    // restore stubbed fetch
    vi.unstubAllGlobals();
  });

  it('recheckConfig hides wizard when API reports completed', async () => {
    const mockResp = {
      json: () => Promise.resolve({ data: { config: { wizardCompleted: true } } }),
    };
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(mockResp)));

    const mod = await import('./App');
    const { recheckConfig, setShowWizard, showWizard, configLoaded } = mod as any;

    setShowWizard(true);
    expect(showWizard()).toBe(true);

    await recheckConfig();

    expect(configLoaded()).toBe(true);
    expect(showWizard()).toBe(false);

    vi.unstubAllGlobals();
  });
});
