import { useEffect, useMemo, useRef, useState } from "react";
import { useRankings } from "@hooks/useRankings";
import { fetchVersions, fetchRegions } from "@services/api";
import { Header, MainLayout } from "@components/Layout";
import { FiltersPanel, RankingsTable, StateMessage } from "@components/UI";
import type { Position, Tier, RankingsFilters } from "@shared-types";
import styles from "./Rankings.module.css";

const PAGE_SIZE = 40;
const FIXED_MIN_GAMES = 20;
const FADE_OUT_MS = 180;
const FADE_IN_MS = 220;

type ListAnimationPhase = "shown" | "fading-out" | "hidden" | "fading-in";

interface QueryFilters {
  position: Position;
  tier: Tier;
  region: string;
  version: string;
}

export function Rankings() {
  // Selected (UI) vs committed (query) filters
  const [selected, setSelected] = useState<QueryFilters>({
    position: "", tier: "", region: "", version: "latest",
  });
  const [query, setQuery] = useState<QueryFilters>({
    position: "", tier: "", region: "", version: "latest",
  });

  const [limit, setLimit] = useState(PAGE_SIZE);
  const [hasMore, setHasMore] = useState(true);
  const [hasReachedListEnd, setHasReachedListEnd] = useState(false);
  const [listPhase, setListPhase] = useState<ListAnimationPhase>("shown");
  const [awaitingNewData, setAwaitingNewData] = useState(false);
  const [isPagingLoading, setIsPagingLoading] = useState(false);
  const [lockedViewportHeight, setLockedViewportHeight] = useState<number | null>(null);

  const [availableVersions, setAvailableVersions] = useState<string[]>([]);
  const [availableRegions, setAvailableRegions] = useState<string[]>([]);

  const listViewportRef = useRef<HTMLDivElement | null>(null);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);
  const switchTimerRef = useRef<number | null>(null);
  const fadeInTimerRef = useRef<number | null>(null);
  const pagingTimerRef = useRef<number | null>(null);
  const startedLoadingForSwitchRef = useRef(false);

  useEffect(() => {
    void fetchVersions().then(setAvailableVersions).catch(() => {});
    void fetchRegions().then(setAvailableRegions).catch(() => {});
  }, []);

  const filters: RankingsFilters = useMemo(
    () => ({
      position: query.position,
      tier: query.tier,
      region: query.region,
      version: query.version,
      minGames: FIXED_MIN_GAMES,
      limit,
    }),
    [query, limit]
  );

  const { items, meta, loading, error } = useRankings(filters);
  const isInitialLoading = loading && items.length === 0 && !awaitingNewData;
  const isLoadingMore = loading && items.length > 0;
  const isSwitchLoading = awaitingNewData || (listPhase === "hidden" && loading);
  const showPagingLoader = !awaitingNewData && (isPagingLoading || isLoadingMore);

  const commitFilterChange = (next: QueryFilters) => {
    const currentHeight = listViewportRef.current?.offsetHeight ?? 0;
    if (currentHeight > 0) setLockedViewportHeight(currentHeight);

    setIsPagingLoading(false);
    setHasMore(true);
    setHasReachedListEnd(false);
    setListPhase("fading-out");

    if (switchTimerRef.current !== null) window.clearTimeout(switchTimerRef.current);
    switchTimerRef.current = window.setTimeout(() => {
      setListPhase("hidden");
      setAwaitingNewData(true);
      startedLoadingForSwitchRef.current = false;
      setLimit(PAGE_SIZE);
      setQuery(next);
    }, FADE_OUT_MS);
  };

  const handlePositionChange = (position: Position) => {
    if (position === selected.position) return;
    const next = { ...selected, position };
    setSelected(next);
    commitFilterChange(next);
  };

  const handleTierChange = (tier: Tier) => {
    if (tier === selected.tier) return;
    const next = { ...selected, tier };
    setSelected(next);
    commitFilterChange(next);
  };

  const handleRegionChange = (region: string) => {
    if (region === selected.region) return;
    const next = { ...selected, region };
    setSelected(next);
    commitFilterChange(next);
  };

  const handleVersionChange = (version: string) => {
    if (version === selected.version) return;
    const next = { ...selected, version };
    setSelected(next);
    commitFilterChange(next);
  };

  useEffect(() => {
    setLimit(PAGE_SIZE);
    setHasMore(true);
    setHasReachedListEnd(false);
  }, [query]);

  useEffect(() => {
    if (!awaitingNewData) return;
    if (loading) { startedLoadingForSwitchRef.current = true; return; }
    if (!startedLoadingForSwitchRef.current) return;

    setAwaitingNewData(false);
    setListPhase("fading-in");
    if (fadeInTimerRef.current !== null) window.clearTimeout(fadeInTimerRef.current);
    fadeInTimerRef.current = window.setTimeout(() => {
      setListPhase("shown");
      setLockedViewportHeight(null);
    }, FADE_IN_MS);
  }, [awaitingNewData, loading]);

  useEffect(() => {
    if (!awaitingNewData && listPhase === "shown") setLockedViewportHeight(null);
  }, [awaitingNewData, listPhase]);

  useEffect(() => {
    return () => {
      if (switchTimerRef.current !== null) window.clearTimeout(switchTimerRef.current);
      if (fadeInTimerRef.current !== null) window.clearTimeout(fadeInTimerRef.current);
      if (pagingTimerRef.current !== null) window.clearTimeout(pagingTimerRef.current);
    };
  }, []);

  useEffect(() => {
    if (!isPagingLoading || loading) return;
    if (pagingTimerRef.current !== null) window.clearTimeout(pagingTimerRef.current);
    pagingTimerRef.current = window.setTimeout(() => setIsPagingLoading(false), 420);
  }, [isPagingLoading, loading]);

  useEffect(() => {
    if (error) { setHasMore(false); return; }
    if (!loading) setHasMore(items.length >= limit);
  }, [items.length, limit, loading, error]);

  useEffect(() => {
    if (awaitingNewData || hasReachedListEnd) return;
    const node = loadMoreRef.current;
    if (!node) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const [entry] = entries;
        if (!entry?.isIntersecting || loading || error || isPagingLoading) return;
        if (!hasMore) { setHasReachedListEnd(true); observer.disconnect(); return; }
        setIsPagingLoading(true);
        setLimit((prev) => prev + PAGE_SIZE);
      },
      { root: null, rootMargin: "220px 0px", threshold: 0 }
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [loading, hasMore, error, isPagingLoading, awaitingNewData, hasReachedListEnd]);

  return (
    <MainLayout>
      <Header />

      <section className={styles.section}>
        <FiltersPanel
          position={selected.position}
          onPositionChange={handlePositionChange}
          tier={selected.tier}
          onTierChange={handleTierChange}
          region={selected.region}
          onRegionChange={handleRegionChange}
          version={selected.version}
          onVersionChange={handleVersionChange}
          availableRegions={availableRegions}
          availableVersions={availableVersions}
        />
      </section>

      {meta && meta.totalMatches > 0 && (
        <section className={styles.section}>
          <div className={styles.statsBar}>
            基于 <strong>{meta.totalMatches.toLocaleString()}</strong> 场比赛数据
            {meta.version && meta.version !== "latest" && (
              <span className={styles.statsDivider}>·</span>
            )}
            {meta.version && <span>版本 {meta.version}</span>}
            {meta.region && (
              <>
                <span className={styles.statsDivider}>·</span>
                <span>{meta.region}</span>
              </>
            )}
          </div>
        </section>
      )}

      <section className={styles.section}>
        <div
          ref={listViewportRef}
          className={`${styles.listViewport} ${
            listPhase === "fading-out" ? styles.fadeOut : styles.fadeIn
          }`}
          style={lockedViewportHeight !== null ? { minHeight: `${lockedViewportHeight}px` } : undefined}
        >
          {isSwitchLoading && (
            <div className={styles.centerLoader}>
              <span className={styles.loader} />
              <span>正在刷新榜单...</span>
            </div>
          )}
          {isInitialLoading && !isSwitchLoading && (
            <StateMessage message="Loading rankings..." type="loading" />
          )}
          {error && <StateMessage message={error} type="error" />}
          {!loading && !error && items.length === 0 && (
            <StateMessage message="No ranking data found." type="empty" />
          )}
          {!error && !awaitingNewData && items.length > 0 && (
            <RankingsTable items={items} />
          )}
          {!error && !awaitingNewData && items.length > 0 && (
            <div
              ref={!hasReachedListEnd ? loadMoreRef : undefined}
              className={styles.loadMoreHint}
            >
              {showPagingLoader ? (
                <span className={styles.inlineLoaderWrap}>
                  <span className={styles.loaderSmall} />
                  正在加载更多数据...
                </span>
              ) : hasMore || !hasReachedListEnd ? (
                "向下滑加载更多数据"
              ) : (
                "已加载全部数据"
              )}
            </div>
          )}
        </div>
      </section>
    </MainLayout>
  );
}
