import { QueryClient } from "@tanstack/react-query";

// Single shared QueryClient instance for the whole app. Tuning notes:
//
//   staleTime: 60s — catalog (versions/regions) barely changes, but
//     rankings can shift between patches; 60s avoids storms when users
//     swap filters rapidly without staleness from a recent crawler run.
//   gcTime:   1h  — keep cache around so back/forward feels instant.
//   retry:    2   — Riot 5xx / network blips; fail fast otherwise.
//
// Window-focus refetch is OFF because the rankings page is a "browse"
// surface, not a dashboard — refetching every tab switch would burn
// the API for no UX gain.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 60_000,
      gcTime: 60 * 60 * 1000,
      retry: 2,
      refetchOnWindowFocus: false,
    },
  },
});
