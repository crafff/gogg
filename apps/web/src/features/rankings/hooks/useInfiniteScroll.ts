import { useEffect, useRef, type RefObject } from "react";

export interface UseInfiniteScrollOptions {
  /** Fired when the sentinel becomes ≥1px visible inside the rootMargin. */
  onLoadMore: () => void;
  /** When false the observer is detached — pause during animations, errors, end-of-list. */
  enabled: boolean;
  /** rootMargin passed to IntersectionObserver; defaults to "220px 0px". */
  rootMargin?: string;
}

/**
 * Thin IntersectionObserver helper used by the rankings infinite
 * scroller. The sentinel element is whatever the caller assigns the
 * returned ref to — typically an empty <div> at the bottom of the
 * list. The hook handles attach/detach + rerunning when `enabled`
 * flips so the parent doesn't have to thread observer state.
 */
export function useInfiniteScroll<T extends HTMLElement>({
  onLoadMore,
  enabled,
  rootMargin = "220px 0px",
}: UseInfiniteScrollOptions): RefObject<T> {
  // useRef<T>(null) picks the convenience overload that returns
  // RefObject<T> (the variant JSX's ref attribute accepts directly).
  // The explicit return-type annotation pins it so the inference can't
  // widen to RefObject<T | null> at the call site.
  const sentinelRef = useRef<T>(null);

  useEffect(() => {
    if (!enabled) return;
    const node = sentinelRef.current;
    if (!node) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry?.isIntersecting) onLoadMore();
      },
      { root: null, rootMargin, threshold: 0 },
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [enabled, onLoadMore, rootMargin]);

  return sentinelRef;
}
