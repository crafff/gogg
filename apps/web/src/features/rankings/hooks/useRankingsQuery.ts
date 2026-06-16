import { useMemo } from "react";
import { keepPreviousData } from "@tanstack/react-query";

import {
  type ChampionRankingsFilter,
  type ChampionRankingsQuery,
  useChampionRankingsQuery,
} from "@shared/api";

import {
  type RankingsFiltersState,
  mapToTierGroup,
} from "./useRankingsFilters";

// Fixed value matching legacy parity — every rankings query asks for
// champions with at least 20 games so noisy low-sample rows don't
// dominate the head of the table.
const FIXED_MIN_GAMES = 20;

export interface UseRankingsQueryOptions {
  filters: RankingsFiltersState;
  /**
   * Current page-size limit. The infinite-scroll handler bumps this
   * up; the backend returns the first `limit` rows.
   */
  limit: number;
  /**
   * When false the hook holds the query in a disabled state — useful
   * during fade-out animations so the new query doesn't fire before
   * the table has finished exiting.
   */
  enabled?: boolean;
}

export interface UseRankingsQueryResult {
  items: ChampionRankingsQuery["championRankings"]["items"];
  totalMatches: number;
  resolvedVersion: string | null;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  error: Error | null;
}

/**
 * Adapter between the UI filter state and the codegen'd
 * `useChampionRankingsQuery`. It (1) projects the UI vocabulary into
 * the GraphQL `ChampionRankingsFilter`, (2) injects the constant
 * minGames floor, and (3) flattens the response into a tuple the
 * presenter components can consume directly.
 *
 * `keepPreviousData` is on so the rankings table doesn't blank out
 * during pagination requests — the legacy fade-out covers the more
 * disruptive filter swaps.
 */
export function useRankingsQuery({
  filters,
  limit,
  enabled = true,
}: UseRankingsQueryOptions): UseRankingsQueryResult {
  const filter = useMemo<ChampionRankingsFilter>(
    () => ({
      position: filters.position || "",
      tierGroup: mapToTierGroup(filters.tier),
      region: filters.region,
      version: filters.version,
      minGames: FIXED_MIN_GAMES,
      limit,
    }),
    [filters, limit],
  );

  const query = useChampionRankingsQuery(
    { filter },
    {
      enabled,
      placeholderData: keepPreviousData,
    },
  );

  return {
    items: query.data?.championRankings.items ?? [],
    totalMatches: query.data?.championRankings.totalMatches ?? 0,
    resolvedVersion: query.data?.championRankings.resolvedVersion ?? null,
    isLoading: query.isLoading,
    isFetching: query.isFetching,
    isError: query.isError,
    error: query.error instanceof Error ? query.error : null,
  };
}
