import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  resolve: {
    dedupe: ["graphql"],
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
      "/graphql": "http://localhost:8080",
      "/oauth": "http://localhost:8080",
      "/ws": { target: "ws://localhost:8080", ws: true },
      "/health": "http://localhost:8080",
      "/metrics": "http://localhost:8080",
      "/logs": "http://localhost:8080",
    },
  },
});
