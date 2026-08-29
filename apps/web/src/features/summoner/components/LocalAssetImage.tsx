import { useState, type ImgHTMLAttributes, type ReactNode } from "react";

interface LocalAssetImageProps extends Omit<
  ImgHTMLAttributes<HTMLImageElement>,
  "src"
> {
  src: string;
  fallback?: ReactNode;
}

export function LocalAssetImage({
  src,
  alt = "",
  className = "",
  fallback = "?",
  ...props
}: LocalAssetImageProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const failed = failedSrc === src;

  if (failed) {
    return (
      <span
        className={`inline-flex items-center justify-center bg-surface-sunken text-xs font-medium text-fg-subtle ${className}`}
        role={alt ? "img" : undefined}
        aria-label={alt || undefined}
        aria-hidden={alt ? undefined : true}
      >
        {fallback}
      </span>
    );
  }

  return (
    <img
      {...props}
      src={src}
      alt={alt}
      className={className}
      onError={() => setFailedSrc(src)}
    />
  );
}
