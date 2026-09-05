import { useCallback, useState } from "react";

import type { TierGroup } from "@shared/api";

// UI-facing position vocabulary. Maps 1:1 to the backend's team
// position string, with "" meaning "no position filter" (returns the
// blended-overall ranking).
export type Position = "" | "TOP" | "JUNGLE" | "MIDDLE" | "BOTTOM" | "UTILITY";

// UI tier vocabulary. Lowercase + underscore form is the i18n key and
// the legacy REST contract; mapToTierGroup() converts it to the
// uppercase TierGroup enum at the GraphQL boundary.
export type UiTier =
  | ""
  | "challenger"
  | "grandmaster_plus"
  | "grandmaster"
  | "master_plus"
  | "master";

export interface RankingsFiltersState {
  position: Position;
  tier: UiTier;
  region: string;
  version: string;
}

export interface UseRankingsFiltersOptions {
  initial?: Partial<RankingsFiltersState>;
  /**
   * Called the moment a filter value changes — *before* the page sets
   * the new value as the committed query. Used by the page to fire
   * `beginExit()` on the fade transition.
   */
  onBeforeCommit?: () => void;
}

const DEFAULT_FILTERS: RankingsFiltersState = {
  position: "",
  tier: "",
  region: "",
  version: "latest",
};

export interface UseRankingsFiltersResult {
  /** Filter currently selected in the UI controls. */
  selected: RankingsFiltersState;
  /** Filter actually driving the query — lags `selected` by one tick. */
  committed: RankingsFiltersState;
  setPosition: (next: Position) => void;
  setTier: (next: UiTier) => void;
  setRegion: (next: string) => void;
  setVersion: (next: string) => void;
  /**
   * Promote `selected` to `committed`. The page calls this from the
   * post-fade-out callback so the new query only fires once the table
   * has finished its exit animation.
   */
  commit: () => void;
}

/**
 * Owns the rankings page filter state, split into two snapshots:
 *
 *   - `selected` — what the user just clicked. Updates immediately so
 *     the dropdown UI feels responsive.
 *   - `committed` — what the data layer actually queries with. Lags
 *     selected until the page calls `commit()`, which is the hook's
 *     way of saying "the fade-out has finished, now refetch."
 *
 * The split lets the consumer keep the URL/dropdown state and the
 * query state decoupled, so changing two filters in rapid succession
 * doesn't fire two queries — only the last commit wins.
 */
export function useRankingsFilters(
  options: UseRankingsFiltersOptions = {},
): UseRankingsFiltersResult {
  const initial: RankingsFiltersState = {
    ...DEFAULT_FILTERS,
    ...options.initial,
  };
  const onBeforeCommit = options.onBeforeCommit;

  const [selected, setSelected] = useState<RankingsFiltersState>(initial);
  const [committed, setCommitted] = useState<RankingsFiltersState>(initial);

  const updateSelected = useCallback(
    <K extends keyof RankingsFiltersState>(
      key: K,
      value: RankingsFiltersState[K],
    ) => {
      setSelected((prev) => {
        if (prev[key] === value) return prev;
        onBeforeCommit?.();
        return { ...prev, [key]: value };
      });
    },
    [onBeforeCommit],
  );

  return {
    selected,
    committed,
    setPosition: (next) => updateSelected("position", next),
    setTier: (next) => updateSelected("tier", next),
    setRegion: (next) => updateSelected("region", next),
    setVersion: (next) => updateSelected("version", next),
    commit: () => setCommitted(selected),
  };
}

const TIER_MAP: Record<UiTier, TierGroup> = {
  "": "ALL",
  challenger: "CHALLENGER",
  grandmaster_plus: "GRANDMASTER_PLUS",
  grandmaster: "GRANDMASTER",
  master_plus: "MASTER_PLUS",
  master: "MASTER",
};

/**
 * Convert the UI tier vocabulary into the TierGroup enum the GraphQL
 * schema expects. Kept exported so the rankings table can also reuse
 * the mapping for badge color decisions.
 */
export function mapToTierGroup(tier: UiTier): TierGroup {
  return TIER_MAP[tier];
}
