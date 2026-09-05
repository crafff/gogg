import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Button } from "./Button";

describe("Button", () => {
  it("renders a button with the label and forwards click events", async () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Sign in</Button>);

    const btn = screen.getByRole("button", { name: "Sign in" });
    expect(btn).toBeInTheDocument();
    expect(btn).toHaveAttribute("type", "button");

    await userEvent.click(btn);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("composes the secondary variant with caller className via tailwind-merge", () => {
    render(
      <Button variant="secondary" className="p-12">
        Cancel
      </Button>,
    );
    const btn = screen.getByRole("button", { name: "Cancel" });
    // tailwind-merge keeps p-12 and drops the default px-3.5 from md.
    expect(btn.className).toContain("p-12");
    expect(btn.className).not.toMatch(/\bpx-3\.5\b/);
  });
});
