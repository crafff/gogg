// Application-wide constants

export const POSITIONS = [
  { value: "", label: "ALL" },
  { value: "TOP", label: "TOP" },
  { value: "JUNGLE", label: "JUNGLE" },
  { value: "MIDDLE", label: "MIDDLE" },
  { value: "BOTTOM", label: "BOTTOM" },
  { value: "UTILITY", label: "UTILITY" },
] as const;

export const DEFAULT_FILTERS = {
  limit: 20,
  minGames: 20,
  position: "" as const,
};
