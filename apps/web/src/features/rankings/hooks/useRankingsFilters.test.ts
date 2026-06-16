import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { mapToTierGroup, useRankingsFilters } from "./useRankingsFilters";

describe("useRankingsFilters", () => {
  it("seeds selected + committed from defaults", () => {
    const { result } = renderHook(() => useRankingsFilters());
    expect(result.current.selected).toEqual({
      position: "",
      tier: "",
      region: "",
      version: "latest",
    });
    expect(result.current.committed).toEqual(result.current.selected);
  });

  it("decouples selected from committed until commit()", () => {
    const { result } = renderHook(() => useRankingsFilters());

    act(() => {
      result.current.setPosition("TOP");
    });
    expect(result.current.selected.position).toBe("TOP");
    expect(result.current.committed.position).toBe("");

    act(() => {
      result.current.commit();
    });
    expect(result.current.committed.position).toBe("TOP");
  });

  it("fires onBeforeCommit exactly once per real change", () => {
    const onBefore = vi.fn();
    const { result } = renderHook(() =>
      useRankingsFilters({ onBeforeCommit: onBefore }),
    );

    act(() => {
      result.current.setPosition("TOP");
    });
    expect(onBefore).toHaveBeenCalledTimes(1);

    // Setting the same value again is a no-op.
    act(() => {
      result.current.setPosition("TOP");
    });
    expect(onBefore).toHaveBeenCalledTimes(1);

    act(() => {
      result.current.setTier("challenger");
    });
    expect(onBefore).toHaveBeenCalledTimes(2);
  });
});

describe("mapToTierGroup", () => {
  it("maps the UI tier vocabulary to the GraphQL TierGroup", () => {
    expect(mapToTierGroup("")).toBe("ALL");
    expect(mapToTierGroup("challenger")).toBe("CHALLENGER");
    expect(mapToTierGroup("grandmaster_plus")).toBe("GRANDMASTER_PLUS");
    expect(mapToTierGroup("grandmaster")).toBe("GRANDMASTER");
    expect(mapToTierGroup("master_plus")).toBe("MASTER_PLUS");
    expect(mapToTierGroup("master")).toBe("MASTER");
  });
});
