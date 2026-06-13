import { useEffect, useState, useCallback, useMemo } from "react";
import { fetchChampionRankings } from "@services/api";
import type { RankingsFilters, ChampionRankingItem, RankingsMeta } from "@shared-types";

interface UseRankingsResult {
  items: ChampionRankingItem[];
  meta: RankingsMeta | null;
  loading: boolean;
  error: string | null;
}

export function useRankings(filters: RankingsFilters): UseRankingsResult {
  const [items, setItems] = useState<ChampionRankingItem[]>([]);
  const [meta, setMeta] = useState<RankingsMeta | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const queryKey = useMemo(
    () =>
      `${filters.position}-${filters.tier}-${filters.region}-${filters.version}-${filters.limit}-${filters.minGames}`,
    [filters.position, filters.tier, filters.region, filters.version, filters.limit, filters.minGames]
  );

  const fetchData = useCallback(async (signal: { cancelled: boolean }) => {
    try {
      setLoading(true);
      setError(null);
      const data = await fetchChampionRankings(filters);
      if (!signal.cancelled) {
        setItems(Array.isArray(data.items) ? data.items : []);
        setMeta(data.meta ?? null);
      }
    } catch (err) {
      if (!signal.cancelled) {
        setError(err instanceof Error ? err.message : "Failed to load rankings");
      }
    } finally {
      if (!signal.cancelled) setLoading(false);
    }
  }, [filters]);

  useEffect(() => {
    const signal = { cancelled: false };
    void fetchData(signal);
    return () => { signal.cancelled = true; };
  }, [queryKey, fetchData]);

  return { items, meta, loading, error };
}
