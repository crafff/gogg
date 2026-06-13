export type Position = "" | "TOP" | "JUNGLE" | "MIDDLE" | "BOTTOM" | "UTILITY";

export type Tier =
  | ""
  | "challenger"
  | "grandmaster_plus"
  | "grandmaster"
  | "master_plus"
  | "master";

export interface RankingsFilters {
  position: Position;
  tier: Tier;
  region: string;
  version: string;
  limit: number;
  minGames: number;
}

export interface ChampionRankingItem {
  championId: number;
  championName: string;
  teamPosition: string[];
  games: number;
  wins: number;
  losses: number;
  winRate: number;
  pickRate: number;
  banRate: number;
  kda: number;
}

export interface RankingsMeta {
  queueId: number;
  version: string;
  region: string;
  tier: string;
  position: string;
  minGames: number;
  limit: number;
  positionThreshold: number;
  totalMatches: number;
}

export interface RankingsResponse {
  items: ChampionRankingItem[];
  meta: RankingsMeta;
}
