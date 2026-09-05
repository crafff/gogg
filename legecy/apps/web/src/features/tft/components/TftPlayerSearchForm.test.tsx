import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { describe, expect, it } from "vitest";

import { parseRiotID, tftPlayerPath } from "../lib/playerIdentity";
import { TftPlayerSearchForm } from "./TftPlayerSearchForm";

function Location() {
  return <output data-testid="location">{useLocation().pathname}</output>;
}

describe("TftPlayerSearchForm", () => {
  it("validates the Riot ID and keeps the platform in the route", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={["/tft/player"]}>
        <TftPlayerSearchForm initialPlatform="EUW1" />
        <Location />
      </MemoryRouter>,
    );

    await user.type(
      screen.getByLabelText(/Game name|游戏名称/),
      "Player One#EUW",
    );
    await user.click(screen.getByRole("button", { name: /Search|查询/ }));
    expect(screen.getByTestId("location")).toHaveTextContent(
      "/tft/player/EUW1/Player%20One/EUW",
    );
  });

  it("rejects a value without a tag", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <TftPlayerSearchForm />
      </MemoryRouter>,
    );
    await user.type(screen.getByLabelText(/Game name|游戏名称/), "NoTag");
    await user.click(screen.getByRole("button", { name: /Search|查询/ }));
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

describe("TFT Riot ID parsing", () => {
  it("uses the final hash and encodes route segments", () => {
    expect(parseRiotID(" Alpha # One #KR1 ")).toEqual({
      gameName: "Alpha # One",
      tagLine: "KR1",
    });
    expect(tftPlayerPath("na1", "A B", "N/A")).toBe(
      "/tft/player/NA1/A%20B/N%2FA",
    );
  });
});
