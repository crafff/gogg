export const TFT_PLATFORMS = [
  "NA1",
  "BR1",
  "LA1",
  "LA2",
  "KR",
  "JP1",
  "EUN1",
  "EUW1",
  "TR1",
  "ME1",
  "RU",
  "OC1",
  "SG2",
  "TW2",
  "VN2",
] as const;

export type TftPlatform = (typeof TFT_PLATFORMS)[number];

export function normalizeTftPlatform(
  value: string | null | undefined,
): TftPlatform {
  const normalized = value?.trim().toUpperCase();
  return TFT_PLATFORMS.includes(normalized as TftPlatform)
    ? (normalized as TftPlatform)
    : "KR";
}
