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
const LoginPage = lazy(() =>
  import("@features/auth").then((m) => ({ default: m.LoginPage })),
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
      // The /summoner index is the search landing; a real route hits
      // it with both segments. Phase E will turn /summoner into the
      // search form and keep /summoner/:region/:name for results.
      { path: "summoner", element: lazyRoute(<SummonerPage />) },
      {
        path: "summoner/:region/:name",
        element: lazyRoute(<SummonerPage />),
      },
      { path: "login", element: lazyRoute(<LoginPage />) },
      { path: "me", element: lazyRoute(<MePage />) },
      { path: "*", element: <RouteErrorBoundary /> },
    ],
  },
];

export const router = createBrowserRouter(routes);
