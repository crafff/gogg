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
});
