export interface TftCatalogSelection {
  platform: string;
  patch: string;
  setNumber: number;
  cohort: string;
  window: string;
}

export interface TftCatalogEntry extends TftCatalogSelection {
  publishedAt: string;
}

export const DEFAULT_MIN_SAMPLES = 200;
export const DEFAULT_PREVIEW_MIN_SAMPLES = 20;

export function readMinSamples(value: string | null): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) return DEFAULT_MIN_SAMPLES;
  return Math.min(parsed, 100_000);
}

export function readPreviewMinSamples(value: string | null): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < DEFAULT_PREVIEW_MIN_SAMPLES)
    return DEFAULT_PREVIEW_MIN_SAMPLES;
  return Math.min(parsed, 100_000);
}

export function selectCatalogEntry(
  entries: readonly TftCatalogEntry[],
  requested: Partial<TftCatalogSelection>,
): TftCatalogEntry | null {
  if (entries.length === 0) return null;

  const sorted = [...entries].sort((left, right) =>
    right.publishedAt.localeCompare(left.publishedAt),
  );
  const requestedPlatform = requested.platform?.toUpperCase();
  const platform = sorted.some((entry) => entry.platform === requestedPlatform)
    ? requestedPlatform
    : sorted.some((entry) => entry.platform === "KR")
      ? "KR"
      : sorted[0]!.platform;
  let candidates = sorted.filter((entry) => entry.platform === platform);

  const patch = requested.patch;
  if (patch && candidates.some((entry) => entry.patch === patch)) {
    candidates = candidates.filter((entry) => entry.patch === patch);
  } else {
    const latestPatch = candidates[0]!.patch;
    candidates = candidates.filter((entry) => entry.patch === latestPatch);
  }

  if (
    requested.setNumber !== undefined &&
    candidates.some((entry) => entry.setNumber === requested.setNumber)
  ) {
    candidates = candidates.filter(
      (entry) => entry.setNumber === requested.setNumber,
    );
  } else {
    const latestSet = Math.max(...candidates.map((entry) => entry.setNumber));
    candidates = candidates.filter((entry) => entry.setNumber === latestSet);
  }

  if (
    requested.cohort &&
    candidates.some((entry) => entry.cohort === requested.cohort)
  ) {
    candidates = candidates.filter(
      (entry) => entry.cohort === requested.cohort,
    );
  } else if (candidates.some((entry) => entry.cohort === "MASTER_PLUS")) {
    candidates = candidates.filter((entry) => entry.cohort === "MASTER_PLUS");
  }

  if (
    requested.window &&
    candidates.some((entry) => entry.window === requested.window)
  ) {
    candidates = candidates.filter(
      (entry) => entry.window === requested.window,
    );
  } else if (candidates.some((entry) => entry.window === "PATCH")) {
    candidates = candidates.filter((entry) => entry.window === "PATCH");
  }

  return candidates[0] ?? null;
}

export function catalogOptions(
  entries: readonly TftCatalogEntry[],
  selection: TftCatalogSelection,
) {
  const forPlatform = entries.filter(
    (entry) => entry.platform === selection.platform,
  );
  const forPatch = forPlatform.filter(
    (entry) => entry.patch === selection.patch,
  );
  const forSet = forPatch.filter(
    (entry) => entry.setNumber === selection.setNumber,
  );
  const forCohort = forSet.filter((entry) => entry.cohort === selection.cohort);

  return {
    platforms: unique(entries.map((entry) => entry.platform)),
    patches: unique(forPlatform.map((entry) => entry.patch)),
    sets: unique(forPatch.map((entry) => entry.setNumber)),
    cohorts: unique(forSet.map((entry) => entry.cohort)),
    windows: unique(forCohort.map((entry) => entry.window)),
  };
}

function unique<T>(values: readonly T[]): T[] {
  return [...new Set(values)];
}
