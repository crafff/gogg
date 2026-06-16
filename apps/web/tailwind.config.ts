import type { Config } from "tailwindcss";

// Design tokens are split into two layers:
//
// 1. Brand primitives — gogg-gold / gogg-ink — match the LoL UI tone
//    and remain so legacy chunk 1 markup keeps rendering until chunk 5
//    swaps the shell.
// 2. Semantic tokens — surface-*, fg-*, border-*, accent-* — are the
//    layer every new component speaks. They map onto the brand
//    primitives today but can be re-skinned (e.g. a light theme, or
//    the upcoming public/anonymous variant) without touching the
//    components themselves.
//
// Tier colors are intentionally NOT exposed as semantic tokens — they
// are domain-specific (rankings, summoner badges) and live under the
// `tier-*` namespace so the autocomplete list stays meaningful.
const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        "gogg-gold": "#c8aa6e",
        "gogg-ink": "#0a1428",
        surface: {
          DEFAULT: "#0a1428",
          raised: "#0f1b34",
          sunken: "#070e1c",
          overlay: "#152544",
        },
        fg: {
          DEFAULT: "#e6e8ed",
          default: "#e6e8ed",
          muted: "#a3aab8",
          subtle: "#6c7689",
          inverse: "#0a1428",
        },
        border: {
          DEFAULT: "#1f2a44",
          default: "#1f2a44",
          strong: "#2e3c5c",
          accent: "#c8aa6e66",
        },
        accent: {
          DEFAULT: "#c8aa6e",
          hover: "#d6bd86",
          active: "#b89856",
          subtle: "#c8aa6e1a",
        },
        tier: {
          challenger: "#f4c874",
          grandmaster: "#d24a4a",
          master: "#9c5bd8",
          diamond: "#5acce0",
          emerald: "#33c270",
          platinum: "#5990c2",
          gold: "#c89b3c",
          silver: "#9aa3ad",
          bronze: "#a07246",
          iron: "#564a40",
        },
      },
      backgroundImage: {
        "tier-challenger": "linear-gradient(135deg, #f4c874 0%, #c8aa6e 100%)",
        "tier-grandmaster": "linear-gradient(135deg, #ff7a7a 0%, #b22a2a 100%)",
        "tier-master": "linear-gradient(135deg, #b985e8 0%, #7438b0 100%)",
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
        mono: [
          "ui-monospace",
          "SFMono-Regular",
          "Menlo",
          "Consolas",
          "monospace",
        ],
      },
      borderRadius: {
        DEFAULT: "0.375rem",
        lg: "0.5rem",
        xl: "0.75rem",
      },
      boxShadow: {
        card: "0 1px 2px rgba(0,0,0,0.4), 0 4px 12px rgba(0,0,0,0.25)",
        "focus-ring": "0 0 0 2px #c8aa6eaa",
      },
      transitionTimingFunction: {
        "out-soft": "cubic-bezier(0.22, 1, 0.36, 1)",
      },
      keyframes: {
        skeleton: {
          "0%, 100%": { opacity: "0.6" },
          "50%": { opacity: "0.3" },
        },
      },
      animation: {
        skeleton: "skeleton 1.4s ease-in-out infinite",
      },
    },
  },
  plugins: [],
};

export default config;
