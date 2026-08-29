import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it } from "vitest";

import { SummonerSearchForm } from "./SummonerSearchForm";

function Location() {
  return <output data-testid="location">{useLocation().pathname}</output>;
}

function renderForm() {
  return render(
    <MemoryRouter initialEntries={["/summoner"]}>
      <SummonerSearchForm />
      <Location />
    </MemoryRouter>,
  );
}

describe("SummonerSearchForm", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("navigates using encoded Riot ID route segments", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByLabelText("Riot ID"), "Hide on bush#KR1");
    await user.click(screen.getByRole("button", { name: /查询|Search/ }));

    expect(screen.getByTestId("location")).toHaveTextContent(
      "/summoner/KR/Hide%20on%20bush/KR1",
    );
    expect(
      JSON.parse(window.localStorage.getItem("gogg:recent-summoners") ?? "[]"),
    ).toEqual([{ region: "KR", gameName: "Hide on bush", tagLine: "KR1" }]);
  });

  it("rejects an ID without a tag", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(screen.getByLabelText("Riot ID"), "Faker");
    await user.click(screen.getByRole("button", { name: /查询|Search/ }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      /游戏名#标签|game name#tag/,
    );
    expect(screen.getByTestId("location")).toHaveTextContent("/summoner");
  });

  it("restores the last selected region on a later visit", () => {
    window.localStorage.setItem("gogg:summoner-region", "NA1");

    renderForm();

    expect(screen.getByLabelText(/服务器|Region/)).toHaveTextContent("NA1");
  });

  it("opens a cached recent summoner", async () => {
    const user = userEvent.setup();
    window.localStorage.setItem(
      "gogg:recent-summoners",
      JSON.stringify([{ region: "NA1", gameName: "Nyven", tagLine: "999" }]),
    );
    renderForm();

    await user.click(screen.getByRole("button", { name: "NA1 Nyven#999" }));

    expect(screen.getByTestId("location")).toHaveTextContent(
      "/summoner/NA1/Nyven/999",
    );
    expect(screen.getByLabelText("Riot ID")).toHaveValue("Nyven#999");
    expect(screen.getByLabelText(/服务器|Region/)).toHaveTextContent("NA1");
  });

  it("keeps the five most recent unique summoners", async () => {
    const user = userEvent.setup();
    window.localStorage.setItem(
      "gogg:recent-summoners",
      JSON.stringify(
        Array.from({ length: 5 }, (_, index) => ({
          region: "KR",
          gameName: `Player${index + 1}`,
          tagLine: "KR1",
        })),
      ),
    );
    renderForm();

    await user.type(screen.getByLabelText("Riot ID"), "New Player#NA1");
    await user.click(screen.getByRole("button", { name: /查询|Search/ }));

    const stored = JSON.parse(
      window.localStorage.getItem("gogg:recent-summoners") ?? "[]",
    );
    expect(stored).toHaveLength(5);
    expect(stored[0]).toEqual({
      region: "KR",
      gameName: "New Player",
      tagLine: "NA1",
    });
    expect(stored.at(-1)).toEqual({
      region: "KR",
      gameName: "Player4",
      tagLine: "KR1",
    });
  });
});
