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
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api/v1": {
        target: "http://localhost:8080",
        changeOrigin: true,
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
  },
});
