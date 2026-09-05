import { cva, type VariantProps } from "class-variance-authority";

// Split out so the Button.tsx module only exports a React component —
// keeps react-refresh's HMR boundary clean and lets non-Button call-
// sites (e.g. a styled <Link asChild>) re-use the same variant matrix.
export const buttonStyles = cva(
  "inline-flex select-none items-center justify-center gap-2 rounded font-medium transition " +
    "focus-visible:outline-none focus-visible:shadow-focus-ring " +
    "disabled:cursor-not-allowed disabled:opacity-50",
  {
    variants: {
      variant: {
        primary:
          "bg-accent text-fg-inverse hover:bg-accent-hover active:bg-accent-active",
        secondary:
          "bg-surface-raised text-fg-default border border-border-default " +
          "hover:border-border-strong hover:bg-surface-overlay",
        ghost:
          "bg-transparent text-fg-muted hover:bg-surface-overlay hover:text-fg-default",
        subtle:
          "bg-accent-subtle text-accent hover:bg-accent/20 hover:text-accent-hover",
      },
      size: {
        sm: "h-7 px-2.5 text-xs",
        md: "h-9 px-3.5 text-sm",
        lg: "h-11 px-5 text-base",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export type ButtonVariants = VariantProps<typeof buttonStyles>;
