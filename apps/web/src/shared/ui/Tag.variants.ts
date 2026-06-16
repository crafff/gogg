import { cva, type VariantProps } from "class-variance-authority";

export const tagStyles = cva(
  "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium leading-none",
  {
    variants: {
      tone: {
        neutral: "bg-surface-raised text-fg-muted border border-border-default",
        accent: "bg-accent-subtle text-accent",
        challenger: "bg-tier-challenger/15 text-tier-challenger",
        grandmaster: "bg-tier-grandmaster/15 text-tier-grandmaster",
        master: "bg-tier-master/15 text-tier-master",
      },
      size: {
        sm: "px-1.5 py-0 text-[10px]",
        md: "px-2 py-0.5 text-xs",
      },
    },
    defaultVariants: { tone: "neutral", size: "md" },
  },
);

export type TagVariants = VariantProps<typeof tagStyles>;
