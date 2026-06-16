import { forwardRef } from "react";

import { cn } from "@shared/lib/cn";

import { tagStyles, type TagVariants } from "./Tag.variants";

// Tag is the chip primitive for static metadata: position labels,
// tier badges, version strings. It is NOT interactive — wrap it in a
// Button or Link if you need that.
//
// The `tier` variant set covers the rankings page filter (challenger
// → master). For other tier names a custom className + the `tone:
// neutral` variant is the escape hatch.

export interface TagProps
  extends React.HTMLAttributes<HTMLSpanElement>, TagVariants {}

export const Tag = forwardRef<HTMLSpanElement, TagProps>(
  ({ className, tone, size, ...props }, ref) => (
    <span
      ref={ref}
      className={cn(tagStyles({ tone, size }), className)}
      {...props}
    />
  ),
);
Tag.displayName = "Tag";
