ALTER TABLE summoner_lookup_jobs
    DROP CONSTRAINT IF EXISTS summoner_lookup_jobs_stage_check;

ALTER TABLE summoner_lookup_jobs
    ADD CONSTRAINT summoner_lookup_jobs_stage_check
    CHECK (stage IN (
        'QUEUED',
        'RESOLVE_ACCOUNT',
        'REFRESH_PROFILE',
        'FETCH_MATCHES',
        'ENRICH_RANKS',
        'FINALIZE'
    ));
