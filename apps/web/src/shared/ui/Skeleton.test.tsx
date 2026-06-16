import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Skeleton } from "./Skeleton";

describe("Skeleton", () => {
  it("renders with status role and the pulse animation class", () => {
    render(<Skeleton className="h-4 w-24" data-testid="sk" />);
    const sk = screen.getByTestId("sk");
    expect(sk).toBeInTheDocument();
    expect(sk).toHaveAttribute("role", "status");
    expect(sk.className).toMatch(/animate-skeleton/);
    expect(sk.className).toMatch(/h-4/);
    expect(sk.className).toMatch(/w-24/);
  });
});
