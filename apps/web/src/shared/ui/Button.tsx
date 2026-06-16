import { forwardRef } from "react";
import { Slot } from "@radix-ui/react-slot";

import { cn } from "@shared/lib/cn";

import { buttonStyles, type ButtonVariants } from "./Button.variants";

// Variant matrix kept deliberately narrow:
//
//   primary  — main CTA, accent gold on raised surface
//   secondary— neutral fill for confirm/cancel pairs
//   ghost    — borderless, used inside dense table rows
//   subtle   — accent-tinted bg, no border (chip-like)
//
// Sizes mirror the rankings table density: sm for inline filter
// triggers, md for default, lg for hero CTAs (login / search submit).

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>, ButtonVariants {
  asChild?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, type, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        // type defaults to "button" so nesting in <form> doesn't
        // accidentally submit. Callers opt into "submit" explicitly.
        // When asChild swaps in an <a> or Radix Trigger, the type
        // attribute is harmless on those elements.
        type={asChild ? undefined : (type ?? "button")}
        ref={ref}
        className={cn(buttonStyles({ variant, size }), className)}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";
