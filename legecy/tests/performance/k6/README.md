# k6 API workloads

`api-baseline.js` implements the Stage 00 executable request contract. Formal
rankings runs use the mobile-drive dataset `rankings-kr-16.13-current-v1` and
explicit game version `16.13`; they never use `latest`.

Start the fixed database and observed API first:

```bash
make perf-env-up
```

Main scenarios:

- `rankings_graphql` (default)
- `rankings_rest`
- `versions`
- `regions`
- `graphql_versions`
- every `RKG-*` workload ID from Stage 00
- `contract_matrix` for the 60 business combinations / 120 transport requests

Examples:

```bash
make perf-warm PERF_SCENARIO=RKG-GQL-KR-MP-ALL
make perf-cold PERF_SCENARIO=RKG-GQL-KR-MP-ALL
make perf-warm PERF_SCENARIO=contract_matrix PERF_VUS=1
```

The runner refuses a formal run unless `gogg-perf-postgres` is healthy, contains
exactly 159,644 KR 16.13 matches, and `gogg-dev-api-observed` is connected to
the mobile-drive database on port 55434.
