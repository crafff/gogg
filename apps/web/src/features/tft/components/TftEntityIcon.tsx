import { useState } from "react";

import { cn } from "@shared/lib/cn";

interface TftEntityIconProps {
  iconUrl?: string | null;
  name?: string | null;
  entityId: string;
  size?: "sm" | "md" | "lg";
  className?: string;
}

export function TftEntityIcon({
  iconUrl,
  name,
  entityId,
  size = "md",
  className,
}: TftEntityIconProps) {
  const [failedSource, setFailedSource] = useState<string | null>(null);
  const label = name?.trim() || entityId;
  const showImage = Boolean(iconUrl && failedSource !== iconUrl);

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center overflow-hidden rounded border border-border-strong bg-surface-sunken font-mono text-fg-subtle",
        size === "sm" && "h-7 w-7 text-[9px]",
        size === "md" && "h-10 w-10 text-[10px]",
        size === "lg" && "h-12 w-12 text-xs",
        className,
      )}
      title={label}
    >
      {showImage ? (
        <img
          src={iconUrl ?? undefined}
          alt=""
          loading="lazy"
          className="h-full w-full object-cover"
          onError={() => setFailedSource(iconUrl ?? null)}
        />
      ) : (
        <span aria-hidden="true">{initials(label)}</span>
      )}
      <span className="sr-only">{label}</span>
    </span>
  );
}

function initials(value: string) {
  const compact = value
    .replace(/^TFT\d*_?/i, "")
    .replace(/[^a-z0-9\p{L}]/giu, "")
    .slice(0, 3);
  return compact || "?";
}
