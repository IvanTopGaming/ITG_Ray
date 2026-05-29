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
  // Bind IPv4 loopback explicitly: on hosts where `localhost` resolves to
  // ::1 first, Vite would otherwise listen only on IPv6, and Electron's
  // loadURL("http://localhost") / the dev:electron wait-on step both hit
  // 127.0.0.1 → ECONNREFUSED → white-screen renderer + hung startup.
  server: { host: "127.0.0.1", port: 34115, strictPort: true },
  build: {
    outDir: "../dist-frontend",
    emptyOutDir: true,
  },
});
