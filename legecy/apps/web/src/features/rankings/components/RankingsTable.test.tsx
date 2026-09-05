import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RankingsTable, type RankingRow } from "./RankingsTable";
import { sortRankings } from "./rankingSort";

const SAMPLE: RankingRow[] = [
  {
    championId: 99,
    championName: "Lux",
    teamPosition: ["MIDDLE"],
    games: 12500,
    wins: 6800,
    losses: 5700,
    winRate: 54.4,
    pickRate: 18,
    banRate: 7,
    kda: 2.31,
  },
];

describe("RankingsTable", () => {
  it("renders a row per item with formatted percentages and KDA", () => {
    render(<RankingsTable items={SAMPLE} />);

    expect(screen.getByText("Lux")).toBeInTheDocument();
    expect(screen.getByText("54.4%")).toBeInTheDocument();
    expect(screen.getByText("18.0%")).toBeInTheDocument();
    expect(screen.getByText("7.0%")).toBeInTheDocument();
    expect(screen.getByText("2.31")).toBeInTheDocument();
    expect(screen.getByText("12,500")).toBeInTheDocument();
  });

  it("supports composite and individual metric sorting", () => {
    const rows: RankingRow[] = [
      {
        ...SAMPLE[0]!,
        championId: 1,
        championName: "Balanced",
        winRate: 52,
        pickRate: 15,
        banRate: 12,
        kda: 3,
      },
      {
        ...SAMPLE[0]!,
        championId: 2,
        championName: "Win only",
        winRate: 60,
        pickRate: 1,
        banRate: 1,
        kda: 1,
      },
      {
        ...SAMPLE[0]!,
        championId: 3,
        championName: "Popular",
        winRate: 50,
        pickRate: 30,
        banRate: 20,
        kda: 4,
      },
    ];

    expect(sortRankings(rows, "composite", "desc")[0]!.championName).toBe(
      "Popular",
    );
    expect(sortRankings(rows, "winRate", "desc")[0]!.championName).toBe(
      "Win only",
    );
    expect(sortRankings(rows, "pickRate", "asc")[0]!.championName).toBe(
      "Win only",
    );
  });

  it("uses localized names and versioned images when assets are ready", () => {
    render(
      <RankingsTable
        items={SAMPLE}
        assetBaseURL="/game-assets/16.14"
        assets={{
          version: "16.14",
          champions: {
            "99": {
              id: 99,
              names: { en_us: "Lady of Luminosity" },
              image: "champions/99.png",
            },
          },
          positions: { middle: "positions/middle.png" },
        }}
      />,
    );
    expect(screen.getByText("Lady of Luminosity")).toBeInTheDocument();
    const images = screen.getAllByRole("presentation");
    expect(images[0]).toHaveAttribute(
      "src",
      "/game-assets/16.14/champions/99.png",
    );
  });
});
