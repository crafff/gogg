import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useRankingsQuery } from "./useRankingsQuery";

// Hits the codegen'd useChampionRankingsQuery under the hood, which
// in turn calls the native-fetch fetcher emitted by typescript-react-
// query. We stub global.fetch to capture the GraphQL payload and
// return canned data — the test is the end-to-end contract between
// UI filter state and the network payload the backend will see.

interface CapturedRequest {
  variables: {
    filter: {
      position?: string;
      tierGroup?: string;
      region?: string;
      version?: string;
      minGames?: number;
      limit?: number;
    };
  };
}

let captured: CapturedRequest | null = null;

function Wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  captured = null;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) => {
      captured = JSON.parse(String(init?.body ?? "{}"));
      return {
        ok: true,
        json: async () => ({
          data: {
            championRankings: {
              totalMatches: 42,
              resolvedVersion: "14.20.1",
              items: [
                {
                  championId: 99,
                  championName: "Lux",
                  teamPosition: ["MIDDLE"],
                  games: 100,
                  wins: 55,
                  losses: 45,
                  winRate: 0.55,
                  pickRate: 0.1,
                  banRate: 0.05,
                  kda: 2.1,
                },
              ],
            },
          },
        }),
      };
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useRankingsQuery", () => {
  it("projects the UI filter into the GraphQL filter shape", async () => {
    const { result } = renderHook(
      () =>
        useRankingsQuery({
          filters: {
            position: "TOP",
            tier: "challenger",
            region: "KR",
            version: "latest",
          },
          limit: 40,
        }),
      { wrapper: Wrapper },
    );

    await waitFor(() => expect(result.current.items.length).toBe(1));

    expect(captured).not.toBeNull();
    expect(captured!.variables.filter).toMatchObject({
      position: "TOP",
      tierGroup: "CHALLENGER",
      region: "KR",
      version: "latest",
      minGames: 20,
      limit: 40,
    });
  });

  it("flattens the response into items + totalMatches + resolvedVersion", async () => {
    const { result } = renderHook(
      () =>
        useRankingsQuery({
          filters: {
            position: "",
            tier: "",
            region: "",
            version: "latest",
          },
          limit: 40,
        }),
      { wrapper: Wrapper },
    );

    await waitFor(() => expect(result.current.totalMatches).toBe(42));

    expect(result.current.resolvedVersion).toBe("14.20.1");
    expect(result.current.items[0]?.championName).toBe("Lux");
    expect(result.current.isError).toBe(false);
  });

  it("does not fire fetch when disabled", async () => {
    renderHook(
      () =>
        useRankingsQuery({
          filters: {
            position: "",
            tier: "",
            region: "",
            version: "latest",
          },
          limit: 40,
          enabled: false,
        }),
      { wrapper: Wrapper },
    );

    // Give react-query a microtask to spin up; if disabled is
    // respected, fetch stays untouched.
    await new Promise((r) => setTimeout(r, 0));
    expect(captured).toBeNull();
  });
});
