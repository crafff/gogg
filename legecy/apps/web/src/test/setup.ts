import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Auto-unmount React trees between tests so DOM queries can't leak
// state into the next test. testing-library does this automatically
// in Jest via its setup but vitest needs the hook wired by hand.
afterEach(() => {
  cleanup();
});
