// Test setup: polyfills and global test helpers for happy-dom
// Polyfill a minimal Popover API used by ContextMenu component.

if (typeof window !== "undefined") {
  // Cast to `any` to avoid conflicts with lib.dom.d.ts, which in modern TypeScript
  // declares showPopover/hidePopover as non-optional on HTMLElement. The polyfill
  // only installs the methods when happy-dom omits them at runtime.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const proto = HTMLElement.prototype as any;

  type PopoverEl = HTMLElement & { __popoverOpen?: boolean };

  if (!proto.showPopover) {
    proto.showPopover = function (this: PopoverEl) {
      try {
        this.__popoverOpen = true;
        if (this.dataset) this.dataset["popoverOpen"] = "true";
      } catch {
        // ignore
      }
    };
  }

  if (!proto.hidePopover) {
    proto.hidePopover = function (this: PopoverEl) {
      try {
        this.__popoverOpen = false;
        if (this.dataset) this.dataset["popoverOpen"] = "false";
      } catch {
        // ignore
      }
    };
  }

  // Patch matches to understand ':popover-open' pseudo-class.
  const originalMatches = HTMLElement.prototype.matches;
  proto.matches = function (this: PopoverEl, selector: string): boolean {
    if (selector === ":popover-open") {
      try {
        if (this.__popoverOpen !== undefined) return !!this.__popoverOpen;
        if (this.dataset && this.dataset["popoverOpen"] !== undefined) {
          return this.dataset["popoverOpen"] === "true";
        }
        return false;
      } catch {
        return false;
      }
    }
    return originalMatches.call(this, selector);
  };
}

export {};
