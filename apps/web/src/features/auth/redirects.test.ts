import { describe, expect, it } from "vitest";

import { safeReturnTo } from "./redirects";

describe("safeReturnTo", () => {
  it.each([
    [
      "/summoner/NA1/Player/Tag?tab=ranked#details",
      "/summoner/NA1/Player/Tag?tab=ranked#details",
    ],
    ["https://evil.example/steal", "/me"],
    ["//evil.example/steal", "/me"],
    ["/\\evil.example/steal", "/me"],
    [null, "/me"],
  ])("maps %s to %s", (raw, want) => {
    expect(safeReturnTo(raw)).toBe(want);
  });
});
