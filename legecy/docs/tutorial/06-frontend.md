# Chapter 06 · Frontend (`apps/web`)

> Goal: by the end of this chapter you understand how vite serves the dev page, how the React Router defines routes, how `graphql-codegen` turns a `.graphql` file into a React hook, how TanStack Query manages the cache, and how the rankings page is split into hooks + presenter components.

This chapter assumes the dev stack from Chapter 02 + the API binary from Chapter 05 are running. Some exercises also need the worker (Chapter 04) to have ingested data, but we'll note when.

## What's in `apps/web`

```bash
ls apps/web/src/
```

You'll see:

```
app/        ← the App shell: providers, layout, router
features/   ← one folder per domain feature (rankings, champion-detail, summoner, auth, user-profile)
shared/     ← cross-cutting code: design tokens, i18n, ui components, api client
test/       ← vitest setup file
main.tsx    ← React mount point
index.css   ← tailwind directives
```

Imports use path aliases:

- `@app/*` → `src/app/*`
- `@features/*` → `src/features/*`
- `@shared/*` → `src/shared/*`
- `@/*` → `src/*`

The aliases are declared once in `vite.config.ts` (for runtime) and `tsconfig.app.json` (for tsc). When you see `import { useChampionRankingsQuery } from "@shared/api"`, that's the codegen'd hooks barrel.

## How a page renders, end-to-end

```bash
cat apps/web/src/main.tsx
```

That mounts `<App />` into `#root`. Then:

```bash
cat apps/web/src/app/App.tsx
```

It's small:

```tsx
return (
  <QueryClientProvider client={queryClient}>
    <RouterProvider router={router} />
  </QueryClientProvider>
);
```

Two providers wrap the entire tree:

1. **QueryClientProvider** — TanStack Query's cache, configured in `src/shared/api/queryClient.ts`. Tuning: 60-second staleTime, 1-hour gcTime, no refetch-on-window-focus.
2. **RouterProvider** — React Router's `createBrowserRouter` instance, defined in `src/app/router.tsx`.

```bash
cat apps/web/src/app/router.tsx
```

Routes are an array. The shell route is `<Layout />` (header + outlet), and inside it:

```
/                       → redirect to /rankings
/rankings               → RankingsPage (lazy)
/champion/:championId   → ChampionDetailPage (lazy)
/summoner               → SummonerPage
/summoner/:region/:name → SummonerPage
/login                  → LoginPage
/me                     → MePage
*                       → RouteErrorBoundary  (404)
```

Each non-shell route uses `React.lazy()` so its bundle only downloads when navigated to. The vite build splits each feature into its own chunk — that's why the rankings JS is 17 KB gzipped and each placeholder is under 1 KB.

🛠️ **Exercise**: open <http://localhost:5173/login> in a fresh tab. Open DevTools Network → JS filter. You'll see `auth-<hash>.js` load — that's the lazy chunk for `LoginPage`. Navigate to `/me` and watch `user-profile-<hash>.js` arrive.

### The layout

```bash
cat apps/web/src/app/Layout.tsx
```

Header has:

- A clickable brand block (links to `/rankings`)
- Nav links (`NavLink` from react-router-dom — adds an `aria-current` attribute when matching)
- LanguageSwitcher (from `@shared/i18n`)
- A login CTA button

Body has `<Outlet />` — that's where the routed page renders.

### i18n setup

```bash
cat apps/web/src/shared/i18n/index.ts
```

`i18next` boots with:

- Two languages: `zh-CN` (default) + `en-US`
- Two namespaces: `common` + `rankings`
- Browser LanguageDetector (`localStorage.gogg.lang` → `navigator.language` → HTML lang)

The bundles are statically imported in `resources.ts`, so they're part of the main chunk. Chapter 4 of the original plan calls out switching to `i18next-http-backend` lazy loading once we have more namespaces.

🛠️ **Exercise**: open `apps/web/src/shared/i18n/locales/zh-CN/common.json`. Change `"brand": "GOGG"` to `"brand": "GOGG-NEW"`. Save. Vite HMR reloads — you should see the header change instantly. Revert.

## The codegen pipeline (where TypeScript types come from)

The backend GraphQL schema is the contract. Instead of manually keeping TypeScript types in sync, `graphql-codegen` reads the schema files in `apps/api/internal/transport/graphql/schema/` and emits:

- `apps/web/src/shared/api/generated/types.ts` — TypeScript types for every input + every operation result.
- `apps/web/src/shared/api/generated/hooks.ts` — `useVersionsQuery`, `useRegionsQuery`, `useChampionRankingsQuery` React hooks.

```bash
cat apps/web/codegen.ts
```

Schema source: relative path to the apps/api tree. No copy of the schema in the web project — the backend file is the source of truth, full stop. CI fails the build if codegen output is out of date.

Run it any time:

```bash
cd apps/web
npm run codegen
```

If you changed the GraphQL schema in apps/api, this is the command that regenerates the frontend types. Commit the regenerated files.

🛠️ **Exercise**: open `apps/api/internal/transport/graphql/schema/catalog.graphql`. Add a new field to the schema:

```graphql
extend type Query {
  versions: [String!]!
  regions: [String!]!
  greeting(name: String): String!     # NEW
}
```

Run `make gen-gql` (regenerates the Go gqlgen layer) and `npm run codegen` from `apps/web`. The generated `useGreetingQuery` would appear in the hooks file. (You'd need to implement the resolver in Go for it to actually work, but this proves the codegen wiring.)

Revert when done — the backend won't compile until you implement the resolver.

## The operations (your queries)

Operations are `.graphql` files on the frontend side:

```bash
cat apps/web/src/shared/api/operations/catalog.graphql
cat apps/web/src/shared/api/operations/rankings.graphql
```

`catalog.graphql` declares the two simple queries (`Versions`, `Regions`). `rankings.graphql` declares `ChampionRankings($filter)`. When codegen runs:

1. Validates the operations against the schema.
2. Generates a typed `useChampionRankingsQuery({ filter })` hook.
3. The hook posts to `/graphql` via native `fetch` (no graphql-request dep), parses the JSON, returns it as TanStack Query's `data`.

The generated fetcher is in `hooks.ts`:

```ts
async function fetcher<TData, TVariables>(query, variables) {
    const res = await fetch("/graphql", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ query, variables }),
    });
    // ...
}
```

Relative `/graphql` URL → in dev, vite proxies it to `:8080`; in prod, nginx serves both the static SPA and proxies `/graphql` to `gogg-api`.

## The rankings page deep dive

The page is the only "real" feature so far. It's intentionally split:

```bash
ls apps/web/src/features/rankings/
```

```
RankingsPage.tsx       ← thin orchestrator (~150 lines)
index.ts               ← barrel
components/
  RankingsFilters.tsx     ← position chips + tier chips + region/version dropdowns
  RankingsStatsBar.tsx    ← "Based on X matches" header
  RankingsTable.tsx       ← the dense table
hooks/
  useRankingsFilters.ts   ← selected vs committed state
  useRankingsQuery.ts     ← wraps useChampionRankingsQuery with filter mapping
  useFadeTransition.ts    ← 4-stage fade animation state machine
  useInfiniteScroll.ts    ← IntersectionObserver wrapper
```

### Read the orchestrator

```bash
head -80 apps/web/src/features/rankings/RankingsPage.tsx
```

The page wires the three hooks together. Key pattern:

```ts
const fade = useFadeTransition();
const filters = useRankingsFilters({
  onBeforeCommit: () => {
    fade.beginExit();
    setLimit(PAGE_SIZE);
  },
});

// On phase change, commit the new filter after the fade completes.
useEffect(() => {
  if (fade.phase === "hidden") filters.commit();
}, [fade.phase]);

const rankings = useRankingsQuery({
  filters: filters.committed,
  limit,
  enabled: fade.phase === "shown" || fade.phase === "fading-in",
});

// When data lands, trigger the fade-in.
useEffect(() => {
  if (fade.phase !== "hidden") return;
  if (rankings.isLoading || rankings.isFetching) return;
  fade.beginEnter();
}, [fade, rankings.isLoading, rankings.isFetching]);
```

Why this dance? When the user changes a filter:

1. `onBeforeCommit` runs → `fade.beginExit()` → the table starts fading out.
2. After the fade-out timer, `fade.phase` flips to `"hidden"`.
3. The effect on `fade.phase` calls `filters.commit()` → `committed` updates → `useRankingsQuery` refires.
4. While loading, the table stays invisible (phase is `hidden`).
5. When the query resolves, the second effect fires `fade.beginEnter()` → fade back in.
6. After fade-in, phase is `"shown"` and infinite scroll re-enables.

The result is a smooth fade between filter swaps instead of a jarring instant swap.

### Read one hook

```bash
cat apps/web/src/features/rankings/hooks/useFadeTransition.ts
```

It owns:

- 4-phase state machine (`shown → fading-out → hidden → fading-in → shown`)
- Viewport ref for height locking (prevents page reflow during the swap)
- Two timer refs (for safe cleanup)
- `beginExit()` / `beginEnter()` methods

Pure UI state. Doesn't know about GraphQL or filters.

```bash
cat apps/web/src/features/rankings/hooks/useRankingsQuery.ts
```

This one wraps the codegen'd query:

```ts
const filter: ChampionRankingsFilter = {
  position: filters.position || "",
  tierGroup: mapToTierGroup(filters.tier),
  region: filters.region,
  version: filters.version,
  minGames: FIXED_MIN_GAMES,
  limit,
};

const query = useChampionRankingsQuery(
  { filter },
  { enabled, placeholderData: keepPreviousData },
);

return {
  items: query.data?.championRankings.items ?? [],
  totalMatches: query.data?.championRankings.totalMatches ?? 0,
  // ...
};
```

`keepPreviousData` (TanStack Query option) is why infinite scroll doesn't blank the table during the next-page fetch — the old data stays visible.

The `mapToTierGroup` function translates the UI vocabulary (`""`, `"challenger"`, `"grandmaster_plus"`, ...) into the GraphQL enum (`"ALL"`, `"CHALLENGER"`, `"GRANDMASTER_PLUS"`, ...). This is the only place the two vocabularies meet.

### Try this — see the fetch happen

🛠️ **Exercise**: with `make run-api` and `make run-web` running, open <http://localhost:5173>. Open DevTools → Network → filter by `graphql`. Click a position chip. You'll see:

- The fade animation starts (table opacity 0)
- A new POST `/graphql` request fires
- The request body has `"variables": {"filter": {"position": "TOP", ...}}`
- When the response arrives, the table fades in with the new data

If the table is empty (no crawled data), only the request goes through; the table re-renders with "No data."

## Components

```bash
ls apps/web/src/shared/ui/
```

Base components, all using `class-variance-authority` (cva) for variant matrices:

- `Button` — 4 variants × 3 sizes
- `Tag` — 5 tones × 2 sizes
- `Skeleton` — animated placeholder
- `Select` — wraps Radix Primitives (accessible dropdown)
- `PlaceholderPage` — used by chunk-5 placeholder routes

Each component lives in `<Name>.tsx`. The cva variants live in `<Name>.variants.ts` (separate file so react-refresh's "only export components" rule stays clean — variants are functions, not components).

🛠️ **Exercise**: edit `apps/web/src/shared/ui/Button.variants.ts`. Change the `primary` variant's background to `bg-red-500`. Save. Every Button using `variant="primary"` (e.g. the "Sign in" CTA in the header) should turn red. Revert.

## Design tokens

```bash
cat apps/web/tailwind.config.ts | head -60
```

Two layers:

1. **Brand primitives** (`gogg-gold`, `gogg-ink`) — direct color hex.
2. **Semantic tokens** (`surface-*`, `fg-*`, `border-*`, `accent-*`) — what components actually speak. These map onto the brand for now but allow a light theme or a public/anonymous variant to be skinned in without touching components.

Plus a `tier-*` palette (challenger/grandmaster/master/...) for tier-specific accents.

In Tailwind classes you'll see `bg-surface-raised text-fg-muted border-border-default`. Search the codebase for any of those — every component speaks semantic tokens.

## i18n + the Trans component

The simple case: `t("filter.region")` returns the localized string for that key. The interesting case: interpolating with markup inside a sentence:

```bash
cat apps/web/src/features/rankings/components/RankingsStatsBar.tsx
```

```tsx
<Trans
  ns="rankings"
  i18nKey="subtitle"
  values={{ count: totalMatches.toLocaleString() }}
  components={{ strong: <strong className="text-fg-default" /> }}
/>
```

The locale file says `"subtitle": "基于 <strong>{{count}}</strong> 场比赛数据"`. The `<Trans>` component parses the placeholder syntax and substitutes the React element. This way translators write natural-feeling sentences with embedded emphasis, without us hand-stitching markup in code.

## Tests

```bash
cd apps/web
npm test
```

39 cases across 13 files. Categories:

- Component smoke (`shared/ui/*.test.tsx` + `features/rankings/components/*.test.tsx`)
- Hook unit (`features/rankings/hooks/*.test.{ts,tsx}`)
- Router integration (`app/router.test.tsx` — uses `createMemoryRouter` to walk routes)
- API smoke (`shared/api/api.test.ts` — asserts the generated hooks/queryKeys exist with the right shapes)

```bash
npm run test:e2e
```

One Playwright golden path: visit `/`, click TOP chip, scroll to load more. All network mocked via `page.route("**/graphql")`. See Chapter 02 for the system-deps note.

🛠️ **Exercise**: run `npm test -- --watch`. Edit `RankingsTable.tsx` — change `54.4%` to render as `54.40%` (add the `.toFixed(2)` etc). The `RankingsTable.test.tsx` should fail. Revert + re-run.

## Up next

[Chapter 07 — End-to-end trace](./07-end-to-end.md) connects everything: we'll follow a single `winRate` number from Riot's API into a row in the browser, watching it pass through every layer you've now seen.
