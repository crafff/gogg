运行测试（需要本地 DB + Riot API key）：
  RIOT_API_KEY=<your_key> go test -tags integration -v -timeout 10m \
    ./internal/crawler/ -run TestFullPipelineReduced

  输出示例：
  ── Phase 0: Version Sync
  game version: 15.1.1
  ── Phase 1: Rank Sync
  fetched 312 players → keeping 3
  ── Phase 2: Match ID Collection
  fetched 847 pending matches → keeping 3
  ── Phase 3: Match Detail Fetch
  match details: done=3  error=0  participants=30
  ── Phase 3.5: On-Demand Rank
  tier backfill: filled=28  missing=2
  ── Phase 3: Match Detail Fetch
  match details: done=3  error=0  participants=30
  ── Phase 3.5: On-Demand Rank
  tier backfill: filled=28  missing=2
  ── Phase 4: Avg Tier Calc
  scored matches: 3
  ── Summary (schema="crawl_inttest"  run_id=1) ──
    game_versions                3 rows
    players                      33 rows
    ...
  Cleanup: go run ./cmd/inttest-cleanup/

  数据检查完之后清理：
  go run ./cmd/inttest-cleanup/