# Champion win factors

Champion win factors are a revisioned, read-optimized statistics surface for
early-game metrics associated with match outcomes. They are deliberately
separate from champion build choices and from future personalized advice.

## First publication scope

The first production schema publishes four descriptive JUNGLE metrics:

- jungle CS at 10 minutes;
- jungle CS gained from 10 to 15 minutes;
- champion damage at 10 minutes;
- champion damage gained from 10 to 15 minutes.

Every result is an observational association. It is not a causal estimate and
the displayed win-rate deltas cannot be added together. Model-adjusted and
causal evidence require a later algorithm version and separate publication
fields.

## Cohort contract

The publication builder starts from `statistics_match_membership` with
`dataset_key='ranked-solo-v1'`. On-demand summoner matches therefore cannot
silently alter the global baseline. Eligible matches must be ranked solo,
complete, at least 15 minutes long, have a valid region/version/average tier,
contain exactly two complete five-position teams, and the selected jungler must
have valid 10/15-minute snapshots.

One team is chosen deterministically from each match with MD5 and only that
team's jungler becomes an observation. This preserves the historical
experiment's protection against treating paired opponents as independent
samples.

The builder materializes the exact request cohorts supported by the UI:

- region: `ALL`, `KR`, `NA1`;
- tier: `ALL`, `MASTER`, `MASTER_PLUS`, `GRANDMASTER`,
  `GRANDMASTER_PLUS`, `CHALLENGER`;
- normalized `matches.version`;
- champion and `JUNGLE` position.

The initial release gates are 500 games and 50 distinct players per cohort.
Every displayed metric must retain at least five non-empty percentile buckets,
and every bucket must contain at least 50 games from at least 10 distinct
players. If any of the four metrics fails these gates, the whole cohort remains
queryable with an explicit unavailable reason and none of its factors are
exposed.

## Publication lifecycle

Migrations `034_champion_win_factors` and
`035_champion_insight_observational_contract` define immutable publications,
cohorts, factors, and buckets. Builders serialize before opening a
repeatable-read snapshot, then derive source coverage and observations from
that snapshot. A rebuild creates a `building` publication, validates complete
four-metric coverage and bucket reconciliation, supersedes the previous
publication, and marks the new revision `published` in one transaction. An
empty or gated-out build fails before publication and leaves the previous
revision readable.

Run a rebuild independently from region crawl workflows:

```bash
make refresh-champion-insights args='--timeout 2h'
```

The rebuild is a PostgreSQL-only scan and does not consume Riot API limits.
The revision includes the algorithm gates plus a deterministic fingerprint of
all observation dimensions and values. Identical input is reused rather than
duplicated; a corrected snapshot or cohort membership produces a new revision
even when its row count and latest timestamp are unchanged.

## Serving path

The API reads only the published tables through the `championinsights` service
and the lazy GraphQL query `championWinFactors`. `version="latest"` resolves to
the newest insight version containing at least one available cohort; it does
not silently fall back when the user explicitly requests an unpublished patch.
In the champion-detail UI, both overview and factors first resolve the shared
rankings version catalog's newest concrete patch, so switching views cannot
silently compare different patches; factors show `NOT_PUBLISHED` if that patch
has not passed its own release gates.

The champion detail page exposes `?view=factors`. It requires an explicit
position, preserves all filters in the URL, and does not run the build-choice
query while the factors view is active. Every bucket includes its observed win
rate, delta from the cohort mean, match count, and distinct-player count.
Match-level confidence intervals are intentionally not published because
repeat matches from the same player violate the independent-trial assumption;
future intervals must use player-cluster-aware estimation.

## Deferred work

The current publication does not provide personalized advice, online model
inference, objective/kill/turret event metrics, or validated estimates for the
other four positions. Those require role-specific validation and, for personal
reports, a durable timeline-enrichment path with explicit coverage handling.
