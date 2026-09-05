import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { type Match, SummonerMatchCard } from "./SummonerMatchCard";

const participants: Match["participants"] = Array.from(
  { length: 10 },
  (_, index) => ({
    participantId: index + 1,
    teamId: index < 5 ? 100 : 200,
    isCurrentPlayer: index === 0,
    gameName: `Player ${index + 1}`,
    tagLine: "KR1",
    position: ["TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"][index % 5]!,
    win: index < 5,
    championId: index + 1,
    championName: `Champion ${index + 1}`,
    championLevel: 18,
    kills: index,
    deaths: 2,
    assists: 10 - index,
    kda: 5,
    minionsKilled: 200 + index,
    goldEarned: 12_000 + index,
    damageToChampions: 20_000 + index,
    visionScore: 20 + index,
    itemIds: [1001, 2003, 0, 0, 0, 0, 0],
    summonerSpellIds: [4, 12],
    primaryStyleId: 8100,
    secondaryStyleId: 8300,
    perkIds: [8112, 0, 0, 0, 0, 0],
    rank:
      index === 9
        ? null
        : index === 8
          ? {
              tier: "UNRANKED",
              division: null,
              leaguePoints: null,
              snapshotDeltaHours: 4,
            }
          : {
              tier: "EMERALD",
              division: "II",
              leaguePoints: 42,
              snapshotDeltaHours: 4,
            },
  }),
);

const match: Match = {
  matchId: "KR_123",
  queue: "RANKED_SOLO",
  queueId: 420,
  gameStartTime: "2026-08-26T12:00:00Z",
  durationSeconds: 1800,
  version: "16.16.1",
  endOfGameResult: "GameComplete",
  position: "MIDDLE",
  win: true,
  championId: 1,
  championName: "Annie",
  championLevel: 18,
  kills: 10,
  deaths: 2,
  assists: 8,
  kda: 9,
  minionsKilled: 230,
  csPerMinute: 7.7,
  goldEarned: 14_500,
  damageToChampions: 30_000,
  visionScore: 28,
  itemIds: [3089, 3020, 0, 0, 0, 0, 0],
  summonerSpellIds: [4, 12],
  primaryStyleId: 8100,
  secondaryStyleId: 8300,
  perkIds: [8112, 0, 0, 0, 0, 0],
  statShardIds: [5008, 5008, 5001],
  earlySurrender: false,
  surrender: false,
  averageTier: "EMERALD",
  averageDivision: "II",
  tierCoverage: 8,
  participants,
};

describe("SummonerMatchCard", () => {
  it("expands and collapses all ten participant details", async () => {
    const user = userEvent.setup();
    render(<SummonerMatchCard match={match} />);

    const trigger = screen.getByRole("button", { name: /10/ });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Player 10#KR1")).not.toBeInTheDocument();
    expect(screen.getByText(/8\/10/)).toBeInTheDocument();

    await user.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("region", { name: /10/ })).toBeInTheDocument();
    for (let index = 1; index <= 10; index += 1) {
      expect(screen.getByText(`Player ${index}#KR1`)).toBeInTheDocument();
    }
    expect(screen.getByText(/蓝方|Blue team/)).toBeInTheDocument();
    expect(screen.getByText(/红方|Red team/)).toBeInTheDocument();
    expect(screen.getAllByText(/流光翡翠|Emerald/).length).toBeGreaterThan(1);
    expect(screen.getByText(/待补齐|Pending/)).toBeInTheDocument();

    await user.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByRole("region", { name: /10/ }),
    ).not.toBeInTheDocument();
  });
});
