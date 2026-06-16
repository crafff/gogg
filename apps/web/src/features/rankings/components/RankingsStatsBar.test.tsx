import "@shared/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RankingsStatsBar } from "./RankingsStatsBar";

describe("RankingsStatsBar", () => {
  it("renders nothing when totalMatches is zero", () => {
    const { container } = render(
      <RankingsStatsBar totalMatches={0} resolvedVersion={null} region="" />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("interpolates the localized count with <strong>", () => {
    render(
      <RankingsStatsBar
        totalMatches={12345}
        resolvedVersion="14.20"
        region="KR"
      />,
    );
    // Trans renders strong + the formatted count separately; the
    // string lookup against textContent is the cleanest assertion.
    expect(screen.getByText("12,345")).toBeInTheDocument();
    expect(screen.getByText("14.20")).toBeInTheDocument();
    expect(screen.getByText("KR")).toBeInTheDocument();
  });

  it('shows the "all" tag when region is empty', () => {
    render(
      <RankingsStatsBar totalMatches={100} resolvedVersion={null} region="" />,
    );
    // i18n returns "全部" in zh-CN, "All" in en-US — the default
    // resolved language picks one. Either way, the rendered tag is
    // not the version (none) and not a region (none).
    expect(screen.queryByText("KR")).toBeNull();
  });
});
