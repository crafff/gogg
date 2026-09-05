import "@shared/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LoginPage } from "./LoginPage";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("LoginPage", () => {
  it("shows a retry state instead of a login button when session lookup fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("network down")),
    );
    const client = new QueryClient({
      defaultOptions: { queries: { retryDelay: 0 } },
    });

    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/login"]}>
          <LoginPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /could not verify your session|无法确认登录状态/i,
    );
    expect(
      screen.queryByRole("link", { name: /Google/ }),
    ).not.toBeInTheDocument();
    expect(fetch).toHaveBeenCalledTimes(2);
  });
});
