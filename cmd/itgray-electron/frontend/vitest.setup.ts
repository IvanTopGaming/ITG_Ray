import "@testing-library/jest-dom/vitest";
import "./src/i18n";

// Node 25 ships a native `localStorage` that leaks into the test global and
// shadows jsdom's fully-functional implementation.  Explicitly re-bind it so
// tests can call .clear(), .setItem(), etc. as expected.
// See: https://github.com/vitest-dev/vitest/issues/4288
const jsdom = (globalThis as { jsdom?: { window: Window } }).jsdom;
if (jsdom) {
  const jsdomWindow = jsdom.window;
  Object.defineProperty(globalThis, "localStorage", {
    value: jsdomWindow.localStorage,
    writable: true,
    configurable: true,
  });
  Object.defineProperty(globalThis, "sessionStorage", {
    value: jsdomWindow.sessionStorage,
    writable: true,
    configurable: true,
  });
}
