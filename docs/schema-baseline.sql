--
-- PostgreSQL database dump
--

\restrict A2NpbHaVT8cqZSbGxZ5tYQ55glKycQRhYcMfFIaHVymEfTBnsNSrFpt3DmOeJT7

-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: game_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.game_versions (
    id integer NOT NULL,
    version text NOT NULL,
    fetched_at timestamp with time zone NOT NULL,
    is_latest boolean DEFAULT false NOT NULL,
    patch_start_at timestamp with time zone
);


--
-- Name: game_versions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.game_versions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: game_versions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.game_versions_id_seq OWNED BY public.game_versions.id;


--
-- Name: item_catalog; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.item_catalog (
    item_id integer NOT NULL,
    patch text NOT NULL,
    name text,
    price_total integer,
    is_completed boolean DEFAULT false NOT NULL,
    is_boots boolean DEFAULT false NOT NULL,
    from_ids integer[],
    to_ids integer[],
    is_skippable boolean DEFAULT false NOT NULL
);


--
-- Name: match_bans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_bans (
    match_id text NOT NULL,
    team_id integer NOT NULL,
    pick_turn integer NOT NULL,
    champion_id integer
);


--
-- Name: match_boots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_boots (
    match_id text NOT NULL,
    participant_id smallint NOT NULL,
    item_id integer NOT NULL,
    timestamp_ms integer NOT NULL
);


--
-- Name: match_completed_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_completed_items (
    match_id text NOT NULL,
    participant_id smallint NOT NULL,
    slot smallint NOT NULL,
    item_id integer NOT NULL,
    timestamp_ms integer NOT NULL,
    is_boots boolean DEFAULT false NOT NULL
);


--
-- Name: match_item_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_item_events (
    match_id text NOT NULL,
    participant_id smallint NOT NULL,
    timestamp_ms integer NOT NULL,
    item_id integer NOT NULL,
    removal_type text
);


--
-- Name: match_participant_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_participant_snapshots (
    match_id text NOT NULL,
    participant_id smallint NOT NULL,
    minute smallint NOT NULL,
    total_gold integer,
    current_gold integer,
    cs integer,
    jungle_cs integer,
    level smallint,
    xp integer,
    pos_x integer,
    pos_y integer,
    time_enemy_cc integer,
    wards_placed smallint,
    wards_killed smallint,
    dmg_total integer,
    dmg_to_champs integer,
    dmg_magic_champs integer,
    dmg_phys_champs integer,
    dmg_true_champs integer,
    dmg_taken integer
);


--
-- Name: match_participants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_participants (
    id bigint NOT NULL,
    match_id text NOT NULL,
    puuid text,
    participant_id integer,
    tier_at_match text,
    division_at_match text,
    lp_at_match integer,
    tier_snapshot_delta_h integer,
    summoner_level integer,
    team_id integer,
    team_position text,
    individual_position text,
    lane text,
    role text,
    win boolean,
    champion_id integer,
    champion_name text,
    champ_level integer,
    champ_experience integer,
    champion_transform integer,
    player_augment1 integer,
    player_augment2 integer,
    player_augment3 integer,
    player_augment4 integer,
    player_augment5 integer,
    player_augment6 integer,
    summoner1_id integer,
    summoner2_id integer,
    summoner1_casts integer,
    summoner2_casts integer,
    spell1_casts integer,
    spell2_casts integer,
    spell3_casts integer,
    spell4_casts integer,
    kills integer,
    deaths integer,
    assists integer,
    double_kills integer,
    triple_kills integer,
    quadra_kills integer,
    penta_kills integer,
    unreal_kills integer,
    killing_sprees integer,
    largest_killing_spree integer,
    largest_multi_kill integer,
    first_blood_kill boolean,
    first_blood_assist boolean,
    longest_time_spent_living integer,
    total_time_spent_dead integer,
    total_damage_dealt integer,
    total_damage_dealt_to_champions integer,
    physical_damage_dealt integer,
    physical_damage_dealt_to_champions integer,
    magic_damage_dealt integer,
    magic_damage_dealt_to_champions integer,
    true_damage_dealt integer,
    true_damage_dealt_to_champions integer,
    largest_critical_strike integer,
    total_damage_taken integer,
    physical_damage_taken integer,
    magic_damage_taken integer,
    true_damage_taken integer,
    damage_self_mitigated integer,
    total_heal integer,
    total_heals_on_teammates integer,
    total_units_healed integer,
    total_damage_shielded_on_teammates integer,
    time_ccing_others integer,
    total_time_cc_dealt integer,
    gold_earned integer,
    gold_spent integer,
    item0 integer,
    item1 integer,
    item2 integer,
    item3 integer,
    item4 integer,
    item5 integer,
    item6 integer,
    items_purchased integer,
    consumables_purchased integer,
    role_bound_item integer,
    total_minions_killed integer,
    neutral_minions_killed integer,
    total_ally_jungle_minions integer,
    total_enemy_jungle_minions integer,
    baron_kills integer,
    dragon_kills integer,
    objectives_stolen integer,
    objectives_stolen_assists integer,
    vision_score integer,
    vision_wards_bought integer,
    sight_wards_bought integer,
    wards_placed integer,
    detector_wards_placed integer,
    wards_killed integer,
    turret_kills integer,
    turret_takedowns integer,
    turrets_lost integer,
    first_tower_kill boolean,
    first_tower_assist boolean,
    inhibitor_kills integer,
    inhibitor_takedowns integer,
    inhibitors_lost integer,
    nexus_kills integer,
    nexus_lost integer,
    nexus_takedowns integer,
    damage_dealt_to_objectives integer,
    damage_dealt_to_buildings integer,
    damage_dealt_to_turrets integer,
    damage_dealt_to_epic_monsters integer,
    all_in_pings integer,
    basic_pings integer,
    assist_me_pings integer,
    command_pings integer,
    danger_pings integer,
    enemy_missing_pings integer,
    get_back_pings integer,
    hold_pings integer,
    on_my_way_pings integer,
    need_vision_pings integer,
    push_pings integer,
    retreat_pings integer,
    enemy_vision_pings integer,
    vision_cleared_pings integer,
    game_ended_in_early_surrender boolean,
    game_ended_in_surrender boolean,
    team_early_surrendered boolean,
    time_played integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: match_participants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.match_participants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: match_participants_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.match_participants_id_seq OWNED BY public.match_participants.id;


--
-- Name: match_perks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_perks (
    match_id text NOT NULL,
    puuid text NOT NULL,
    stat_defense integer,
    stat_flex integer,
    stat_offense integer,
    style0 integer,
    style1 integer,
    perk0 integer,
    perk1 integer,
    perk2 integer,
    perk3 integer,
    perk4 integer,
    perk5 integer,
    var01 integer,
    var02 integer,
    var11 integer,
    var12 integer,
    var13 integer,
    var21 integer,
    var22 integer,
    var23 integer,
    var31 integer,
    var32 integer,
    var33 integer,
    var41 integer,
    var42 integer,
    var43 integer,
    var51 integer,
    var52 integer,
    var53 integer
);


--
-- Name: match_skill_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_skill_events (
    match_id text NOT NULL,
    participant_id smallint NOT NULL,
    timestamp_ms integer NOT NULL,
    skill_slot smallint NOT NULL,
    level_up_type text
);


--
-- Name: match_starter_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_starter_items (
    match_id text NOT NULL,
    participant_id smallint NOT NULL,
    item_id integer NOT NULL,
    timestamp_ms integer NOT NULL
);


--
-- Name: match_teams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_teams (
    match_id text NOT NULL,
    team_id integer NOT NULL,
    win boolean,
    baron_kills integer,
    dragon_kills integer,
    tower_kills integer,
    inhibitor_kills integer,
    rift_herald_kills integer,
    feats jsonb
);


--
-- Name: match_timelines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.match_timelines (
    match_id text NOT NULL,
    puuid text NOT NULL,
    boots_item_id integer,
    first_item_id integer,
    second_item_id integer,
    third_item_id integer,
    fourth_item_id integer,
    raw_events jsonb
);


--
-- Name: matches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.matches (
    match_id text NOT NULL,
    data_version text,
    platform_id text,
    queue_id integer,
    game_version text,
    game_mode text,
    game_type text,
    game_start_ts timestamp with time zone,
    game_end_ts timestamp with time zone,
    game_duration integer,
    end_of_game_result text,
    fetch_status text DEFAULT 'pending'::text NOT NULL,
    retry_count integer DEFAULT 0 NOT NULL,
    avg_tier_score integer,
    tier_coverage smallint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    region text DEFAULT 'KR'::text NOT NULL,
    version text DEFAULT ''::text NOT NULL,
    avg_tier text,
    avg_division text,
    timeline_status text DEFAULT 'pending'::text NOT NULL,
    items_status text DEFAULT 'pending'::text NOT NULL,
    timeline_retry_count integer DEFAULT 0 NOT NULL,
    items_retry_count integer DEFAULT 0 NOT NULL
);


--
-- Name: player_match_sync; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.player_match_sync (
    puuid text NOT NULL,
    last_synced_at timestamp with time zone NOT NULL,
    region text DEFAULT 'KR'::text NOT NULL
);


--
-- Name: player_rank_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.player_rank_snapshots (
    id bigint NOT NULL,
    run_id integer,
    puuid text NOT NULL,
    source text DEFAULT 'phase1'::text NOT NULL,
    league_id text,
    queue text NOT NULL,
    tier text NOT NULL,
    division text,
    league_points integer,
    wins integer,
    losses integer,
    veteran boolean,
    inactive boolean,
    fresh_blood boolean,
    hot_streak boolean,
    rank_status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    region text DEFAULT 'KR'::text NOT NULL
);


--
-- Name: player_rank_snapshots_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.player_rank_snapshots_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: player_rank_snapshots_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.player_rank_snapshots_id_seq OWNED BY public.player_rank_snapshots.id;


--
-- Name: players; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.players (
    puuid text NOT NULL,
    game_name text,
    tag_line text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    region text DEFAULT 'KR'::text NOT NULL
);


--
-- Name: runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.runs (
    id integer NOT NULL,
    status text DEFAULT 'running'::text NOT NULL,
    profile text,
    mode text NOT NULL,
    target_tiers text[],
    current_phase integer DEFAULT 0 NOT NULL,
    current_tier text,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    last_run_end timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    version text,
    rank_prefetch_tiers text[] DEFAULT '{}'::text[] NOT NULL,
    queue text DEFAULT 'RANKED_SOLO_5x5'::text NOT NULL,
    execution text DEFAULT 'pipeline'::text NOT NULL,
    region text DEFAULT 'KR'::text NOT NULL
);


--
-- Name: runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.runs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: runs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.runs_id_seq OWNED BY public.runs.id;


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: game_versions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.game_versions ALTER COLUMN id SET DEFAULT nextval('public.game_versions_id_seq'::regclass);


--
-- Name: match_participants id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_participants ALTER COLUMN id SET DEFAULT nextval('public.match_participants_id_seq'::regclass);


--
-- Name: player_rank_snapshots id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_rank_snapshots ALTER COLUMN id SET DEFAULT nextval('public.player_rank_snapshots_id_seq'::regclass);


--
-- Name: runs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runs ALTER COLUMN id SET DEFAULT nextval('public.runs_id_seq'::regclass);


--
-- Name: game_versions game_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.game_versions
    ADD CONSTRAINT game_versions_pkey PRIMARY KEY (id);


--
-- Name: game_versions game_versions_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.game_versions
    ADD CONSTRAINT game_versions_version_key UNIQUE (version);


--
-- Name: item_catalog item_catalog_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.item_catalog
    ADD CONSTRAINT item_catalog_pkey PRIMARY KEY (item_id, patch);


--
-- Name: match_bans match_bans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_bans
    ADD CONSTRAINT match_bans_pkey PRIMARY KEY (match_id, team_id, pick_turn);


--
-- Name: match_boots match_boots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_boots
    ADD CONSTRAINT match_boots_pkey PRIMARY KEY (match_id, participant_id);


--
-- Name: match_completed_items match_completed_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_completed_items
    ADD CONSTRAINT match_completed_items_pkey PRIMARY KEY (match_id, participant_id, slot);


--
-- Name: match_participant_snapshots match_participant_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_participant_snapshots
    ADD CONSTRAINT match_participant_snapshots_pkey PRIMARY KEY (match_id, participant_id, minute);


--
-- Name: match_participants match_participants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_participants
    ADD CONSTRAINT match_participants_pkey PRIMARY KEY (id);


--
-- Name: match_perks match_perks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_perks
    ADD CONSTRAINT match_perks_pkey PRIMARY KEY (match_id, puuid);


--
-- Name: match_teams match_teams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_teams
    ADD CONSTRAINT match_teams_pkey PRIMARY KEY (match_id, team_id);


--
-- Name: match_timelines match_timelines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_timelines
    ADD CONSTRAINT match_timelines_pkey PRIMARY KEY (match_id, puuid);


--
-- Name: matches matches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.matches
    ADD CONSTRAINT matches_pkey PRIMARY KEY (match_id);


--
-- Name: player_match_sync player_match_sync_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_match_sync
    ADD CONSTRAINT player_match_sync_pkey PRIMARY KEY (puuid, region);


--
-- Name: player_rank_snapshots player_rank_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_rank_snapshots
    ADD CONSTRAINT player_rank_snapshots_pkey PRIMARY KEY (id);


--
-- Name: players players_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.players
    ADD CONSTRAINT players_pkey PRIMARY KEY (puuid);


--
-- Name: runs runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runs
    ADD CONSTRAINT runs_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: idx_bans_champion; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bans_champion ON public.match_bans USING btree (champion_id);


--
-- Name: idx_completed_items_match; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_completed_items_match ON public.match_completed_items USING btree (match_id);


--
-- Name: idx_item_events_match; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_item_events_match ON public.match_item_events USING btree (match_id, participant_id);


--
-- Name: idx_matches_fetch_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_matches_fetch_status ON public.matches USING btree (fetch_status);


--
-- Name: idx_matches_game_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_matches_game_version ON public.matches USING btree (game_version);


--
-- Name: idx_matches_region_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_matches_region_status ON public.matches USING btree (region, fetch_status);


--
-- Name: idx_matches_timeline; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_matches_timeline ON public.matches USING btree (region, version, timeline_status) WHERE (fetch_status = 'done'::text);


--
-- Name: idx_participants_champion; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_participants_champion ON public.match_participants USING btree (champion_id);


--
-- Name: idx_participants_match; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_participants_match ON public.match_participants USING btree (match_id);


--
-- Name: idx_participants_puuid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_participants_puuid ON public.match_participants USING btree (puuid);


--
-- Name: idx_participants_tier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_participants_tier ON public.match_participants USING btree (tier_at_match);


--
-- Name: idx_rank_snapshots_puuid_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rank_snapshots_puuid_created ON public.player_rank_snapshots USING btree (puuid, created_at DESC);


--
-- Name: idx_rank_snapshots_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_rank_snapshots_run ON public.player_rank_snapshots USING btree (run_id, source);


--
-- Name: idx_skill_events_match; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_skill_events_match ON public.match_skill_events USING btree (match_id, participant_id);


--
-- Name: idx_starter_items_match; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_starter_items_match ON public.match_starter_items USING btree (match_id, participant_id);


--
-- Name: match_bans match_bans_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_bans
    ADD CONSTRAINT match_bans_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_boots match_boots_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_boots
    ADD CONSTRAINT match_boots_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_completed_items match_completed_items_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_completed_items
    ADD CONSTRAINT match_completed_items_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_item_events match_item_events_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_item_events
    ADD CONSTRAINT match_item_events_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_participant_snapshots match_participant_snapshots_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_participant_snapshots
    ADD CONSTRAINT match_participant_snapshots_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_participants match_participants_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_participants
    ADD CONSTRAINT match_participants_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_participants match_participants_puuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_participants
    ADD CONSTRAINT match_participants_puuid_fkey FOREIGN KEY (puuid) REFERENCES public.players(puuid);


--
-- Name: match_perks match_perks_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_perks
    ADD CONSTRAINT match_perks_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_perks match_perks_puuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_perks
    ADD CONSTRAINT match_perks_puuid_fkey FOREIGN KEY (puuid) REFERENCES public.players(puuid);


--
-- Name: match_skill_events match_skill_events_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_skill_events
    ADD CONSTRAINT match_skill_events_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_starter_items match_starter_items_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_starter_items
    ADD CONSTRAINT match_starter_items_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_teams match_teams_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_teams
    ADD CONSTRAINT match_teams_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_timelines match_timelines_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_timelines
    ADD CONSTRAINT match_timelines_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.matches(match_id);


--
-- Name: match_timelines match_timelines_puuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.match_timelines
    ADD CONSTRAINT match_timelines_puuid_fkey FOREIGN KEY (puuid) REFERENCES public.players(puuid);


--
-- Name: player_match_sync player_match_sync_puuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_match_sync
    ADD CONSTRAINT player_match_sync_puuid_fkey FOREIGN KEY (puuid) REFERENCES public.players(puuid);


--
-- Name: player_rank_snapshots player_rank_snapshots_puuid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_rank_snapshots
    ADD CONSTRAINT player_rank_snapshots_puuid_fkey FOREIGN KEY (puuid) REFERENCES public.players(puuid);


--
-- Name: player_rank_snapshots player_rank_snapshots_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_rank_snapshots
    ADD CONSTRAINT player_rank_snapshots_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id);


--
-- PostgreSQL database dump complete
--

\unrestrict A2NpbHaVT8cqZSbGxZ5tYQ55glKycQRhYcMfFIaHVymEfTBnsNSrFpt3DmOeJT7

