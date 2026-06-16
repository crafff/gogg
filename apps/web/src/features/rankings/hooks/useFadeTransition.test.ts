import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useFadeTransition } from "./useFadeTransition";

describe("useFadeTransition", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts in the shown phase", () => {
    const { result } = renderHook(() =>
      useFadeTransition({ fadeOutMs: 100, fadeInMs: 100 }),
    );
    expect(result.current.phase).toBe("shown");
    expect(result.current.lockedHeight).toBeNull();
    expect(result.current.isTransitioning).toBe(false);
  });

  it("walks shown → fading-out → hidden on beginExit + timer", () => {
    const { result } = renderHook(() =>
      useFadeTransition({ fadeOutMs: 100, fadeInMs: 100 }),
    );

    act(() => {
      result.current.beginExit();
    });
    expect(result.current.phase).toBe("fading-out");
    expect(result.current.isTransitioning).toBe(true);

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current.phase).toBe("hidden");
  });

  it("walks hidden → fading-in → shown on beginEnter + timer", () => {
    const { result } = renderHook(() =>
      useFadeTransition({ fadeOutMs: 50, fadeInMs: 100 }),
    );

    act(() => {
      result.current.beginExit();
      vi.advanceTimersByTime(50);
    });
    expect(result.current.phase).toBe("hidden");

    act(() => {
      result.current.beginEnter();
    });
    expect(result.current.phase).toBe("fading-in");

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current.phase).toBe("shown");
    expect(result.current.lockedHeight).toBeNull();
  });

  it("ignores beginEnter when not in the hidden phase", () => {
    const { result } = renderHook(() =>
      useFadeTransition({ fadeOutMs: 100, fadeInMs: 100 }),
    );

    // Calling beginEnter() from "shown" is a no-op.
    act(() => {
      result.current.beginEnter();
    });
    expect(result.current.phase).toBe("shown");
  });
});
