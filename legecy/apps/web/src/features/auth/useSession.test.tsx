import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useMeQuery, type MeQuery } from "@shared/api";

import { useLogout } from "./useSession";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useLogout", () => {
  it("revokes the server session and clears the Me cache", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    const me: MeQuery = {
      me: {
        id: "018f06a5-19c8-7a4c-b277-4b93e4888dd8",
        displayName: "Player",
        email: "player@example.com",
        avatarUrl: null,
        locale: "zh-CN",
        identities: [{ provider: "google", username: "Player" }],
      },
    };
    client.setQueryData(useMeQuery.getKey(), me);

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useLogout(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(fetchMock).toHaveBeenCalledWith("/auth/logout", {
      method: "POST",
      credentials: "same-origin",
      headers: { "X-GOGG-CSRF": "1" },
    });
    await waitFor(() =>
      expect(client.getQueryData(useMeQuery.getKey())).toEqual({ me: null }),
    );
  });
});
