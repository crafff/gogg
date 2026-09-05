import { normalizeTftPlatform } from "./platforms";

export function parseRiotID(value: string) {
  const trimmed = value.trim();
  const separator = trimmed.lastIndexOf("#");
  if (separator <= 0) return null;
  const gameName = trimmed.slice(0, separator).trim();
  const tagLine = trimmed.slice(separator + 1).trim();
  return gameName && tagLine ? { gameName, tagLine } : null;
}

export function tftPlayerPath(
  platform: string,
  gameName: string,
  tagLine: string,
) {
  return `/tft/player/${normalizeTftPlatform(platform)}/${encodeURIComponent(gameName)}/${encodeURIComponent(tagLine)}`;
}
