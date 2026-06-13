import type { RankingsResponse, RankingsFilters } from "@shared-types";

const API_BASE = "/api";

export async function fetchChampionRankings(
  filters: RankingsFilters
): Promise<RankingsResponse> {
  const params = new URLSearchParams();
  params.set("limit", String(filters.limit));
  params.set("minGames", String(filters.minGames));
  if (filters.position) params.set("position", filters.position);
  if (filters.tier)     params.set("tier", filters.tier);
  if (filters.region)   params.set("region", filters.region);
  if (filters.version)  params.set("version", filters.version);

  const response = await fetch(`${API_BASE}/rankings/champions?${params.toString()}`);
  if (!response.ok) throw new Error(`Request failed (${response.status})`);
  return response.json() as Promise<RankingsResponse>;
}

export async function fetchVersions(): Promise<string[]> {
  const response = await fetch(`${API_BASE}/versions`);
  if (!response.ok) throw new Error(`Failed to fetch versions (${response.status})`);
  const data = await response.json() as { versions: string[] };
  return data.versions;
}

export async function fetchRegions(): Promise<string[]> {
  const response = await fetch(`${API_BASE}/regions`);
  if (!response.ok) throw new Error(`Failed to fetch regions (${response.status})`);
  const data = await response.json() as { regions: string[] };
  return data.regions;
}
