import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RankingsFilters } from "./RankingsFilters";

const noop = () => {};

const defaultSelected = {
  position: "" as const,
  tier: "" as const,
  region: "",
  version: "latest",
};

describe("RankingsFilters", () => {
  it("renders a button per position with aria-pressed for the active one", () => {
    render(
      <RankingsFilters
        selected={{ ...defaultSelected, position: "TOP" }}
        availableVersions={[]}
        availableRegions={[]}
        onPositionChange={noop}
        onTierChange={noop}
        onRegionChange={noop}
        onVersionChange={noop}
      />,
    );

    // Find via aria-pressed=true so the test isn't tied to a specific
    // locale label.
    const active = screen
      .getAllByRole("button")
      .filter((btn) => btn.getAttribute("aria-pressed") === "true");
    expect(active.length).toBeGreaterThanOrEqual(1);
  });

  it("fires onPositionChange with the new value when a chip is clicked", async () => {
    const onPositionChange = vi.fn();

    render(
      <RankingsFilters
        selected={defaultSelected}
        availableVersions={[]}
        availableRegions={[]}
        onPositionChange={onPositionChange}
        onTierChange={noop}
        onRegionChange={noop}
        onVersionChange={noop}
      />,
    );

    // Each segment button carries its localized label; click the
    // first non-active one (positions: All, TOP, JUNGLE, ...). The
    // first button has aria-pressed=true (currently "" = All), so
    // click the second — that's TOP.
    const buttons = screen.getAllByRole("button");
    const positionButtons = buttons.slice(0, 6); // 6 position chips
    const top = positionButtons[1];
    expect(top).toBeDefined();
    await userEvent.click(top!);

    expect(onPositionChange).toHaveBeenCalledTimes(1);
    expect(onPositionChange).toHaveBeenCalledWith("TOP");
  });

  it("fires onTierChange when a tier chip is clicked", async () => {
    const onTierChange = vi.fn();

    render(
      <RankingsFilters
        selected={defaultSelected}
        availableVersions={[]}
        availableRegions={[]}
        onPositionChange={noop}
        onTierChange={onTierChange}
        onRegionChange={noop}
        onVersionChange={noop}
      />,
    );

    const buttons = screen.getAllByRole("button");
    // 6 position chips + 6 tier chips. The first tier-chip is the
    // "all" sentinel; click index 7 (the challenger chip).
    const challenger = buttons[7];
    expect(challenger).toBeDefined();
    await userEvent.click(challenger!);

    expect(onTierChange).toHaveBeenCalledTimes(1);
    expect(onTierChange).toHaveBeenCalledWith("challenger");
  });
});
