import { forwardRef } from "react";

import { cn } from "@shared/lib/cn";

// Skeleton wraps the `animate-skeleton` tailwind keyframe in a tiny
// component so call-sites carry semantic intent instead of stray
// animation classes. Width/height come from caller via className.
export type SkeletonProps = React.HTMLAttributes<HTMLDivElement>;

export const Skeleton = forwardRef<HTMLDivElement, SkeletonProps>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      role="status"
      aria-live="polite"
      aria-label="loading"
      className={cn(
        "block animate-skeleton rounded bg-surface-raised",
        className,
      )}
      {...props}
    />
  ),
);
Skeleton.displayName = "Skeleton";
