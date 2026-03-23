// Test setup: polyfills and global test helpers for happy-dom
// Polyfill a minimal Popover API used by ContextMenu component.

if (typeof window !== "undefined") {
  interface PopoverProto extends HTMLElement {
    __popoverOpen?: boolean;
    dataset?: { popoverOpen?: string };
    showPopover?: () => void;
    hidePopover?: () => void;
  }
  const proto = (window as unknown as { HTMLElement?: { prototype?: PopoverProto } }).HTMLElement?.prototype;
  if (proto) {
    if (!proto.showPopover) {
      proto.showPopover = function showPopover() {
        // mark as open
        try {
          this.__popoverOpen = true;
          // reflect to dataset for matches polyfill
          if (this.dataset) this.dataset.popoverOpen = "true";
        } catch {
          // ignore
        }
      };
    }

    if (!proto.hidePopover) {
      proto.hidePopover = function hidePopover() {
        try {
          this.__popoverOpen = false;
          if (this.dataset) this.dataset.popoverOpen = "false";
        } catch {
          // Ignore error
        }
      };
    }

    // Patch matches to understand ':popover-open' pseudo
    const originalMatches = proto.matches;
    proto.matches = function matches(this: PopoverProto, selector: string) {
      if (selector === ":popover-open") {
        try {
          if (this.__popoverOpen !== undefined) return !!this.__popoverOpen;
          if (this.dataset && this.dataset.popoverOpen !== undefined) {
            return this.dataset.popoverOpen === "true";
          }
          return false;
        } catch {
          return false;
        }
      }
      return originalMatches.call(this, selector);
    };
  }
}

export {};
