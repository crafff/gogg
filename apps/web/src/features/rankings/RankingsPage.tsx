import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useRegionsQuery, useVersionsQuery } from "@shared/api";
import { Skeleton } from "@shared/ui";
import { cn } from "@shared/lib/cn";

import { RankingsFilters } from "./components/RankingsFilters";
import { RankingsStatsBar } from "./components/RankingsStatsBar";
import { RankingsTable } from "./components/RankingsTable";
import { useFadeTransition } from "./hooks/useFadeTransition";
import { useInfiniteScroll } from "./hooks/useInfiniteScroll";
import { useRankingsFilters } from "./hooks/useRankingsFilters";
import { useRankingsQuery } from "./hooks/useRankingsQuery";

const PAGE_SIZE = 40;

/**
 * Rankings page presenter. The orchestration is:
 *
 *  1. Filters hook owns the UI vs committed state split.
 *  2. The user changing a filter triggers `fade.beginExit()` (via the
 *     onBeforeCommit callback) — table starts fading out immediately.
 *  3. The post-fade-out effect commits the filter and resets the
 *     paging limit, which kicks off a new GraphQL query.
 *  4. When the query resolves (data refreshes), the post-load effect
 *     calls `fade.beginEnter()` → fade-in → "shown".
 *  5. While `phase ≠ "shown"` the query is disabled; the previous data
 *     stays on screen thanks to keepPreviousData inside useRankingsQuery.
 */
export function RankingsPage() {
  const { t } = useTranslation(["rankings", "common"]);
  const fade = useFadeTransition();
  const filters = useRankingsFilters({
    onBeforeCommit: () => {
      // Filter change fires synchronously; beginExit triggers the
      // fade animation immediately — the commit step below picks up
      // after the fade has finished.
      fade.beginExit();
      // Reset paging window so each new filter slice starts at page 1.
      setLimit(PAGE_SIZE);
    },
  });

  const [limit, setLimit] = useState(PAGE_SIZE);

  // Commit the selected filter once the fade-out has completed (state
  // arrives at "hidden"). The hook handles "shown" before any swap,
  // so this guard prevents firing commit at mount time.
  useEffect(() => {
    if (fade.phase === "hidden") {
      filters.commit();
    }
    // filters.commit is stable enough for chunk 4; chunk 5 will move
    // to a reducer to make this dependency cleaner.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fade.phase]);

  const versionsQuery = useVersionsQuery();
  const regionsQuery = useRegionsQuery();

  const rankings = useRankingsQuery({
    filters: filters.committed,
    limit,
    enabled: fade.phase === "shown" || fade.phase === "fading-in",
  });

  // Drive the "fade-in" half once new data has actually landed.
  useEffect(() => {
    if (fade.phase !== "hidden") return;
    if (rankings.isLoading || rankings.isFetching) return;
    fade.beginEnter();
  }, [fade, rankings.isLoading, rankings.isFetching]);

  const onLoadMore = useCallback(() => {
    if (rankings.isFetching || rankings.isError) return;
    if (rankings.items.length < limit) return; // backend returned fewer = end
    setLimit((prev) => prev + PAGE_SIZE);
  }, [rankings.isFetching, rankings.isError, rankings.items.length, limit]);

  const sentinelRef = useInfiniteScroll<HTMLDivElement>({
    onLoadMore,
    enabled: fade.phase === "shown" && rankings.items.length >= limit,
  });

  const lockedStyle =
    fade.lockedHeight !== null
      ? { minHeight: `${fade.lockedHeight}px` }
      : undefined;
  const reachedEnd = rankings.items.length > 0 && rankings.items.length < limit;

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-fg-default">
          {t("title")}
        </h1>
        <RankingsStatsBar
          totalMatches={rankings.totalMatches}
          resolvedVersion={rankings.resolvedVersion}
          region={filters.committed.region}
        />
      </header>

      <RankingsFilters
        selected={filters.selected}
        availableVersions={versionsQuery.data?.versions ?? []}
        availableRegions={regionsQuery.data?.regions ?? []}
        onPositionChange={filters.setPosition}
        onTierChange={filters.setTier}
        onRegionChange={filters.setRegion}
        onVersionChange={filters.setVersion}
      />

      <div
        ref={fade.viewportRef}
        style={lockedStyle}
        className={cn(
          "transition-opacity duration-200 ease-out-soft",
          fade.phase === "fading-out" && "opacity-0",
          fade.phase === "hidden" && "opacity-0",
          fade.phase === "fading-in" && "opacity-100",
          fade.phase === "shown" && "opacity-100",
        )}
      >
        {fade.phase === "hidden" && (
          <p className="py-8 text-center text-fg-muted">
            {t("common:state.refreshing")}
          </p>
        )}

        {rankings.isError && (
          <p
            className="py-8 text-center text-red-300"
            data-testid="rankings-error"
          >
            {t("common:state.error")}
          </p>
        )}

        {rankings.isLoading &&
          rankings.items.length === 0 &&
          !rankings.isError && <SkeletonTable />}

        {!rankings.isError && rankings.items.length > 0 && (
          <RankingsTable items={rankings.items} />
        )}

        {!rankings.isError && rankings.items.length > 0 && (
          <div
            ref={!reachedEnd ? sentinelRef : undefined}
            className="py-4 text-center text-xs text-fg-subtle"
            data-testid="rankings-load-more"
          >
            {rankings.isFetching
              ? t("common:state.loadingMore")
              : reachedEnd
                ? t("common:state.endOfList")
                : t("common:state.loadMore")}
          </div>
        )}

        {!rankings.isError &&
          !rankings.isLoading &&
          rankings.items.length === 0 && (
            <p className="py-8 text-center text-fg-muted">
              {t("common:state.empty")}
            </p>
          )}
      </div>
    </section>
  );
}

function SkeletonTable() {
  return (
    <div className="space-y-2" data-testid="rankings-skeleton">
      {Array.from({ length: 8 }).map((_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}
