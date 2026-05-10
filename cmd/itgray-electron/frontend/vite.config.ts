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
      // Phase 2 wails-shim: redirect Wails-generated import paths to the
      // shim modules, which in turn route through window.itg.
      "../../wailsjs/runtime/runtime": path.resolve(__dirname, "wails-shim/runtime"),
      "../../wailsjs/go/models": path.resolve(__dirname, "wails-shim/models"),
      "../../wailsjs/go/bindings/AppService": path.resolve(__dirname, "wails-shim/bindings/AppService"),
      "../../wailsjs/go/bindings/RunService": path.resolve(__dirname, "wails-shim/bindings/RunService"),
      "../../wailsjs/go/bindings/ServersService": path.resolve(__dirname, "wails-shim/bindings/ServersService"),
      "../../wailsjs/go/bindings/SubsService": path.resolve(__dirname, "wails-shim/bindings/SubsService"),
      "../../wailsjs/go/bindings/SettingsService": path.resolve(__dirname, "wails-shim/bindings/SettingsService"),
      "../../wailsjs/go/bindings/HelperService": path.resolve(__dirname, "wails-shim/bindings/HelperService"),
      "../../wailsjs/go/bindings/OnboardingService": path.resolve(__dirname, "wails-shim/bindings/OnboardingService"),
    },
  },
  server: { port: 34115, strictPort: true },
  build: {
    outDir: "../dist-frontend",
    emptyOutDir: true,
  },
});
