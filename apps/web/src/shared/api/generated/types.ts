/** Internal type. DO NOT USE DIRECTLY. */
type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
/** Internal type. DO NOT USE DIRECTLY. */
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
/**
 * Filter for championRankings. Every field is optional with sensible
 * defaults; clients pass only what differs from the defaults. Mirrors
 * the legacy REST query params 1:1.
 */
export type ChampionRankingsFilter = {
  /** Cap the result rows. -1 = unlimited. */
  limit?: number | null | undefined;
  /** Drop champions with fewer games than this. 0 = no floor. */
  minGames?: number | null | undefined;
  /**
   * Restrict to a single team position (TOP, JUNGLE, MIDDLE, BOTTOM,
   * UTILITY). When set the resolver calls the per-position path and
   * every result row's teamPosition is exactly [position].
   */
  position?: string | null | undefined;
  /**
   * Minimum share of games per position to list the position in
   * teamPosition. 0.5 = position is "primary" if played in ≥50% of
   * games. Ignored when `position` is set.
   */
  positionThreshold?: number | null | undefined;
  /** Queue ID. 420 = Ranked Solo/Duo, 440 = Ranked Flex. */
  queueId?: number | null | undefined;
  /** Region code (KR, NA1). Empty = all. */
  region?: string | null | undefined;
  /** Rank bucket. ALL = no tier filter. */
  tierGroup?: TierGroup | null | undefined;
  /** Exact game version (e.g. "14.23.1") or "latest". Empty = all. */
  version?: string | null | undefined;
};

/**
 * Coarse rank bucket used to filter rankings. Matches the legacy REST
 * tier_group query param 1:1 so /api/v1 and /graphql return the same
 * aggregations for any given filter combination.
 */
export type TierGroup =
  /** All tiers (no filter). */
  | 'ALL'
  /** CHALLENGER only. */
  | 'CHALLENGER'
  /** GRANDMASTER only. */
  | 'GRANDMASTER'
  /** GRANDMASTER + CHALLENGER. */
  | 'GRANDMASTER_PLUS'
  /** MASTER only. */
  | 'MASTER'
  /** MASTER + GRANDMASTER + CHALLENGER. */
  | 'MASTER_PLUS';

export type VersionsQueryVariables = Exact<{ [key: string]: never; }>;


export type VersionsQuery = { versions: Array<string> };

export type RegionsQueryVariables = Exact<{ [key: string]: never; }>;


export type RegionsQuery = { regions: Array<string> };

export type ChampionRankingsQueryVariables = Exact<{
  filter?: ChampionRankingsFilter | null | undefined;
}>;


export type ChampionRankingsQuery = { championRankings: { totalMatches: number, resolvedVersion: string | null, items: Array<{ championId: number, championName: string, teamPosition: Array<string>, games: number, wins: number, losses: number, winRate: number, pickRate: number, banRate: number, kda: number }> } };
