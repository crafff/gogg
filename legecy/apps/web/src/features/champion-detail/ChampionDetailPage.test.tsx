import "@shared/i18n";

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { GameAssetEntry } from "@features/rankings/hooks/useGameAssets";

import {
  RuneChoice,
  WinFactorCard,
  type WinFactor,
} from "./ChampionDetailPage";

function entry(id: number, name: string): GameAssetEntry {
  return {
    id,
    names: { en_us: name, zh_cn: name },
    image: `${id}.png`,
  };
}

describe("RuneChoice", () => {
  it("separates four primary runes, two secondary runes, and three stat shards", () => {
    const perks = Object.fromEntries(
      [
        entry(8000, "Precision"),
        entry(8300, "Inspiration"),
        ...Array.from({ length: 9 }, (_, index) =>
          entry(index + 1, `Rune ${index + 1}`),
        ),
      ].map((asset) => [String(asset.id), asset]),
    );

    render(
      <RuneChoice
        build={{
          primaryStyleId: 8000,
          secondaryStyleId: 8300,
          perkIds: [1, 2, 3, 4, 5, 6],
          statShardIds: [7, 8, 9],
          games: 100,
          eligibleGames: 200,
          pickRate: 50,
          winRate: 52,
        }}
        perks={perks}
        locale="en_us"
        base="/game-assets/16.14"
      />,
    );

    const primary = screen.getByRole("group", {
      name: /Primary Precision|主系 Precision/,
    });
    const secondary = screen.getByRole("group", {
      name: /Secondary Inspiration|副系 Inspiration/,
    });
    const shards = screen.getByRole("group", { name: /Stat shards|属性碎片/ });

    expect(within(primary).getAllByRole("img")).toHaveLength(5);
    expect(within(secondary).getAllByRole("img")).toHaveLength(3);
    expect(within(shards).getAllByRole("img")).toHaveLength(3);
    expect(screen.queryByText("8000")).not.toBeInTheDocument();
    expect(screen.queryByText("8300")).not.toBeInTheDocument();
  });
});

describe("WinFactorCard", () => {
  it("renders observed rates and match/player sample sizes as text", () => {
    const factor: WinFactor = {
      metricKey: "JUNGLE_CS_10",
      kind: "BEHAVIOR_METRIC",
      startMinute: 0,
      endMinute: 10,
      unit: "COUNT",
      p50: 60,
      p70: 70,
      p90: 80,
      evidenceGrade: "OBSERVED",
      displayOrder: 1,
      buckets: [
        {
          ordinal: 1,
          lowerBound: 30,
          upperBound: 59,
          games: 500,
          wins: 220,
          samplePlayers: 175,
          observedWinRate: 44,
          observedWinRateDelta: -6,
        },
      ],
    };

    render(<WinFactorCard factor={factor} />);

    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByText("44.0%")).toBeInTheDocument();
    expect(screen.getByText("-6.0 pp")).toBeInTheDocument();
    expect(screen.getByText("500")).toBeInTheDocument();
    expect(screen.getByText("175")).toBeInTheDocument();
  });
});
