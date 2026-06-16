import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Tag } from "./Tag";

describe("Tag", () => {
  it("renders text and applies the tier-coded tone class", () => {
    render(<Tag tone="challenger">Challenger</Tag>);
    const tag = screen.getByText("Challenger");
    expect(tag).toBeInTheDocument();
    expect(tag.className).toMatch(/text-tier-challenger/);
  });

  it("falls back to neutral tone with default sizing", () => {
    render(<Tag data-testid="t">14.20</Tag>);
    const tag = screen.getByTestId("t");
    expect(tag.className).toMatch(/text-fg-muted/);
    expect(tag.className).toMatch(/text-xs/);
  });
});
