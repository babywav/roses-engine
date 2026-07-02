import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Em dev, /api e proxyado para o backend Go (porta 8080).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
