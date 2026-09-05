/** Internal type. DO NOT USE DIRECTLY. */
type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
/** Internal type. DO NOT USE DIRECTLY. */
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
export type ChampionDetailFilter = {
  position?: string | null | undefined;
  queueId?: number | null | undefined;
  region?: string | null | undefined;
  tierGroup?: TierGroup | null | undefined;
  version?: string | null | undefined;
};

export type ChampionInsightAvailability =
  | 'AVAILABLE'
  | 'INSUFFICIENT_SAMPLE'
  | 'UNAVAILABLE';

export type ChampionInsightEvidenceGrade =
  | 'OBSERVED';

export type ChampionInsightFactorKind =
  | 'BEHAVIOR_METRIC';

/**
 * Filter for championRankings. Every field is optional with sensible
 * defaults; clients pass only what differs from the defaults. Mirrors
 * the legacy REST query params 1:1.
 */
export type ChampionRankingsFilter = {
  /** Drop champions with fewer games than this. Legal range is 1–20000. */
  minGames?: number | null | undefined;
  /**
   * Restrict to a single team position (TOP, JUNGLE, MIDDLE, BOTTOM,
   * UTILITY). When set the resolver calls the per-position path and
   * every result row's teamPosition is exactly [position].
   */
  position?: string | null | undefined;
  /**
   * Minimum percentage of games per position to list the position in
   * teamPosition. Legal range is 0–100. Ignored when `position` is set.
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

export type ChampionWinFactorsFilter = {
  position: string;
  queueId?: number | null | undefined;
  region?: string | null | undefined;
  tierGroup?: TierGroup | null | undefined;
  version?: string | null | undefined;
};

export type SummonerHistoryInput = {
  after?: string | null | undefined;
  first?: number | null | undefined;
  queue?: SummonerQueueFilter | null | undefined;
};

export type SummonerIdentityInput = {
  gameName: string;
  region: string;
  tagLine: string;
};

export type SummonerLookupStage =
  | 'ENRICH_RANKS'
  | 'FETCH_MATCHES'
  | 'FINALIZE'
  | 'QUEUED'
  | 'REFRESH_PROFILE'
  | 'RESOLVE_ACCOUNT';

export type SummonerLookupStatus =
  | 'COMPLETED'
  | 'FAILED'
  | 'PARTIAL'
  | 'QUEUED'
  | 'RUNNING';

export type SummonerQueueFilter =
  | 'ALL'
  | 'DRAFT_PICK'
  | 'RANKED_FLEX'
  | 'RANKED_SOLO'
  | 'SWIFTPLAY';

export type TftCohort =
  | 'DIAMOND'
  | 'MASTER_PLUS';

export type TftHistoryInput = {
  after?: string | null | undefined;
  first?: number | null | undefined;
  gameName: string;
  locale?: string | null | undefined;
  platform: string;
  queueId?: number | null | undefined;
  tagLine: string;
};

export type TftIdentityInput = {
  gameName: string;
  platform: string;
  tagLine: string;
};

export type TftItemClass =
  | 'ARTIFACT'
  | 'EMBLEM'
  | 'STANDARD'
  | 'UNKNOWN';

export type TftLineupSignalKind =
  | 'ARTIFACT'
  | 'AUGMENT'
  | 'EMBLEM'
  | 'THREE_STAR';

export type TftLineupsFilter = {
  cohort?: TftCohort | null | undefined;
  limit?: number | null | undefined;
  locale?: string | null | undefined;
  minSamples?: number | null | undefined;
  patch?: string | null | undefined;
  platform: string;
  setNumber: number;
  window?: TftWindow | null | undefined;
};

export type TftLookupStage =
  | 'FETCH_MATCHES'
  | 'FETCH_MATCH_IDS'
  | 'FINALIZE'
  | 'QUEUED'
  | 'RESOLVE_ACCOUNT';

export type TftLookupStatus =
  | 'COMPLETED'
  | 'FAILED'
  | 'PARTIAL'
  | 'QUEUED'
  | 'RUNNING';

export type TftObservedDataKind =
  | 'OBSERVED_RUN_PREVIEW';

export type TftObservedLineupsFilter = {
  limit?: number | null | undefined;
  locale?: string | null | undefined;
  minSamples?: number | null | undefined;
  platform?: string | null | undefined;
};

export type TftWindow =
  | 'PATCH'
  | 'THREE_DAYS';

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

export type MeQueryVariables = Exact<{ [key: string]: never; }>;


export type MeQuery = { me: { id: string, displayName: string, email: string | null, avatarUrl: string | null, locale: string, identities: Array<{ provider: string, username: string | null }> } | null };

export type AuthProvidersQueryVariables = Exact<{ [key: string]: never; }>;


export type AuthProvidersQuery = { authProviders: Array<{ id: string }> };

export type VersionsQueryVariables = Exact<{ [key: string]: never; }>;


export type VersionsQuery = { versions: Array<string> };

export type RegionsQueryVariables = Exact<{ [key: string]: never; }>;


export type RegionsQuery = { regions: Array<string> };

export type ChampionDetailQueryVariables = Exact<{
  id: number;
  filter?: ChampionDetailFilter | null | undefined;
}>;


export type ChampionDetailQuery = { championDetail: { championId: number, championName: string, games: number, resolvedVersion: string | null, runeBuilds: Array<{ primaryStyleId: number, secondaryStyleId: number, perkIds: Array<number>, statShardIds: Array<number>, games: number, eligibleGames: number, pickRate: number, winRate: number }>, summonerSpellBuilds: Array<{ ids: Array<number>, games: number, eligibleGames: number, pickRate: number, winRate: number }>, starterBuilds: Array<{ ids: Array<number>, games: number, eligibleGames: number, pickRate: number, winRate: number }>, bootsBuilds: Array<{ ids: Array<number>, games: number, eligibleGames: number, pickRate: number, winRate: number }>, itemBuilds: Array<{ stage: number, builds: Array<{ ids: Array<number>, games: number, eligibleGames: number, pickRate: number, winRate: number }> }> } | null };

export type ChampionWinFactorsQueryVariables = Exact<{
  id: number;
  filter: ChampionWinFactorsFilter;
}>;


export type ChampionWinFactorsQuery = { championWinFactors: { championId: number, championName: string, position: string, resolvedVersion: string | null, regionScope: string, tierGroup: TierGroup, revision: string | null, algorithm: string | null, dataThrough: string | null, publishedAt: string | null, availability: ChampionInsightAvailability, unavailableReason: string | null, cohortScope: string | null, sampleGames: number, samplePlayers: number, factors: Array<{ metricKey: string, kind: ChampionInsightFactorKind, startMinute: number, endMinute: number, unit: string, p50: number, p70: number, p90: number, evidenceGrade: ChampionInsightEvidenceGrade, displayOrder: number, buckets: Array<{ ordinal: number, lowerBound: number, upperBound: number, games: number, wins: number, samplePlayers: number, observedWinRate: number, observedWinRateDelta: number }> }> } | null };

export type ChampionRankingsQueryVariables = Exact<{
  filter?: ChampionRankingsFilter | null | undefined;
}>;


export type ChampionRankingsQuery = { championRankings: { totalMatches: number, resolvedVersion: string | null, items: Array<{ championId: number, championName: string, teamPosition: Array<string>, games: number, wins: number, losses: number, winRate: number, pickRate: number, banRate: number, kda: number }> } };

export type SummonerQueryVariables = Exact<{
  identity: SummonerIdentityInput;
  history?: SummonerHistoryInput | null | undefined;
}>;


export type SummonerQuery = { summoner: { profile: { region: string, gameName: string, tagLine: string, profileIconId: number, summonerLevel: number, lastRefreshedAt: string | null, isStale: boolean }, ranks: Array<{ queueType: string, tier: string, division: string, leaguePoints: number, wins: number, losses: number, winRate: number }>, history: { items: Array<{ matchId: string, queue: SummonerQueueFilter, queueId: number, gameStartTime: string, durationSeconds: number, version: string, endOfGameResult: string, position: string, win: boolean, championId: number, championName: string, championLevel: number, kills: number, deaths: number, assists: number, kda: number, minionsKilled: number, csPerMinute: number, goldEarned: number, damageToChampions: number, visionScore: number, itemIds: Array<number>, summonerSpellIds: Array<number>, primaryStyleId: number, secondaryStyleId: number, perkIds: Array<number>, statShardIds: Array<number>, earlySurrender: boolean, surrender: boolean, averageTier: string | null, averageDivision: string | null, tierCoverage: number, participants: Array<{ participantId: number, teamId: number, isCurrentPlayer: boolean, gameName: string, tagLine: string, position: string, win: boolean, championId: number, championName: string, championLevel: number, kills: number, deaths: number, assists: number, kda: number, minionsKilled: number, goldEarned: number, damageToChampions: number, visionScore: number, itemIds: Array<number>, summonerSpellIds: Array<number>, primaryStyleId: number, secondaryStyleId: number, perkIds: Array<number>, rank: { tier: string, division: string | null, leaguePoints: number | null, snapshotDeltaHours: number | null } | null }> }>, pageInfo: { endCursor: string | null, hasNextPage: boolean, returned: number } } } | null };

export type RefreshSummonerMutationVariables = Exact<{
  identity: SummonerIdentityInput;
}>;


export type RefreshSummonerMutation = { refreshSummoner: { fresh: boolean, reused: boolean, job: { id: string, region: string, gameName: string, tagLine: string, status: SummonerLookupStatus, stage: SummonerLookupStage, scannedCount: number, supportedCount: number, fetchedCount: number, failedCount: number, errorCode: string | null, errorMessage: string | null, createdAt: string, updatedAt: string, completedAt: string | null } | null } };

export type SummonerLookupJobQueryVariables = Exact<{
  id: string | number;
}>;


export type SummonerLookupJobQuery = { summonerLookupJob: { id: string, region: string, gameName: string, tagLine: string, status: SummonerLookupStatus, stage: SummonerLookupStage, scannedCount: number, supportedCount: number, fetchedCount: number, failedCount: number, errorCode: string | null, errorMessage: string | null, createdAt: string, updatedAt: string, completedAt: string | null } | null };

export type TftAnalysisCatalogQueryVariables = Exact<{ [key: string]: never; }>;


export type TftAnalysisCatalogQuery = { tftAnalysisCatalog: Array<{ platform: string, patch: string, setNumber: number, queueId: number, cohort: TftCohort, window: TftWindow, publishedAt: string, sourceMatches: number, sourceParticipants: number, coverage: { windowStart: string, windowEnd: string, exactLineups: number, familyThreshold: number } }> };

export type TftLineupsQueryVariables = Exact<{
  filter: TftLineupsFilter;
}>;


export type TftLineupsQuery = { tftLineups: { platform: string, patch: string, setNumber: number, queueId: number, cohort: TftCohort, window: TftWindow, locale: string, algorithmVersion: string, publishedAt: string, sourceMatches: number, sourceParticipants: number, coverage: { windowStart: string, windowEnd: string, exactLineups: number, familyThreshold: number }, items: Array<{ id: string, teamCode: string | null, starCompositionKnownSamples: number, starCompositionUnknownSamples: number, starCompositionCoverage: number, coreUnits: Array<{ id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass }>, commonItems: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, commonAugments: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, commonTraits: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, signals: Array<{ kind: TftLineupSignalKind, sampleSize: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass }, holderUnit: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } | null }>, unitItems: Array<{ isCore: boolean, coreRank: number | null, averageItems: number, itemInvestmentRate: number, equippedRate: number, threeItemRate: number, knownStarSamples: number, unknownStarSamples: number, starCoverage: number, unit: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass }, commonItems: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, starDistribution: Array<{ stars: number, sampleSize: number, rate: number, knownRate: number, avgPlacement: number | null, firstRate: number | null, top4Rate: number | null }> }>, starLevels: Array<{ totalStars: number, sampleSize: number, lobbyCount: number, rate: number, avgPlacement: number, firstRate: number, top4Rate: number }>, starCompositions: Array<{ totalStars: number, sampleSize: number, rate: number, avgPlacement: number | null, firstRate: number | null, top4Rate: number | null, levels: Array<{ stars: number, unitCount: number }> }>, metrics: { sampleSize: number, lobbyCount: number, pickRate: number, avgPlacement: number, firstRate: number, top4Rate: number, contestedRate: number } }> } };

export type TftObservedLineupsQueryVariables = Exact<{
  filter: TftObservedLineupsFilter;
}>;


export type TftObservedLineupsQuery = { tftObservedLineups: { dataKind: TftObservedDataKind, runId: string, platform: string, platforms: Array<string>, queueId: number, setNumber: number, patch: string | null, rawGameVersions: Array<string>, locale: string, algorithmVersion: string, sourceMatches: number, sourceParticipants: number, usableParticipants: number, exactLineups: number, windowStart: string, windowEnd: string, catalogSnapshot: { source: string, patch: string, revision: string }, assetSnapshot: { source: string, patch: string, revision: string } | null, items: Array<{ id: string, teamCode: string | null, starCompositionKnownSamples: number, starCompositionUnknownSamples: number, starCompositionCoverage: number, coreUnits: Array<{ id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass }>, commonItems: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, commonAugments: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, commonTraits: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, signals: Array<{ kind: TftLineupSignalKind, sampleSize: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass }, holderUnit: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } | null }>, unitItems: Array<{ isCore: boolean, coreRank: number | null, averageItems: number, itemInvestmentRate: number, equippedRate: number, threeItemRate: number, knownStarSamples: number, unknownStarSamples: number, starCoverage: number, unit: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass }, commonItems: Array<{ count: number, rate: number, entity: { id: string, name: string | null, iconUrl: string | null, cost: number | null, itemClass: TftItemClass } }>, starDistribution: Array<{ stars: number, sampleSize: number, rate: number, knownRate: number, avgPlacement: number | null, firstRate: number | null, top4Rate: number | null }> }>, starLevels: Array<{ totalStars: number, sampleSize: number, lobbyCount: number, rate: number, avgPlacement: number, firstRate: number, top4Rate: number }>, starCompositions: Array<{ totalStars: number, sampleSize: number, rate: number, avgPlacement: number | null, firstRate: number | null, top4Rate: number | null, levels: Array<{ stars: number, unitCount: number }> }>, metrics: { sampleSize: number, lobbyCount: number, pickRate: number, avgPlacement: number, firstRate: number, top4Rate: number, contestedRate: number } }> } | null };

export type TftMatchHistoryQueryVariables = Exact<{
  input: TftHistoryInput;
}>;


export type TftMatchHistoryQuery = { tftMatchHistory: { profile: { platform: string, gameName: string, tagLine: string, lastRefreshedAt: string | null, isStale: boolean }, matches: Array<{ matchId: string, platform: string, queueId: number, patch: string, gameVersion: string, gameDatetime: string, gameLengthSeconds: number, setNumber: number, tftGameType: string, endOfGameResult: string, participant: { puuid: string, gameName: string, tagLine: string, isCurrentPlayer: boolean, placement: number, level: number, goldLeft: number, lastRound: number, playersEliminated: number, totalDamageToPlayers: number, augments: Array<{ id: string, name: string | null, iconUrl: string | null }>, traits: Array<{ numUnits: number, style: number, tierCurrent: number, tierTotal: number, entity: { id: string, name: string | null, iconUrl: string | null } }>, units: Array<{ rarity: number, tier: number, entity: { id: string, name: string | null, iconUrl: string | null }, items: Array<{ id: string, name: string | null, iconUrl: string | null }> }> } }>, pageInfo: { endCursor: string | null, hasNextPage: boolean, returned: number } } | null };

export type RefreshTftPlayerMutationVariables = Exact<{
  identity: TftIdentityInput;
}>;


export type RefreshTftPlayerMutation = { refreshTFTPlayer: { fresh: boolean, reused: boolean, job: { id: string, platform: string, gameName: string, tagLine: string, status: TftLookupStatus, stage: TftLookupStage, scannedCount: number, fetchedCount: number, failedCount: number, errorCode: string | null, createdAt: string, updatedAt: string, completedAt: string | null } | null } };

export type TftLookupJobQueryVariables = Exact<{
  id: string | number;
}>;


export type TftLookupJobQuery = { tftLookupJob: { id: string, platform: string, gameName: string, tagLine: string, status: TftLookupStatus, stage: TftLookupStage, scannedCount: number, fetchedCount: number, failedCount: number, errorCode: string | null, createdAt: string, updatedAt: string, completedAt: string | null } | null };
