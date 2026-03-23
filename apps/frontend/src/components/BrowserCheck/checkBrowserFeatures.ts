export function checkBrowserFeatures(): boolean {
  try {
    const hasProxy = typeof Proxy !== "undefined";

    const hasAsync = (function () {
      try {
        return (async function () {}).constructor.name === "AsyncFunction";
      } catch {
        return false;
      }
    })();

    const hasFetch = typeof fetch !== "undefined";

    const cssObj = (globalThis as unknown as { CSS?: { supports?: (prop: string, value: string) => boolean } }).CSS;
    // If the CSS.supports API is available, prefer its result — tests mock
    // this function to simulate browser capabilities. Otherwise fall back
    // to the broader feature checks.
    if (typeof cssObj !== "undefined" && typeof cssObj.supports === "function") {
      // eslint-disable-next-line no-console
      console.debug("BrowserCheck: cssObj.supports exists, calling supports");
      const hasGrid = cssObj.supports("display", "grid");
      // eslint-disable-next-line no-console
      console.debug("BrowserCheck: supports(display,grid) ->", hasGrid);
      const hasCSSVariables = cssObj.supports("color", "var(--test)");
      // eslint-disable-next-line no-console
      console.debug("BrowserCheck: supports(color,var(--test)) ->", hasCSSVariables);
      return hasProxy && hasAsync && hasFetch && hasGrid && hasCSSVariables;
    }

    const hasES6 = (function () {
      try {
        return [1, 2].map((x) => x).length === 2;
      } catch {
        return false;
      }
    })();

    return hasProxy && hasAsync && hasFetch && hasES6;
  } catch {
    return false;
  }
}
