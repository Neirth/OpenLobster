import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import path from "path";

export default defineConfig({
  plugins: [solid()],
  resolve: {
    dedupe: ["graphql", "solid-js", "@solidjs/router", "@tanstack/solid-query"],
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    // Output to the Go embed directory so `go build` bundles the frontend.
    outDir: "../../apps/backend/cmd/openlobster/public/assets",
    emptyOutDir: true,
    assetsDir: ".",
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("solid-js") || id.includes("@solidjs/router")) {
            return "vendor-solid";
          }
          if (id.includes("graphql-request") || id.includes("graphql-ws") || id.includes("graphql")) {
            return "vendor-graphql";
          }
          if (id.includes("markdown-it")) {
            return "vendor-markdown";
          }
        },
      },
    },
  },
  server: {
    proxy: {
      "/graphql": "http://localhost:8081",
      "/oauth": "http://localhost:8081",
      "/ws": { target: "ws://localhost:8081", ws: true },
      "/health": "http://localhost:8081",
      "/metrics": "http://localhost:8081",
      "/logs": "http://localhost:8081",
    },
  },
});
