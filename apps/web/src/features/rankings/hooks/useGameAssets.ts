import { useQuery } from "@tanstack/react-query";

export interface GameAssetChampion {
  id: number;
  names: Record<string, string>;
  image: string;
}

export interface GameAssetManifest {
  schemaVersion?: number;
  version: string;
  champions: Record<string, GameAssetChampion>;
  positions?: Record<string, string>;
  items?: Record<string, GameAssetEntry>;
  summonerSpells?: Record<string, GameAssetEntry>;
  perks?: Record<string, GameAssetEntry>;
}

export interface GameAssetEntry {
  id: number;
  names: Record<string, string>;
  image: string;
}

function assetPatch(version: string): string {
  return version.split(".").slice(0, 2).join(".");
}

export function useGameAssets(version: string | null) {
  const patch = version ? assetPatch(version) : "";
  const query = useQuery({
    queryKey: ["game-assets", patch],
    enabled: patch !== "",
    staleTime: Infinity,
    retry: 1,
    queryFn: async (): Promise<GameAssetManifest> => {
      const response = await fetch(`/game-assets/${patch}/manifest.json`);
      if (!response.ok) throw new Error(`game assets HTTP ${response.status}`);
      return response.json() as Promise<GameAssetManifest>;
    },
  });
  return {
    manifest: query.data ?? null,
    baseURL: patch ? `/game-assets/${patch}` : "",
  };
}
