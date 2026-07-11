import type { Config } from "tailwindcss";
import plugin from "tailwindcss/plugin";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: [
          "Inter",
          "ui-sans-serif",
          "system-ui",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "sans-serif",
        ],
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "Consolas", "monospace"],
      },
      colors: {
        accent: { start: "#7ed4ff", mid: "#3478e0", end: "#1a3a99" },
        bg: { 0: "#0a0620", 1: "#150a3d", 2: "#3a2580" },
        success: "#00e676",
        warn: "#ffb13c",
        danger: "#ff5e5e",
      },
      backgroundImage: {
        "app-bg":
          "radial-gradient(130% 90% at 50% 0%, #3a2580 0%, #150a3d 55%, #0a0620 100%)",
        "orb-accent":
          "radial-gradient(circle at 30% 30%, #7ed4ff, #3478e0 60%, #1a3a99 100%)",
        "orb-warn":
          "radial-gradient(circle at 30% 30%, #ffd28a, #ffb13c 60%, #a36a00 100%)",
        "orb-danger":
          "radial-gradient(circle at 30% 30%, #ff8a8a, #ff5e5e 60%, #7a1f1f 100%)",
        "btn-accent":
          "linear-gradient(180deg, #7ed4ff 0%, #3478e0 100%)",
      },
      animation: {
        "orb-pulse": "orb-pulse 2.5s ease-in-out infinite",
        "orb-shake": "orb-shake 0.4s ease-in-out 2",
        "orb-breathe": "orb-breathe 2.8s ease-in-out infinite",
        "spin-slow": "spin 2s linear infinite",
      },
      keyframes: {
        "orb-pulse": {
          "0%, 100%": { transform: "scale(1)" },
          "50%": { transform: "scale(1.06)" },
        },
        "orb-shake": {
          "0%, 100%": { transform: "translateX(0)" },
          "25%": { transform: "translateX(-6px)" },
          "75%": { transform: "translateX(6px)" },
        },
        "orb-breathe": {
          "0%, 100%": { transform: "scale(1)" },
          "50%": { transform: "scale(1.025)" },
        },
      },
      transitionTimingFunction: {
        snap: "cubic-bezier(0.16, 1, 0.3, 1)",
      },
      transitionDuration: {
        instant: "120ms",
        standard: "240ms",
      },
    },
  },
  plugins: [
    plugin(({ addUtilities }) => {
      addUtilities({
        ".glass-dim": {
          background: "rgba(255,255,255,0.05)",
          "backdrop-filter": "blur(20px)",
          "-webkit-backdrop-filter": "blur(20px)",
          "border-color": "rgba(255,255,255,0.12)",
          "border-width": "1px",
        },
        ".glass-regular": {
          background: "rgba(255,255,255,0.09)",
          "backdrop-filter": "blur(28px)",
          "-webkit-backdrop-filter": "blur(28px)",
          "border-color": "rgba(255,255,255,0.20)",
          "border-width": "1px",
        },
        ".glass-elevated": {
          background: "rgba(255,255,255,0.15)",
          "backdrop-filter": "blur(36px)",
          "-webkit-backdrop-filter": "blur(36px)",
          "border-color": "rgba(255,255,255,0.30)",
          "border-width": "1px",
          "box-shadow": "0 24px 48px -12px rgba(0,0,0,0.4)",
        },
      });
    }),
  ],
} satisfies Config;
