import { test, expect, type Route } from "@playwright/test";

// Golden path for the rankings page:
//   1. Visit /            → redirect lands on /rankings.
//   2. Initial query renders 40 champions in the default slice.
//   3. Click the "Top" position chip → second query fires with
//      position: "TOP" and the table re-renders.
//   4. Scroll the sentinel into view → infinite scroll triggers a
//      third query with limit: 80 and the row count grows.
//
// All network responses are stubbed via page.route so the test is
// hermetic — no need to boot the backend.

interface GraphQLRequest {
  query: string;
  variables?: {
    filter?: {
      position?: string;
      tierGroup?: string;
      region?: string;
      version?: string;
      minGames?: number;
      limit?: number;
    };
  };
}

// typescript-react-query's fetcher posts { query, variables } without
// an operationName field, so we recover the operation by sniffing the
// first "query Name" token out of the document body.
function detectOperation(req: GraphQLRequest): string {
  const match = req.query.match(/query\s+(\w+)/);
  return match?.[1] ?? "unknown";
}

function mockRow(id: number, name: string, position: string) {
  return {
    championId: id,
    championName: name,
    teamPosition: [position],
    games: 1000 + id,
    wins: 540 + id,
    losses: 460,
    winRate: 0.53 + id / 1000,
    pickRate: 0.12,
    banRate: 0.05,
    kda: 2.1 + id / 1000,
  };
}

function makeRows(count: number, position: string) {
  return Array.from({ length: count }, (_, i) =>
    mockRow(i + 1, `Champion-${position}-${i + 1}`, position),
  );
}

test.describe("rankings golden path", () => {
  test("loads default slice, swaps position, paginates on scroll", async ({
    page,
  }) => {
    const seen: GraphQLRequest[] = [];

    await page.route("**/graphql", async (route: Route) => {
      const body = JSON.parse(
        route.request().postData() ?? "{}",
      ) as GraphQLRequest;
      seen.push(body);

      const op = detectOperation(body);

      if (op === "Versions") {
        await route.fulfill({
          contentType: "application/json",
          body: JSON.stringify({ data: { versions: ["14.20", "14.19"] } }),
        });
        return;
      }
      if (op === "Regions") {
        await route.fulfill({
          contentType: "application/json",
          body: JSON.stringify({ data: { regions: ["KR", "NA1"] } }),
        });
        return;
      }
      if (op === "ChampionRankings") {
        const filter = body.variables?.filter ?? {};
        const limit = filter.limit ?? 40;
        const position = filter.position || "ALL";
        await route.fulfill({
          contentType: "application/json",
          body: JSON.stringify({
            data: {
              championRankings: {
                totalMatches: 12_500,
                resolvedVersion: "14.20.1",
                items: makeRows(limit, position),
              },
            },
          }),
        });
        return;
      }
      await route.fulfill({
        status: 500,
        body: `unhandled op: ${op ?? "<none>"}`,
      });
    });

    // 1. / → /rankings redirect
    await page.goto("/");
    await expect(page).toHaveURL(/\/rankings$/);

    // 2. Initial query renders 40 rows
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: /Champion rankings|英雄排行/,
      }),
    ).toBeVisible();
    await expect(page.getByText("Champion-ALL-1")).toBeVisible();
    await expect(page.getByText("Champion-ALL-40")).toBeVisible();

    // 3. Click the TOP position chip — find it by label, in either locale.
    const topChip = page.getByRole("button", { name: /^Top$|^上单$/ });
    await topChip.click();
    await expect(page.getByText("Champion-TOP-1")).toBeVisible();
    await expect(page.getByText("Champion-TOP-40")).toBeVisible();

    const championRequests = seen.filter(
      (r) => detectOperation(r) === "ChampionRankings",
    );
    expect(championRequests.length).toBeGreaterThanOrEqual(2);
    const lastRequest = championRequests[championRequests.length - 1];
    expect(lastRequest?.variables?.filter?.position).toBe("TOP");

    // 4. Scroll the sentinel into view → limit grows to 80
    await page.getByTestId("rankings-load-more").scrollIntoViewIfNeeded();
    await expect(page.getByText("Champion-TOP-80")).toBeVisible();

    const grown = seen
      .filter((r) => detectOperation(r) === "ChampionRankings")
      .map((r) => r.variables?.filter?.limit ?? 0);
    expect(grown.some((l) => l >= 80)).toBe(true);
  });
});
