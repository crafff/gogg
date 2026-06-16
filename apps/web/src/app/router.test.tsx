import "@shared/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { describe, expect, it } from "vitest";

import { routes } from "./router";

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe("router", () => {
  it("redirects / to /rankings", async () => {
    renderAt("/");
    // Rankings page is lazy-loaded; wait for the heading to land.
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(
      /Champion rankings|英雄排行/,
    );
  });

  it("renders the champion-detail placeholder with the URL param", async () => {
    renderAt("/champion/99");
    const placeholder = await screen.findByTestId("placeholder-page");
    expect(placeholder).toBeInTheDocument();
    expect(placeholder).toHaveTextContent("championId = 99");
  });

  it("renders the summoner placeholder with region + name", async () => {
    renderAt("/summoner/kr/Faker");
    const placeholder = await screen.findByTestId("placeholder-page");
    expect(placeholder).toHaveTextContent("KR / Faker");
  });

  it("renders the login placeholder", async () => {
    renderAt("/login");
    expect(await screen.findByTestId("placeholder-page")).toBeInTheDocument();
  });

  it("renders the me placeholder", async () => {
    renderAt("/me");
    expect(await screen.findByTestId("placeholder-page")).toBeInTheDocument();
  });

  it("renders the route error boundary for unknown paths", async () => {
    renderAt("/this/route/does/not/exist");
    expect(await screen.findByTestId("route-error")).toBeInTheDocument();
  });
});
