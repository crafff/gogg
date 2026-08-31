import "@shared/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { routes } from "./router";

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body ?? "{}")) as { query?: string };
      if (body.query?.includes("query Me")) {
        return Response.json({ data: { me: null } });
      }
      if (body.query?.includes("query AuthProviders")) {
        return Response.json({ data: { authProviders: [{ id: "google" }] } });
      }
      if (body.query?.includes("query Versions")) {
        return Response.json({ data: { versions: ["16.15", "16.14"] } });
      }
      if (body.query?.includes("query Regions")) {
        return Response.json({ data: { regions: ["KR", "NA1"] } });
      }
      return Response.json({ data: {} });
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("router", () => {
  it("redirects / to /rankings", async () => {
    renderAt("/");
    // Rankings page is lazy-loaded; wait for the heading to land.
    expect(
      await screen.findByRole("heading", { level: 1 }, { timeout: 5_000 }),
    ).toHaveTextContent(/Champion rankings|英雄排行/);
  });

  it("renders the champion-detail page with the URL param", async () => {
    renderAt("/champion/99");
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      "#99",
    );
  });

  it("loads only win factors for a route-addressable factors view", async () => {
    renderAt(
      "/champion/421?view=factors&position=JUNGLE&region=KR&tier=master_plus&version=16.15",
    );

    await waitFor(() => {
      const operations = vi.mocked(fetch).mock.calls.map(
        ([, init]) =>
          JSON.parse(String(init?.body ?? "{}")) as {
            query?: string;
            variables?: Record<string, unknown>;
          },
      );
      expect(
        operations.some((operation) =>
          operation.query?.includes("query ChampionWinFactors"),
        ),
      ).toBe(true);
    });

    const operations = vi.mocked(fetch).mock.calls.map(
      ([, init]) =>
        JSON.parse(String(init?.body ?? "{}")) as {
          query?: string;
          variables?: {
            id?: number;
            filter?: Record<string, unknown>;
          };
        },
    );
    const factors = operations.find((operation) =>
      operation.query?.includes("query ChampionWinFactors"),
    );
    expect(factors?.variables).toEqual({
      id: 421,
      filter: {
        queueId: 420,
        position: "JUNGLE",
        region: "KR",
        version: "16.15",
        tierGroup: "MASTER_PLUS",
      },
    });
    expect(
      operations.some((operation) =>
        operation.query?.includes("query ChampionDetail"),
      ),
    ).toBe(false);

    const overview = await screen.findByRole("link", {
      name: /Overview|概览/,
    });
    expect(overview.getAttribute("href")).toContain("position=JUNGLE");
    expect(overview.getAttribute("href")).toContain("region=KR");
    expect(overview.getAttribute("href")).not.toContain("view=factors");
  });

  it("renders the summoner Riot ID search", async () => {
    renderAt("/summoner");
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      /Summoner match history|查询召唤师战绩/,
    );
    expect(
      screen.getByRole("form", { name: /Summoner search|召唤师搜索/ }),
    ).toBeInTheDocument();
  });

  it("renders the TFT analysis route", async () => {
    renderAt("/tft");
    expect(
      await screen.findByText(
        /No lineup analysis has been published|还没有可用的阵容分析/,
      ),
    ).toBeInTheDocument();
  });

  it("renders the TFT player search across all supported platforms", async () => {
    renderAt("/tft/player");
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      /TFT match history|查询 TFT 战绩/,
    );
    expect(
      screen.getByRole("form", { name: /TFT player search|TFT 玩家搜索/ }),
    ).toBeInTheDocument();
  });

  it("renders the configured Google login", async () => {
    renderAt("/login");
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      /Sign in|登录/,
    );
    expect(await screen.findByRole("link", { name: /Google/ })).toHaveAttribute(
      "href",
      "/oauth/start/google?returnTo=%2Fme",
    );
  });

  it("redirects anonymous /me visitors to login", async () => {
    renderAt("/me");
    expect(await screen.findByRole("link", { name: /Google/ })).toHaveAttribute(
      "href",
      "/oauth/start/google?returnTo=%2Fme",
    );
  });

  it("renders the route error boundary for unknown paths", async () => {
    renderAt("/this/route/does/not/exist");
    expect(await screen.findByTestId("route-error")).toBeInTheDocument();
  });
});
