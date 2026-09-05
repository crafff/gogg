#!/usr/bin/env python3
"""Score one Riot API user against the current-patch jungle catalog."""

import argparse
import json
from pathlib import Path

import lightgbm as lgb
import numpy as np
import pandas as pd

from hybrid_ranker import build_catalog, candidate_frame, patch_key
from precompute import reason_codes, select_slots
from profiles import STYLE_FEATURES, add_rates


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--user-input", type=Path, required=True)
    parser.add_argument("--catalog-input", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--output", type=Path, default=Path("data/user_recommendations.csv"))
    args = parser.parse_args()

    user = add_rates(pd.read_csv(args.user_input, dtype={"patch": str}))
    user["game_start_ts"] = pd.to_datetime(user.game_start_ts, format="mixed", utc=True)
    user = user[user.position == "JUNGLE"].sort_values("game_start_ts").copy()
    if len(user) < 5:
        raise SystemExit(f"only {len(user)} jungle games; need at least 5")
    global_frame = add_rates(pd.read_csv(args.catalog_input, dtype={"patch": str}))
    global_frame["game_start_ts"] = pd.to_datetime(global_frame.game_start_ts, format="mixed", utc=True)
    current_patch = max(global_frame.patch.unique(), key=patch_key)
    patch_frame = global_frame[global_frame.patch == current_patch].copy()
    catalog = build_catalog(patch_frame, STYLE_FEATURES)

    puuid = user.iloc[0].puuid
    targets = pd.DataFrame({"puuid": [puuid], "champion_id": [int(catalog.iloc[0].champion_id)]})
    candidates = candidate_frame(user, targets, catalog, STYLE_FEATURES, all_candidates=True)
    booster = lgb.Booster(model_file=str(args.model))
    candidates["score"] = booster.predict(candidates[booster.feature_name()])

    champion_columns = [f"champion_{feature}" for feature in STYLE_FEATURES]
    vectors = catalog.set_index("champion_id")[champion_columns].copy()
    vectors = (vectors - vectors.mean()) / vectors.std().replace(0, 1)
    matrix = vectors.to_numpy()
    matrix /= np.maximum(np.linalg.norm(matrix, axis=1, keepdims=True), 1e-9)
    similarity = matrix @ matrix.T
    id_to_index = {int(item): index for index, item in enumerate(vectors.index)}
    win_threshold = catalog.champion_win_rate.quantile(0.75)
    popularity_threshold = catalog.champion_popularity.quantile(0.75)

    rows = []
    for rank, (row, slot) in enumerate(select_slots(candidates, similarity, id_to_index), 1):
        reasons = reason_codes(row, win_threshold, popularity_threshold)
        if slot == "NEW_LOW_RISK":
            reasons.insert(0, "NEW_TO_PLAYER")
        elif slot == "DIVERSE_EXPLORATION":
            reasons.insert(0, "DIVERSE_EXPLORATION")
        rows.append({
            "rank": rank, "champion_id": int(row.champion_id), "champion_name": row.champion_name,
            "slot_type": slot, "score": float(row.score),
            "patch_win_rate": float(row.champion_win_rate),
            "history_games": int(row.history_games), "reason_codes": "|".join(reasons[:3]),
        })
    output = pd.DataFrame(rows)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    output.to_csv(args.output, index=False)
    history = user.groupby("champion_name").agg(games=("match_id", "size"), wins=("win", "sum")).sort_values("games", ascending=False)
    history["win_rate"] = history.wins / history.games
    report = {
        "ranked_games": len(user), "distinct_champions": int(user.champion_id.nunique()),
        "win_rate": float(user.win.mean()), "current_catalog_patch": current_patch,
        "top_history": history.head(10).reset_index().to_dict("records"),
        "recommendations": output.to_dict("records"),
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
