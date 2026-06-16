import type { Config } from "tailwindcss";

// Phase D chunk 1 ships a minimal token set so smoke verification can
// confirm tailwind is wired. The full design system (per-tier colors,
// rank badge gradients, champion portrait sizes, semantic spacing)
// arrives in chunk 2 alongside the base UI components.
const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // Brand placeholder: gogg-gold echoes the LoL UI tone, but
        // these will be replaced by semantic tokens (surface/fg/accent)
        // in chunk 2 once the dark + light themes ship.
        "gogg-gold": "#c8aa6e",
        "gogg-ink": "#0a1428",
      },
      fontFamily: {
        sans: [
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "PingFang SC",
          "Hiragino Sans GB",
          "Microsoft YaHei",
          "sans-serif",
        ],
      },
    },
  },
  plugins: [],
};

export default config;
