import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

const target = process.env.CONTROLPLANE_URL || "http://localhost:8081";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/v1": { target, changeOrigin: true },
      "/healthz": { target, changeOrigin: true },
    },
  },
});
