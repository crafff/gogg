import type { ChampionRankingsQuery } from "@shared/api";

export type RankingRow =
  ChampionRankingsQuery["championRankings"]["items"][number];
export type SortKey =
  "composite" | "winRate" | "pickRate" | "banRate" | "kda" | "games";
export type SortDirection = "asc" | "desc";

export function sortRankings(
  items: ReadonlyArray<RankingRow>,
  key: SortKey,
  direction: SortDirection,
): RankingRow[] {
  const scores = key === "composite" ? compositeScores(items) : null;
  const sign = direction === "desc" ? -1 : 1;
  return [...items].sort((left, right) => {
    const leftValue =
      scores?.get(left) ?? left[key as Exclude<SortKey, "composite">];
    const rightValue =
      scores?.get(right) ?? right[key as Exclude<SortKey, "composite">];
    const difference = (leftValue - rightValue) * sign;
    return (
      difference ||
      right.games - left.games ||
      left.championId - right.championId
    );
  });
}

function compositeScores(
  items: ReadonlyArray<RankingRow>,
): Map<RankingRow, number> {
  const dimensions = ["winRate", "pickRate", "banRate", "kda"] as const;
  const sorted = Object.fromEntries(
    dimensions.map((key) => [
      key,
      items.map((row) => row[key]).sort((a, b) => a - b),
    ]),
  ) as Record<(typeof dimensions)[number], number[]>;
  return new Map(
    items.map((row) => [
      row,
      dimensions.reduce(
        (sum, key) => sum + percentile(sorted[key], row[key]),
        0,
      ) / dimensions.length,
    ]),
  );
}

function percentile(sorted: number[], value: number): number {
  if (sorted.length <= 1) return 1;
  const below = sorted.findIndex((candidate) => candidate >= value);
  const first = below === -1 ? sorted.length - 1 : below;
  let last = first;
  while (last + 1 < sorted.length && sorted[last + 1] === value) last++;
  return (first + last) / 2 / (sorted.length - 1);
}
