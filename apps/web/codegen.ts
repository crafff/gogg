import type { CodegenConfig } from "@graphql-codegen/cli";

// graphql-codegen reads the gqlgen schema files directly from the
// apps/api source tree. The schema is the contract — keeping a local
// copy in apps/web would mean two sources of truth. CI fails if the
// schema moves and codegen hasn't been re-run.
//
// Operations live alongside the runtime fetcher under
// shared/api/operations/*.graphql so a feature's `useChampionRankings`
// hook and the GraphQL document that powers it stay in the same git
// blame line.
//
// Output is one self-contained module per generated artifact:
//   - types.ts  → enums, input types, operation results
//   - hooks.ts  → useQuery / useMutation hooks with embedded fetcher
//
// Generated files are ignored by ESLint (eslint.config.js) and by
// prettier (via .prettierignore once chunk 5 adds one).
const config: CodegenConfig = {
  schema: "../api/internal/transport/graphql/schema/*.graphql",
  documents: ["src/**/*.graphql", "src/**/*.gql"],
  ignoreNoDocuments: false,
  generates: {
    "src/shared/api/generated/types.ts": {
      // typescript-operations 6.x silently re-emits any schema input
      // or enum referenced by an operation under its OWN config, which
      // causes TS2300 duplicates when paired with the `typescript`
      // plugin in the same file. We use typescript-operations alone —
      // it emits both the operation shapes AND the input/enum types
      // they reference (TierGroup, ChampionRankingsFilter), giving us
      // a single non-duplicated source of truth.
      plugins: ["typescript-operations"],
      config: {
        skipTypename: false,
        enumsAsTypes: true,
        useTypeImports: true,
        avoidOptionals: { field: true, inputValue: false, object: false },
        scalars: {
          // GraphQL Int is 32-bit by spec, but the backend uses Go int
          // (8 bytes on amd64) for counts like `games` / `totalMatches`
          // that comfortably exceed 2^31 in aggregate. Map to number;
          // JSON numbers are doubles and round-trip exactly up to 2^53.
          Int: "number",
          Float: "number",
        },
      },
    },
    "src/shared/api/generated/hooks.ts": {
      preset: undefined,
      plugins: [
        // The `add` plugin injects an import for the sibling types
        // module so `Operations.VersionsQuery` etc. resolve; the
        // typescript-react-query plugin only *references* the
        // `Operations` namespace, it doesn't write the import itself.
        {
          add: {
            content: 'import type * as Operations from "./types";',
          },
        },
        "typescript-react-query",
      ],
      config: {
        importOperationTypesFrom: "Operations",
        addInfiniteQuery: false,
        fetcher: {
          // Endpoint is wrapped in an extra pair of quotes because the
          // plugin emits the value as a raw JS expression. Without the
          // outer quoting the output reads `fetch(/graphql as string,
          // …)` and won't compile.
          endpoint: '"/graphql"',
          fetchParams: {
            headers: { "Content-Type": "application/json" },
            credentials: "same-origin",
          },
        },
        exposeQueryKeys: true,
        exposeFetcher: true,
        reactQueryVersion: 5,
        scalars: {
          Int: "number",
          Float: "number",
        },
      },
    },
  },
  // No afterAllFileWrite hook — `prettier` isn't on PATH in every
  // dev shell. Generated files are excluded from lint/prettier checks
  // anyway, and the plugin's own formatting is consistent enough.
};

export default config;
