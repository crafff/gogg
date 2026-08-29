// React-refresh's "only export components" rule doesn't fit a router
// file — it has to export the `router` const plus a tiny fallback
// component side-by-side. Changing the route table requires a full
// reload anyway, so HMR boundaries here aren't load-bearing.
/* eslint-disable react-refresh/only-export-components */
import { lazy, Suspense } from "react";
import {
  createBrowserRouter,
  Navigate,
  type RouteObject,
} from "react-router-dom";

import { Skeleton } from "@shared/ui";
import { LoginPage, RequireSession } from "@features/auth";

import { Layout } from "./Layout";
import { RouteErrorBoundary } from "./RouteErrorBoundary";

// Route-level code splitting. Each feature page is loaded only when
// the route is hit — the rankings bundle stays the only synchronous
// payload because it's the landing route after the / → /rankings
// redirect.
const RankingsPage = lazy(() =>
  import("@features/rankings").then((m) => ({ default: m.RankingsPage })),
);
const ChampionDetailPage = lazy(() =>
  import("@features/champion-detail").then((m) => ({
    default: m.ChampionDetailPage,
  })),
);
const SummonerPage = lazy(() =>
  import("@features/summoner").then((m) => ({ default: m.SummonerPage })),
);
const TftAnalysisPage = lazy(() =>
  import("@features/tft").then((m) => ({ default: m.TftAnalysisPage })),
);
const TftPlayerPage = lazy(() =>
  import("@features/tft").then((m) => ({ default: m.TftPlayerPage })),
);
const MePage = lazy(() =>
  import("@features/user-profile").then((m) => ({ default: m.MePage })),
);

function PageFallback() {
  // Three-row skeleton roughly matching the visual density of every
  // feature page so the layout doesn't jump during the lazy fetch.
  return (
    <div className="space-y-3" data-testid="route-fallback">
      <Skeleton className="h-8 w-1/3" />
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-4 w-1/2" />
    </div>
  );
}

function lazyRoute(node: React.ReactNode): React.ReactNode {
  return <Suspense fallback={<PageFallback />}>{node}</Suspense>;
}

// Route definitions exported so unit tests can re-use them under
// createMemoryRouter without paying the BrowserRouter setup cost.
export const routes: RouteObject[] = [
  {
    element: <Layout />,
    errorElement: <RouteErrorBoundary />,
    children: [
      { index: true, element: <Navigate to="/rankings" replace /> },
      { path: "rankings", element: lazyRoute(<RankingsPage />) },
      {
        path: "champion/:championId",
        element: lazyRoute(<ChampionDetailPage />),
      },
      { path: "summoner", element: lazyRoute(<SummonerPage />) },
      {
        path: "summoner/:region/:gameName/:tagLine",
        element: lazyRoute(<SummonerPage />),
      },
      { path: "tft", element: lazyRoute(<TftAnalysisPage />) },
      { path: "tft/player", element: lazyRoute(<TftPlayerPage />) },
      {
        path: "tft/player/:platform/:gameName/:tagLine",
        element: lazyRoute(<TftPlayerPage />),
      },
      { path: "login", element: lazyRoute(<LoginPage />) },
      {
        path: "me",
        element: <RequireSession>{lazyRoute(<MePage />)}</RequireSession>,
      },
      { path: "*", element: <RouteErrorBoundary /> },
    ],
  },
];

export const router = createBrowserRouter(routes);
