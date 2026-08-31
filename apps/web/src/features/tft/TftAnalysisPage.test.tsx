import "@shared/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TftAnalysisPage } from "./TftAnalysisPage";

function renderPage(path = "/tft") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <TftAnalysisPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body ?? "{}")) as {
        query?: string;
      };
      if (body.query?.includes("query TFTAnalysisCatalog")) {
        return Response.json({ data: { tftAnalysisCatalog: [] } });
      }
      if (body.query?.includes("query TFTObservedLineups")) {
        return Response.json({
          data: {
            tftObservedLineups: {
              dataKind: "OBSERVED_RUN_PREVIEW",
              runId: "5",
              platform: "KR",
              platforms: ["KR", "NA1"],
              queueId: 1100,
              setNumber: 18,
              patch: null,
              rawGameVersions: ["TFT Unreal Version ?.?.?.?"],
              locale: "zh_cn",
              algorithmVersion: "exact-observed-board-items-stars-v3",
              catalogSnapshot: {
                source: "ddragon",
                patch: "16.17",
                revision: "catalog-rev",
              },
              assetSnapshot: {
                source: "cdragon",
                patch: "16.17",
                revision: "static-rev",
              },
              sourceMatches: 946,
              sourceParticipants: 7568,
              usableParticipants: 6838,
              exactLineups: 3675,
              windowStart: "2026-08-26T21:07:43Z",
              windowEnd: "2026-08-29T18:07:59Z",
              items: [
                {
                  id: "lineup-1",
                  coreUnits: [
                    {
                      id: "DA_18_Aphelios",
                      name: "厄斐琉斯",
                      iconUrl: "/game-assets/tft/aphelios.png",
                    },
                    {
                      id: "DA_18_Xayah",
                      name: "霞",
                      iconUrl: "/game-assets/tft/xayah.png",
                    },
                  ],
                  commonItems: [],
                  commonAugments: [],
                  commonTraits: [],
                  unitItems: [
                    {
                      unit: {
                        id: "DA_18_Aphelios",
                        name: "厄斐琉斯",
                        iconUrl: "/game-assets/tft/aphelios.png",
                      },
                      commonItems: [
                        {
                          entity: {
                            id: "DA_GuinsoosRageblade",
                            name: "鬼索的狂暴之刃",
                            iconUrl: "/game-assets/tft/guinsoos.png",
                          },
                          count: 40,
                          rate: 40 / 58,
                        },
                      ],
                      isCore: true,
                      coreRank: 1,
                      averageItems: 2.95,
                      itemInvestmentRate: 2.95 / 3,
                      equippedRate: 1,
                      threeItemRate: 0.98,
                      knownStarSamples: 57,
                      unknownStarSamples: 1,
                      starCoverage: 57 / 58,
                      starDistribution: [
                        {
                          stars: 2,
                          sampleSize: 1,
                          rate: 1 / 58,
                          knownRate: 1 / 57,
                          avgPlacement: null,
                          firstRate: null,
                          top4Rate: null,
                        },
                        {
                          stars: 3,
                          sampleSize: 56,
                          rate: 56 / 58,
                          knownRate: 56 / 57,
                          avgPlacement: 2.58,
                          firstRate: 0.37,
                          top4Rate: 0.88,
                        },
                      ],
                    },
                    {
                      unit: {
                        id: "DA_18_Xayah",
                        name: "霞",
                        iconUrl: "/game-assets/tft/xayah.png",
                      },
                      commonItems: [],
                      isCore: true,
                      coreRank: 2,
                      averageItems: 2.41,
                      itemInvestmentRate: 2.41 / 3,
                      equippedRate: 0.9,
                      threeItemRate: 0.72,
                      knownStarSamples: 58,
                      unknownStarSamples: 0,
                      starCoverage: 1,
                      starDistribution: [
                        {
                          stars: 2,
                          sampleSize: 2,
                          rate: 2 / 58,
                          knownRate: 2 / 58,
                          avgPlacement: null,
                          firstRate: null,
                          top4Rate: null,
                        },
                        {
                          stars: 3,
                          sampleSize: 56,
                          rate: 56 / 58,
                          knownRate: 56 / 58,
                          avgPlacement: 2.6,
                          firstRate: 0.36,
                          top4Rate: 0.87,
                        },
                      ],
                    },
                  ],
                  starLevels: [
                    {
                      totalStars: 20,
                      sampleSize: 15,
                      lobbyCount: 15,
                      rate: 15 / 58,
                      avgPlacement: 2.73,
                      firstRate: 0.27,
                      top4Rate: 0.73,
                    },
                    {
                      totalStars: 21,
                      sampleSize: 23,
                      lobbyCount: 23,
                      rate: 23 / 58,
                      avgPlacement: 2.17,
                      firstRate: 0.43,
                      top4Rate: 1,
                    },
                  ],
                  starCompositionKnownSamples: 58,
                  starCompositionUnknownSamples: 0,
                  starCompositionCoverage: 1,
                  starCompositions: [
                    {
                      levels: [
                        { stars: 2, unitCount: 4 },
                        { stars: 3, unitCount: 4 },
                      ],
                      totalStars: 20,
                      sampleSize: 15,
                      rate: 15 / 58,
                      avgPlacement: 2.73,
                      firstRate: 0.27,
                      top4Rate: 0.73,
                    },
                    {
                      levels: [
                        { stars: 2, unitCount: 3 },
                        { stars: 3, unitCount: 5 },
                      ],
                      totalStars: 21,
                      sampleSize: 23,
                      rate: 23 / 58,
                      avgPlacement: 2.17,
                      firstRate: 0.43,
                      top4Rate: 1,
                    },
                  ],
                  metrics: {
                    sampleSize: 58,
                    lobbyCount: 58,
                    pickRate: 0.0085,
                    avgPlacement: 2.62,
                    firstRate: 0.36,
                    top4Rate: 0.86,
                    contestedRate: 0,
                  },
                },
              ],
            },
          },
        });
      }
      return Response.json({ data: {} });
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("TftAnalysisPage observed preview", () => {
  it("renders real-run provenance, coverage, and lineup cards", async () => {
    renderPage();

    expect(
      await screen.findByText(/Observed match preview|实战样本预览/),
    ).toBeInTheDocument();
    expect(screen.getByText(/厄斐琉斯.*霞/)).toBeInTheDocument();
    expect(screen.getByText("2.62")).toBeInTheDocument();
    expect(screen.getAllByText("鬼索的狂暴之刃")).toHaveLength(2);
    expect(screen.getAllByText(/Item core 1|装备主力 1/)).toHaveLength(2);
    expect(
      screen.getByLabelText(/厄斐琉斯 3 星|厄斐琉斯 at 3 stars/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /星级记录 57 个，缺失 1 个，覆盖率 98%|57 recorded, 1 missing, 98% coverage/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("2★×4 · 3★×4")).toBeInTheDocument();
    expect(screen.getByText("2.73")).toBeInTheDocument();
    expect(screen.getByText("73%")).toBeInTheDocument();
    expect(
      screen.getByText(/Unknown \(masked by Riot\)|未知（Riot 已屏蔽）/),
    ).toBeInTheDocument();
    expect(screen.getByText("6,838")).toBeInTheDocument();
    expect(
      screen.getByText(
        /ddragon 16\.17 \(catalog-\).*cdragon 16\.17 \(static-r\)/,
      ),
    ).toBeInTheDocument();

    await waitFor(() => {
      const requests = vi.mocked(fetch).mock.calls.map(
        ([, init]) =>
          JSON.parse(String(init?.body ?? "{}")) as {
            query?: string;
            variables?: Record<string, unknown>;
          },
      );
      const preview = requests.find((request) =>
        request.query?.includes("query TFTObservedLineups"),
      );
      expect(preview?.variables).toEqual({
        filter: {
          platform: "KR",
          locale: "en_us",
          minSamples: 20,
          limit: 30,
        },
      });
    });
  });
});
