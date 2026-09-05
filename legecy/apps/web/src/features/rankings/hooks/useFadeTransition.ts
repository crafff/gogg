import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type RefObject,
} from "react";

/**
 * Phase of the fade transition used while the rankings list swaps to a
 * new filter slice. The full cycle is:
 *
 *   shown → fading-out → hidden → (data arrives) → fading-in → shown
 *
 * `hidden` is the window in which the new query is in flight; the
 * viewport keeps its `lockedHeight` so the scroll position above the
 * list doesn't jump while the table re-renders.
 */
export type FadeTransitionPhase =
  | "shown"
  | "fading-out"
  | "hidden"
  | "fading-in";

export interface UseFadeTransitionOptions {
  fadeOutMs?: number;
  fadeInMs?: number;
}

export interface UseFadeTransitionResult {
  phase: FadeTransitionPhase;
  /**
   * Height of the viewport captured at the moment the fade-out began.
   * Apply as `style.minHeight` while phase ≠ "shown" so the page doesn't
   * collapse and re-flow while the new data loads.
   */
  lockedHeight: number | null;
  /**
   * Attach to the scrollable viewport so the hook can read its height
   * when fade-out is triggered.
   */
  viewportRef: RefObject<HTMLDivElement>;
  /** Kick off the fade-out half of the cycle (e.g. on filter change). */
  beginExit: () => void;
  /**
   * Signal that fresh data has landed; advances `hidden → fading-in →
   * shown`. Calling before `beginExit` is a no-op.
   */
  beginEnter: () => void;
  /** True while we're in fading-out, hidden, or fading-in phases. */
  isTransitioning: boolean;
}

const DEFAULT_FADE_OUT_MS = 180;
const DEFAULT_FADE_IN_MS = 220;

/**
 * Drives the 4-stage fade animation used when the rankings filter
 * changes. State machine is owned here so the page component can stay
 * declarative — it just calls beginExit() on filter change and
 * beginEnter() once new data has landed.
 *
 * Timers are tracked with refs (not state) so back-to-back filter
 * changes during the fade-out don't queue up multiple transitions.
 */
export function useFadeTransition(
  options: UseFadeTransitionOptions = {},
): UseFadeTransitionResult {
  const fadeOutMs = options.fadeOutMs ?? DEFAULT_FADE_OUT_MS;
  const fadeInMs = options.fadeInMs ?? DEFAULT_FADE_IN_MS;

  const [phase, setPhase] = useState<FadeTransitionPhase>("shown");
  const [lockedHeight, setLockedHeight] = useState<number | null>(null);

  const viewportRef = useRef<HTMLDivElement>(null);
  const outTimerRef = useRef<number | null>(null);
  const inTimerRef = useRef<number | null>(null);

  const clearTimers = useCallback(() => {
    if (outTimerRef.current !== null) {
      window.clearTimeout(outTimerRef.current);
      outTimerRef.current = null;
    }
    if (inTimerRef.current !== null) {
      window.clearTimeout(inTimerRef.current);
      inTimerRef.current = null;
    }
  }, []);

  useEffect(() => clearTimers, [clearTimers]);

  const beginExit = useCallback(() => {
    const h = viewportRef.current?.offsetHeight ?? 0;
    if (h > 0) setLockedHeight(h);

    clearTimers();
    setPhase("fading-out");
    outTimerRef.current = window.setTimeout(() => {
      setPhase("hidden");
      outTimerRef.current = null;
    }, fadeOutMs);
  }, [clearTimers, fadeOutMs]);

  const beginEnter = useCallback(() => {
    // Only meaningful from the `hidden` phase — guards against double
    // calls when the query resolves AFTER the user has already
    // triggered a second filter swap.
    setPhase((current) => {
      if (current !== "hidden") return current;

      if (inTimerRef.current !== null) window.clearTimeout(inTimerRef.current);
      inTimerRef.current = window.setTimeout(() => {
        setPhase("shown");
        setLockedHeight(null);
        inTimerRef.current = null;
      }, fadeInMs);

      return "fading-in";
    });
  }, [fadeInMs]);

  return {
    phase,
    lockedHeight,
    viewportRef,
    beginExit,
    beginEnter,
    isTransitioning: phase !== "shown",
  };
}
