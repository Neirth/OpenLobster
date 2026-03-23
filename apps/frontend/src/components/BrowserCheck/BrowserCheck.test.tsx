// Copyright (c) OpenLobster contributors. See LICENSE for details.
// Removed unused eslint-disable

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render } from "@solidjs/testing-library";

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

// Ensure CSS.supports exists in happy-dom before any tests run
if (!globalThis.CSS) {
  (globalThis as unknown as Record<string, unknown>).CSS = {};
}
if (!(globalThis.CSS as Record<string, unknown>).supports) {
  (globalThis.CSS as Record<string, unknown>).supports = () => true;
}

import BrowserCheck from "./BrowserCheck";

describe("BrowserCheck Component", () => {
  const originalSupports = (globalThis as unknown as Record<string, any>).CSS?.supports;

  beforeEach(() => {
    // Reset CSS.supports to a default (compatible) function before each test
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => true }),
      configurable: true,
    });
  });

  afterEach(() => {
    // Restore original supports implementation if present
    if (originalSupports) {
      Object.defineProperty(globalThis, 'CSS', {
        value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: originalSupports }),
        configurable: true,
      });
    }
  });

  it("renders children when browser is compatible", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => true }),
      configurable: true,
    });
    const { getByText } = render(() => (
      <BrowserCheck>
        <div>Compatible content</div>
      </BrowserCheck>
    ));
    expect(getByText("Compatible content")).toBeTruthy();
  });

  it("does not render incompatibility message when browser is compatible", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => true }),
      configurable: true,
    });
    const { container } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".browser-check")).toBeNull();
  });

  it("renders browser-check container when CSS grid not supported", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => false }),
      configurable: true,
    });
    const { container } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".browser-check")).toBeTruthy();
  });

  it("renders browser-check__content when incompatible", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => false }),
      configurable: true,
    });
    const { container } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".browser-check__content")).toBeTruthy();
  });

  it("renders title via t() when incompatible", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => false }),
      configurable: true,
    });
    const { getByText } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(getByText("browser.outdated.title")).toBeTruthy();
  });

  it("renders message via t() when incompatible", () => {
    (globalThis as unknown as Record<string, any>).CSS.supports = () => false;
    const { getByText } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(getByText("browser.outdated.message")).toBeTruthy();
  });

  it("renders features text via t() when incompatible", () => {
    (globalThis as unknown as Record<string, any>).CSS.supports = () => false;
    const { getByText } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(getByText("browser.outdated.features")).toBeTruthy();
  });

  it("hides children when browser is incompatible", () => {
    // Use defineProperty to ensure the mock is applied in happy-dom
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => false }),
      configurable: true,
    });
    const { container } = render(() => (
      <BrowserCheck>
        <div class="my-child">Hidden content</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".my-child")).toBeNull();
  });

  it("renders multiple children correctly in compatible browser", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => true }),
      configurable: true,
    });
    const { getByText } = render(() => (
      <BrowserCheck>
        <>
          <div>First child</div>
          <div>Second child</div>
        </>
      </BrowserCheck>
    ));
    expect(getByText("First child")).toBeTruthy();
    expect(getByText("Second child")).toBeTruthy();
  });

  it("title element has correct class", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => false }),
      configurable: true,
    });
    const { container } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".browser-check__title")).toBeTruthy();
  });

  it("message element has correct class", () => {
    Object.defineProperty(globalThis, 'CSS', {
      value: Object.assign({}, (globalThis as unknown as Record<string, any>).CSS || {}, { supports: () => false }),
      configurable: true,
    });
    const { container } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".browser-check__message")).toBeTruthy();
  });

  it("features element has correct class", () => {
    (globalThis as unknown as Record<string, any>).CSS.supports = () => false;
    const { container } = render(() => (
      <BrowserCheck>
        <div>child</div>
      </BrowserCheck>
    ));
    expect(container.querySelector(".browser-check__features")).toBeTruthy();
  });
});
