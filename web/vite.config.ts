/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// During development the React app lives on :5173 (Vite's dev server)
// and the Go API on :8080. We could either:
//   (a) enable CORS on the backend and call http://localhost:8080
//       directly from the browser (current setup, see RouterConfig
//       AllowedOrigins), or
//   (b) reverse-proxy /api/* through Vite so the browser only ever
//       talks to one origin (this block).
//
// Doing both is harmless and gives us the best of both worlds: the
// app works whether we hit it through Vite (dev) or through the
// embedded static-file handler (production single-binary build).
//
// `HOST` is set by `make web-dev HOST=…` (default localhost). Use
// 0.0.0.0 to listen on every interface, or a concrete LAN IP.
//
// Vite runs this config in Node, where `process` exists at runtime. We
// declare it locally instead of pulling in `@types/node` just to type a
// single env lookup — keeping the frontend's dev dependencies lean.
declare const process: { env: Record<string, string | undefined> };
const devHost = process.env.HOST || "localhost";

export default defineConfig({
  plugins: [react()],
  server: {
    host: devHost,
    port: 5173,
    proxy: {
      "/api/v1": {
        target: "http://localhost:8080",
        changeOrigin: true,
        ws: true,
      },
      "/healthz": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
    // Baseline for older Android System WebViews (Motorola G5 / Chrome ~87 era).
    // Avoids emitting syntax that those engines cannot parse.
    target: "chrome87",
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
