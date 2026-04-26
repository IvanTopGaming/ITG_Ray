import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        "surface-base": "#0a0d17",
        "accent-primary": "#6366f1",
        "accent-secondary": "#ec4899",
        "text-primary": "#e8ebf3",
        "text-secondary": "#9aa4bf",
        "text-muted": "#7b83a0",
      },
      borderRadius: {
        sm: "6px", md: "10px", lg: "14px", xl: "18px", "2xl": "22px",
      },
    },
  },
  plugins: [],
} satisfies Config;
