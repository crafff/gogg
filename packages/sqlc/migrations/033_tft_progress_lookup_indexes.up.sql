CREATE INDEX tft_match_jobs_active_route_status
    ON tft_match_jobs (routing_region, status)
    WHERE status IN ('pending', 'retry', 'leased');

CREATE INDEX tft_crawl_runs_schedule_created
    ON tft_crawl_runs (schedule_id, created_at DESC, id DESC);
