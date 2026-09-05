import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LocalAssetImage } from "./LocalAssetImage";

describe("LocalAssetImage", () => {
  it("shows a stable fallback for a missing file and retries a changed source", () => {
    const view = render(
      <LocalAssetImage src="/missing.png" alt="Annie" fallback="A" />,
    );

    fireEvent.error(screen.getByRole("img", { name: "Annie" }));
    expect(screen.getByRole("img", { name: "Annie" })).toHaveTextContent("A");

    view.rerender(
      <LocalAssetImage src="/available.png" alt="Annie" fallback="A" />,
    );
    expect(screen.getByRole("img", { name: "Annie" })).toBeInstanceOf(
      HTMLImageElement,
    );
  });
});
