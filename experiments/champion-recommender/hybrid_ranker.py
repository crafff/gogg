#!/usr/bin/env python3
"""Point-in-time hybrid LambdaRank baseline for champion recommendation."""

import argparse
import json
import math
from pathlib import Path

import lightgbm as lgb
import numpy as np
import pandas as pd

from profiles import STYLE_FEATURES, add_rates, available_style_features


def patch_key(value):
    return tuple(map(int, str(value).split(".")))


def build_catalog(history, style_features):
    global_win = history.win.mean()
    catalog = history.groupby(["champion_id", "champion_name"]).agg(
        champion_games=("match_id", "size"), champion_wins=("win", "sum"),
        **{f"champion_{feature}": (feature, "mean") for feature in style_features},
    ).reset_index()
    catalog["champion_win_rate"] = (catalog.champion_wins + 100 * global_win) / (catalog.champion_games + 100)
    catalog["champion_popularity"] = catalog.champion_games / catalog.champion_games.sum()
    return catalog


def build_users(history, style_features):
    return history.groupby("puuid").agg(
        user_games=("match_id", "size"), user_wins=("win", "sum"),
        user_pool_size=("champion_id", "nunique"),
        **{f"user_{feature}": (feature, "mean") for feature in style_features},
    ).reset_index()


def build_history_pairs(history, as_of):
    pairs = history.groupby(["puuid", "champion_id"]).agg(
        history_games=("match_id", "size"), history_wins=("win", "sum"),
        last_played=("game_start_ts", "max"),
    ).reset_index()
    pairs["history_win_rate"] = (pairs.history_wins + 5 * history.win.mean()) / (pairs.history_games + 5)
    pairs["recency_days"] = (as_of - pairs.last_played).dt.total_seconds().div(86400).clip(lower=0)
    return pairs.drop(columns=["last_played"])


def candidate_frame(history, targets, catalog, style_features, all_candidates=False, negatives=40, seed=421):
    users = build_users(history, style_features).merge(
        targets[["puuid", "champion_id"]].rename(columns={"champion_id": "target_champion_id"}),
        on="puuid", how="inner",
    )
    rng = np.random.default_rng(seed)
    champion_ids = catalog.champion_id.to_numpy()
    rows = []
    for row in users.itertuples():
        if all_candidates:
            choices = champion_ids
        else:
            pool = champion_ids[champion_ids != row.target_champion_id]
            choices = np.concatenate(([row.target_champion_id], rng.choice(pool, min(negatives, len(pool)), replace=False)))
        rows.extend((row.puuid, int(item), int(item == row.target_champion_id)) for item in choices)
    candidates = pd.DataFrame(rows, columns=["puuid", "champion_id", "label"])
    candidates = candidates.merge(users.drop(columns=["target_champion_id"]), on="puuid").merge(catalog, on="champion_id")
    pairs = build_history_pairs(history[history.puuid.isin(users.puuid)], history.game_start_ts.max())
    candidates = candidates.merge(pairs, on=["puuid", "champion_id"], how="left")
    candidates[["history_games", "history_wins"]] = candidates[["history_games", "history_wins"]].fillna(0)
    candidates["history_win_rate"] = candidates.history_win_rate.fillna(history.win.mean())
    candidates["recency_days"] = candidates.recency_days.fillna(999)
    candidates["is_new_champion"] = (candidates.history_games == 0).astype(int)
    candidates["log_history_games"] = np.log1p(candidates.history_games)
    candidates["log_user_games"] = np.log1p(candidates.user_games)
    for feature in style_features:
        candidates[f"style_absdiff_{feature}"] = (candidates[f"user_{feature}"] - candidates[f"champion_{feature}"]).abs()
    return candidates.sort_values(["puuid", "champion_id"]).reset_index(drop=True)


def ranking_metrics(frame, scores, k, discovery_only=False):
    ranked = frame.assign(score=scores).sort_values(["puuid", "score"], ascending=[True, False])
    hits, ndcgs, recommended = [], [], set()
    for _, group in ranked.groupby("puuid", sort=False):
        target = group[group.label == 1]
        if target.empty or (discovery_only and target.iloc[0].is_new_champion != 1):
            continue
        candidates = group[group.is_new_champion == 1] if discovery_only else group
        top = candidates.head(k)
        positions = np.flatnonzero(top.label.to_numpy() == 1)
        hits.append(float(len(positions) > 0))
        ndcgs.append(1 / math.log2(int(positions[0]) + 2) if len(positions) else 0.0)
        recommended.update(top.champion_id.astype(int))
    return {
        f"hit_rate@{k}": float(np.mean(hits)), f"ndcg@{k}": float(np.mean(ndcgs)),
        "users": len(hits), "coverage": len(recommended) / frame.champion_id.nunique(),
    }


def diverse_discovery_metrics(frame, scores, catalog, k, relevance_weight=0.85):
    champion_columns = [f"champion_{feature}" for feature in STYLE_FEATURES]
    style = catalog.set_index("champion_id")[champion_columns].copy()
    style = (style - style.mean()) / style.std().replace(0, 1)
    vectors = style.to_numpy()
    vectors /= np.maximum(np.linalg.norm(vectors, axis=1, keepdims=True), 1e-9)
    similarity = vectors @ vectors.T
    ids = style.index.astype(int).to_list()
    id_to_index = {item: index for index, item in enumerate(ids)}
    ranked = frame.assign(score=scores)
    hits, ndcgs, recommended, diversities = [], [], set(), []
    for _, group in ranked.groupby("puuid", sort=False):
        target = group[group.label == 1]
        if target.empty or target.iloc[0].is_new_champion != 1:
            continue
        candidates = group[group.is_new_champion == 1].copy()
        low, high = candidates.score.min(), candidates.score.max()
        candidates["relevance"] = (candidates.score - low) / max(high - low, 1e-9)
        selected = []
        while len(selected) < k and len(selected) < len(candidates):
            best_id, best_value = None, -np.inf
            for row in candidates.itertuples():
                item = int(row.champion_id)
                if item in selected:
                    continue
                redundancy = max((similarity[id_to_index[item], id_to_index[other]] for other in selected), default=0)
                value = relevance_weight * row.relevance - (1 - relevance_weight) * redundancy
                if value > best_value:
                    best_id, best_value = item, value
            selected.append(best_id)
        target_id = int(target.iloc[0].champion_id)
        hits.append(float(target_id in selected))
        ndcgs.append(1 / math.log2(selected.index(target_id) + 2) if target_id in selected else 0.0)
        recommended.update(selected)
        if len(selected) > 1:
            pair_sims = [similarity[id_to_index[a], id_to_index[b]] for i, a in enumerate(selected) for b in selected[i + 1:]]
            diversities.append(1 - float(np.mean(pair_sims)))
    return {
        f"hit_rate@{k}": float(np.mean(hits)), f"ndcg@{k}": float(np.mean(ndcgs)),
        "users": len(hits), "coverage": len(recommended) / frame.champion_id.nunique(),
        "mean_intra_list_diversity": float(np.mean(diversities)),
        "relevance_weight": relevance_weight,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/hybrid-ranker"))
    parser.add_argument("--min-games", type=int, default=5)
    parser.add_argument("--top-k", type=int, default=5)
    args = parser.parse_args()
    frame = add_rates(pd.read_csv(args.input, dtype={"patch": str}))
    style_features = available_style_features(frame)
    frame["game_start_ts"] = pd.to_datetime(frame.game_start_ts, format="mixed", utc=True)
    patches = sorted(frame.patch.unique(), key=patch_key)
    development = frame[frame.patch != patches[-1]].sort_values("game_start_ts").copy()
    test_events = frame[frame.patch == patches[-1]].sort_values("game_start_ts").copy()

    cutoff = development.game_start_ts.quantile(0.8)
    rank_history = development[development.game_start_ts < cutoff]
    rank_future = development[development.game_start_ts >= cutoff]
    eligible_train = set(rank_history.groupby("puuid").size().loc[lambda s: s >= args.min_games].index)
    rank_targets = rank_future[rank_future.puuid.isin(eligible_train)].groupby("puuid", sort=False).first().reset_index()
    train_candidates = candidate_frame(
        rank_history, rank_targets, build_catalog(rank_history, style_features), style_features,
    )

    familiarity_features = [
        "log_history_games", "history_win_rate", "recency_days", "is_new_champion",
        "log_user_games", "user_pool_size",
    ]
    patch_features = ["champion_win_rate", "champion_popularity"]
    terminal_style = [f"style_absdiff_{feature}" for feature in STYLE_FEATURES]
    timeline_style = [
        f"style_absdiff_{feature}" for feature in style_features if feature not in STYLE_FEATURES
    ]
    feature_sets = {
        "familiarity": familiarity_features,
        "familiarity_patch": familiarity_features + patch_features,
        "terminal_style": familiarity_features + patch_features + terminal_style,
        "full_timeline": familiarity_features + patch_features + terminal_style + timeline_style,
    }
    groups = train_candidates.groupby("puuid", sort=False).size().to_numpy()

    eligible_test = set(development.groupby("puuid").size().loc[lambda s: s >= args.min_games].index)
    test_targets = test_events[test_events.puuid.isin(eligible_test)].groupby("puuid", sort=False).first().reset_index()
    test_catalog = build_catalog(development, style_features)
    test_candidates = candidate_frame(
        development, test_targets, test_catalog,
        style_features, all_candidates=True,
    )
    ablations = {}
    selected_model = None
    for name, feature_columns in feature_sets.items():
        model = lgb.LGBMRanker(
            objective="lambdarank", metric="ndcg",
            n_estimators=350, learning_rate=0.03, num_leaves=31,
            min_child_samples=30, reg_lambda=1.0, random_state=421, verbosity=-1,
        )
        model.fit(train_candidates[feature_columns], train_candidates.label, group=groups)
        scores = model.predict(test_candidates[feature_columns])
        ablations[name] = {
            "features": feature_columns,
            "next_choice": ranking_metrics(test_candidates, scores, args.top_k),
            "new_champion_discovery": ranking_metrics(test_candidates, scores, args.top_k, True),
        }
        if name == "terminal_style":
            ablations[name]["diverse_new_champion_discovery"] = diverse_discovery_metrics(
                test_candidates, scores, test_catalog, args.top_k,
            )
        if name == "terminal_style":
            selected_model = model
    report = {
        "train_patches": patches[:-1], "test_patch": patches[-1],
        "rank_train_users": int(train_candidates.puuid.nunique()),
        "test_users": int(test_candidates.puuid.nunique()),
        "ablations": ablations,
    }
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / "metrics.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    feature_columns = feature_sets["terminal_style"]
    selected_model.booster_.save_model(args.output_dir / "model.txt")
    pd.DataFrame({
        "feature": feature_columns, "gain": selected_model.booster_.feature_importance("gain"),
    }).sort_values("gain", ascending=False).to_csv(args.output_dir / "feature_importance.csv", index=False)
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
