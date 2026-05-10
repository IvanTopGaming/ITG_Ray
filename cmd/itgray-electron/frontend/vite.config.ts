import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  // Relative asset paths so the bundle works under Electron's file:// loader
  // in production. Without this Vite emits absolute /assets/... URLs that
  // resolve to filesystem root and fail to load (white-screen renderer).
  base: "./",
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: { port: 34115, strictPort: true },
  build: {
    outDir: "../dist-frontend",
    emptyOutDir: true,
  },
});
