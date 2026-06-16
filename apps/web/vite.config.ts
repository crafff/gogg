import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Vite config for the Phase D rewrite of the frontend.
//
// Proxy targets match the Phase B gogg-api binary on :8080 — both the
// REST compat layer (/api/v1/*) and the GraphQL endpoint (/graphql)
// pass through unmodified, so dev `npm run dev` works against a local
// `make run-api` without CORS gymnastics.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@app": path.resolve(__dirname, "./src/app"),
      "@features": path.resolve(__dirname, "./src/features"),
      "@shared": path.resolve(__dirname, "./src/shared"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true },
      "/graphql": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
});
