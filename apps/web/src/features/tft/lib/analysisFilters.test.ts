import { describe, expect, it } from "vitest";

import {
  catalogOptions,
  readMinSamples,
  selectCatalogEntry,
  type TftCatalogEntry,
} from "./analysisFilters";

const entries: TftCatalogEntry[] = [
  {
    platform: "KR",
    patch: "16.17",
    setNumber: 15,
    cohort: "MASTER_PLUS",
    window: "PATCH",
    publishedAt: "2026-08-29T12:00:00Z",
  },
  {
    platform: "KR",
    patch: "16.17",
    setNumber: 15,
    cohort: "DIAMOND",
    window: "THREE_DAYS",
    publishedAt: "2026-08-29T11:00:00Z",
  },
  {
    platform: "NA1",
    patch: "16.16",
    setNumber: 15,
    cohort: "MASTER_PLUS",
    window: "PATCH",
    publishedAt: "2026-08-28T12:00:00Z",
  },
];

describe("TFT analysis filters", () => {
  it("falls back to a real KR publication instead of inventing a combination", () => {
    expect(
      selectCatalogEntry(entries, { platform: "EUW1", patch: "latest" }),
    ).toMatchObject({
      platform: "KR",
      patch: "16.17",
      cohort: "MASTER_PLUS",
      window: "PATCH",
    });
  });

  it("preserves an exact published combination", () => {
    expect(
      selectCatalogEntry(entries, {
        platform: "KR",
        patch: "16.17",
        setNumber: 15,
        cohort: "DIAMOND",
        window: "THREE_DAYS",
      }),
    ).toEqual(entries[1]);
  });

  it("only offers dimensions backed by the current catalog branch", () => {
    expect(catalogOptions(entries, entries[0]!)).toEqual({
      platforms: ["KR", "NA1"],
      patches: ["16.17"],
      sets: [15],
      cohorts: ["MASTER_PLUS", "DIAMOND"],
      windows: ["PATCH"],
    });
  });

  it("bounds invalid sample input", () => {
    expect(readMinSamples("0")).toBe(200);
    expect(readMinSamples("1250")).toBe(1250);
    expect(readMinSamples("9999999")).toBe(100_000);
  });
});
