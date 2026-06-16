import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./Select";

describe("Select", () => {
  it("renders the trigger with the placeholder and accent border", () => {
    render(
      <Select>
        <SelectTrigger className="w-40" aria-label="Region">
          <SelectValue placeholder="Pick a region" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="kr">KR</SelectItem>
          <SelectItem value="na1">NA1</SelectItem>
        </SelectContent>
      </Select>,
    );

    const trigger = screen.getByRole("combobox", { name: "Region" });
    expect(trigger).toBeInTheDocument();
    expect(trigger.className).toMatch(/border-border-default/);
    expect(screen.getByText("Pick a region")).toBeInTheDocument();
  });
});
