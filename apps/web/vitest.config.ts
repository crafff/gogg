import { defineConfig, mergeConfig } from "vitest/config";

import viteConfig from "./vite.config";

// Vitest reuses the same alias/plugin setup as vite (so `@shared/...`
// resolves identically) and layers jsdom on top. The setup file wires
// up testing-library's matchers and the cleanup-after-each-test hook.
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      globals: true,
      setupFiles: ["./src/test/setup.ts"],
      include: ["src/**/*.{test,spec}.{ts,tsx}"],
      // Bail tests fast in CI when a previous test left global state
      // dirty — easier to debug than a downstream cascade.
      isolate: true,
      restoreMocks: true,
    },
  }),
);
