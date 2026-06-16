import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useInfiniteScroll } from "./useInfiniteScroll";

// IntersectionObserver isn't part of jsdom. The hook only needs four
// pieces of the surface — constructor, observe, disconnect, and a way
// to push entries into the callback — so we fake exactly those and
// capture the constructor call for assertions.
type EntryShape = { isIntersecting: boolean };

interface ObserverHandle {
  callback: (entries: EntryShape[]) => void;
  observe: ReturnType<typeof vi.fn>;
  disconnect: ReturnType<typeof vi.fn>;
  options: IntersectionObserverInit | undefined;
}

let lastObserver: ObserverHandle | null = null;

beforeEach(() => {
  lastObserver = null;
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe: ReturnType<typeof vi.fn>;
      disconnect: ReturnType<typeof vi.fn>;
      constructor(
        cb: (entries: EntryShape[]) => void,
        options?: IntersectionObserverInit,
      ) {
        this.observe = vi.fn();
        this.disconnect = vi.fn();
        lastObserver = {
          callback: cb,
          observe: this.observe,
          disconnect: this.disconnect,
          options,
        };
      }
    },
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

interface ProbeProps {
  onLoadMore: () => void;
  enabled: boolean;
  rootMargin?: string;
}

function Probe({ onLoadMore, enabled, rootMargin }: ProbeProps) {
  const ref = useInfiniteScroll<HTMLDivElement>({
    onLoadMore,
    enabled,
    rootMargin,
  });
  return <div ref={ref} data-testid="sentinel" />;
}

describe("useInfiniteScroll", () => {
  it("does not attach an observer when disabled", () => {
    const onLoadMore = vi.fn();
    render(<Probe onLoadMore={onLoadMore} enabled={false} />);
    expect(lastObserver).toBeNull();
  });

  it("attaches an observer with the configured rootMargin", () => {
    const onLoadMore = vi.fn();
    render(
      <Probe onLoadMore={onLoadMore} enabled={true} rootMargin="100px 0px" />,
    );
    expect(lastObserver).not.toBeNull();
    expect(lastObserver?.observe).toHaveBeenCalledTimes(1);
    expect(lastObserver?.options?.rootMargin).toBe("100px 0px");
  });

  it("fires onLoadMore when the observer reports intersection", () => {
    const onLoadMore = vi.fn();
    render(<Probe onLoadMore={onLoadMore} enabled={true} />);
    expect(lastObserver).not.toBeNull();

    // Push a canned intersection entry into the observer callback.
    lastObserver!.callback([{ isIntersecting: true }]);
    expect(onLoadMore).toHaveBeenCalledTimes(1);
  });

  it("does not fire onLoadMore for non-intersecting entries", () => {
    const onLoadMore = vi.fn();
    render(<Probe onLoadMore={onLoadMore} enabled={true} />);
    lastObserver!.callback([{ isIntersecting: false }]);
    expect(onLoadMore).not.toHaveBeenCalled();
  });

  it("disconnects when unmounted", () => {
    const onLoadMore = vi.fn();
    const { unmount } = render(
      <Probe onLoadMore={onLoadMore} enabled={true} />,
    );
    unmount();
    expect(lastObserver?.disconnect).toHaveBeenCalledTimes(1);
  });
});
