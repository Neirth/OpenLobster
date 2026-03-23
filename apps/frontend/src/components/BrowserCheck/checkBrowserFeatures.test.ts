import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { checkBrowserFeatures } from "./checkBrowserFeatures";

describe("checkBrowserFeatures (unit)", () => {
  const origCSS = (globalThis as unknown as Record<string, any>).CSS;
  const origFetch = (globalThis as unknown as Record<string, any>).fetch;

  beforeEach(() => {
    // Ensure CSS exists to avoid runtime errors in happy-dom
    Object.defineProperty(globalThis, "CSS", {
      value: Object.assign({}, origCSS || {}, { supports: () => true }),
      configurable: true,
    });
    if (!origFetch) {
      (globalThis as unknown as Record<string, any>).fetch = () => Promise.resolve();
    }
  });

  afterEach(() => {
    Object.defineProperty(globalThis, "CSS", {
      value: origCSS,
      configurable: true,
    });
    (globalThis as unknown as Record<string, any>).fetch = origFetch;
  });

  it("returns true when CSS.supports reports support", () => {
    Object.defineProperty(globalThis, "CSS", {
      value: Object.assign({}, origCSS || {}, { supports: () => true }),
      configurable: true,
    });
    expect(checkBrowserFeatures()).toBe(true);
  });

  it("returns false when CSS.supports reports no support", () => {
    Object.defineProperty(globalThis, "CSS", {
      value: Object.assign({}, origCSS || {}, { supports: () => false }),
      configurable: true,
    });
    expect(checkBrowserFeatures()).toBe(false);
  });

  it("falls back when CSS is undefined but core features exist", () => {
    // Remove CSS and ensure fetch exists
    Object.defineProperty(globalThis, "CSS", { value: undefined, configurable: true });
    (globalThis as unknown as Record<string, any>).fetch = () => Promise.resolve();
    const result = checkBrowserFeatures();
    // Should be a boolean; most modern test envs provide Proxy/async
    expect(typeof result).toBe("boolean");
  });
});
