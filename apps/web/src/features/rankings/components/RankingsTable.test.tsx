import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RankingsTable, type RankingRow } from "./RankingsTable";

const SAMPLE: RankingRow[] = [
  {
    championId: 99,
    championName: "Lux",
    teamPosition: ["MIDDLE"],
    games: 12500,
    wins: 6800,
    losses: 5700,
    winRate: 0.544,
    pickRate: 0.18,
    banRate: 0.07,
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
