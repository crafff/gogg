import { describe, expect, it } from "vitest";

import {
  ChampionRankingsDocument,
  RegionsDocument,
  VersionsDocument,
  useChampionRankingsQuery,
  useRegionsQuery,
  useVersionsQuery,
  queryClient,
} from "./index";

// Smoke test for the codegen output. We don't fire a real fetch here —
// the GraphQL endpoint isn't running during unit tests — we just
// assert the generated artifacts are shaped the way feature code
// expects (functions exist, query documents are strings, query keys
// follow the operation-name convention).
describe("generated GraphQL hooks", () => {
  it("exposes useQuery hooks for each operation", () => {
    expect(useVersionsQuery).toBeTypeOf("function");
    expect(useRegionsQuery).toBeTypeOf("function");
    expect(useChampionRankingsQuery).toBeTypeOf("function");
  });

  it("exposes the document strings under <Operation>Document", () => {
    // typescript-react-query wraps documents in TypedDocumentString
    // which subclasses String — coercing to a plain string gives the
    // raw query body.
    expect(String(VersionsDocument)).toMatch(/query Versions/);
    expect(String(RegionsDocument)).toMatch(/query Regions/);
    expect(String(ChampionRankingsDocument)).toMatch(/query ChampionRankings/);
  });

  it("derives stable query keys via .getKey", () => {
    expect(useVersionsQuery.getKey()).toEqual(["Versions"]);
    expect(useRegionsQuery.getKey()).toEqual(["Regions"]);

    const vars = { filter: { region: "KR" } };
    expect(useChampionRankingsQuery.getKey(vars)).toEqual([
      "ChampionRankings",
      vars,
    ]);
  });

  it("exports a singleton QueryClient with the shared defaults", () => {
    const opts = queryClient.getDefaultOptions();
    expect(opts.queries?.staleTime).toBe(60_000);
    expect(opts.queries?.refetchOnWindowFocus).toBe(false);
  });
});
