CREATE TABLE IF NOT EXISTS game_versions (
    id         serial primary key,
    version    text not null unique,
    fetched_at timestamptz not null,
    is_latest  boolean not null default false
);

CREATE TABLE IF NOT EXISTS players (
    puuid      text primary key,
    game_name  text,
    tag_line   text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

CREATE TABLE IF NOT EXISTS runs (
    id            serial primary key,
    status        text not null default 'running',
    profile       text,
    mode          text not null,
    target_tiers  text[],
    current_phase int not null default 0,
    current_tier  text,
    started_at    timestamptz not null default now(),
    ended_at      timestamptz,
    last_run_end  timestamptz not null,
    created_at    timestamptz not null default now()
);

CREATE TABLE IF NOT EXISTS player_rank_snapshots (
    id             bigserial primary key,
    run_id         int references runs(id),
    puuid          text not null references players(puuid),
    source         text not null default 'phase1',
    league_id      text,
    queue          text not null,
    tier           text not null,
    division       text,
    league_points  int,
    wins           int,
    losses         int,
    veteran        boolean,
    inactive       boolean,
    fresh_blood    boolean,
    hot_streak     boolean,
    rank_status    text not null default 'active',
    created_at     timestamptz not null default now()
);

CREATE INDEX IF NOT EXISTS idx_rank_snapshots_puuid_created
    ON player_rank_snapshots(puuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_rank_snapshots_run
    ON player_rank_snapshots(run_id, source);

CREATE TABLE IF NOT EXISTS player_match_sync (
    puuid          text primary key references players(puuid),
    last_synced_at timestamptz not null
);

CREATE TABLE IF NOT EXISTS matches (
    match_id           text primary key,
    data_version       text,
    platform_id        text,
    queue_id           int,
    game_version       text,
    game_mode          text,
    game_type          text,
    game_start_ts      timestamptz,
    game_end_ts        timestamptz,
    game_duration      int,
    end_of_game_result text,
    fetch_status       text not null default 'pending',
    retry_count        int not null default 0,
    avg_tier_score     int,
    tier_coverage      smallint,
    created_at         timestamptz not null default now()
);

CREATE INDEX IF NOT EXISTS idx_matches_fetch_status ON matches(fetch_status);
CREATE INDEX IF NOT EXISTS idx_matches_game_version ON matches(game_version);

CREATE TABLE IF NOT EXISTS match_participants (
    id                                     bigserial primary key,
    match_id                               text not null references matches(match_id),
    puuid                                  text references players(puuid),
    participant_id                         int,
    tier_at_match                          text,
    division_at_match                      text,
    lp_at_match                            int,
    tier_snapshot_delta_h                  int,
    summoner_level                         int,
    team_id                                int,
    team_position                          text,
    individual_position                    text,
    lane                                   text,
    role                                   text,
    win                                    boolean,
    champion_id                            int,
    champion_name                          text,
    champ_level                            int,
    champ_experience                       int,
    champion_transform                     int,
    player_augment1                        int,
    player_augment2                        int,
    player_augment3                        int,
    player_augment4                        int,
    player_augment5                        int,
    player_augment6                        int,
    summoner1_id                           int,
    summoner2_id                           int,
    summoner1_casts                        int,
    summoner2_casts                        int,
    spell1_casts                           int,
    spell2_casts                           int,
    spell3_casts                           int,
    spell4_casts                           int,
    kills                                  int,
    deaths                                 int,
    assists                                int,
    double_kills                           int,
    triple_kills                           int,
    quadra_kills                           int,
    penta_kills                            int,
    unreal_kills                           int,
    killing_sprees                         int,
    largest_killing_spree                  int,
    largest_multi_kill                     int,
    first_blood_kill                       boolean,
    first_blood_assist                     boolean,
    longest_time_spent_living              int,
    total_time_spent_dead                  int,
    total_damage_dealt                     int,
    total_damage_dealt_to_champions        int,
    physical_damage_dealt                  int,
    physical_damage_dealt_to_champions     int,
    magic_damage_dealt                     int,
    magic_damage_dealt_to_champions        int,
    true_damage_dealt                      int,
    true_damage_dealt_to_champions         int,
    largest_critical_strike                int,
    total_damage_taken                     int,
    physical_damage_taken                  int,
    magic_damage_taken                     int,
    true_damage_taken                      int,
    damage_self_mitigated                  int,
    total_heal                             int,
    total_heals_on_teammates               int,
    total_units_healed                     int,
    total_damage_shielded_on_teammates     int,
    time_ccing_others                      int,
    total_time_cc_dealt                    int,
    gold_earned                            int,
    gold_spent                             int,
    item0                                  int,
    item1                                  int,
    item2                                  int,
    item3                                  int,
    item4                                  int,
    item5                                  int,
    item6                                  int,
    items_purchased                        int,
    consumables_purchased                  int,
    role_bound_item                        int,
    total_minions_killed                   int,
    neutral_minions_killed                 int,
    total_ally_jungle_minions              int,
    total_enemy_jungle_minions             int,
    baron_kills                            int,
    dragon_kills                           int,
    objectives_stolen                      int,
    objectives_stolen_assists              int,
    vision_score                           int,
    vision_wards_bought                    int,
    sight_wards_bought                     int,
    wards_placed                           int,
    detector_wards_placed                  int,
    wards_killed                           int,
    turret_kills                           int,
    turret_takedowns                       int,
    turrets_lost                           int,
    first_tower_kill                       boolean,
    first_tower_assist                     boolean,
    inhibitor_kills                        int,
    inhibitor_takedowns                    int,
    inhibitors_lost                        int,
    nexus_kills                            int,
    nexus_lost                             int,
    nexus_takedowns                        int,
    damage_dealt_to_objectives             int,
    damage_dealt_to_buildings              int,
    damage_dealt_to_turrets                int,
    damage_dealt_to_epic_monsters          int,
    all_in_pings                           int,
    basic_pings                            int,
    assist_me_pings                        int,
    command_pings                          int,
    danger_pings                           int,
    enemy_missing_pings                    int,
    get_back_pings                         int,
    hold_pings                             int,
    on_my_way_pings                        int,
    need_vision_pings                      int,
    push_pings                             int,
    retreat_pings                          int,
    enemy_vision_pings                     int,
    vision_cleared_pings                   int,
    game_ended_in_early_surrender          boolean,
    game_ended_in_surrender                boolean,
    team_early_surrendered                 boolean,
    time_played                            int,
    created_at                             timestamptz not null default now()
);

CREATE INDEX IF NOT EXISTS idx_participants_match   ON match_participants(match_id);
CREATE INDEX IF NOT EXISTS idx_participants_puuid   ON match_participants(puuid);
CREATE INDEX IF NOT EXISTS idx_participants_champion ON match_participants(champion_id);
CREATE INDEX IF NOT EXISTS idx_participants_tier    ON match_participants(tier_at_match);

CREATE TABLE IF NOT EXISTS match_perks (
    match_id      text not null references matches(match_id),
    puuid         text not null references players(puuid),
    stat_defense  int,
    stat_flex     int,
    stat_offense  int,
    style0        int,
    style1        int,
    perk0         int,
    perk1         int,
    perk2         int,
    perk3         int,
    perk4         int,
    perk5         int,
    var01 int, var02 int,
    var11 int, var12 int, var13 int,
    var21 int, var22 int, var23 int,
    var31 int, var32 int, var33 int,
    var41 int, var42 int, var43 int,
    var51 int, var52 int, var53 int,
    primary key (match_id, puuid)
);

CREATE TABLE IF NOT EXISTS match_bans (
    match_id    text not null references matches(match_id),
    team_id     int  not null,
    pick_turn   int  not null,
    champion_id int,
    primary key (match_id, team_id, pick_turn)
);

CREATE INDEX IF NOT EXISTS idx_bans_champion ON match_bans(champion_id);

CREATE TABLE IF NOT EXISTS match_teams (
    match_id          text not null references matches(match_id),
    team_id           int  not null,
    win               boolean,
    baron_kills       int,
    dragon_kills      int,
    tower_kills       int,
    inhibitor_kills   int,
    rift_herald_kills int,
    feats             jsonb,
    primary key (match_id, team_id)
);

CREATE TABLE IF NOT EXISTS match_timelines (
    match_id       text not null references matches(match_id),
    puuid          text not null references players(puuid),
    boots_item_id  int,
    first_item_id  int,
    second_item_id int,
    third_item_id  int,
    fourth_item_id int,
    raw_events     jsonb,
    primary key (match_id, puuid)
);
