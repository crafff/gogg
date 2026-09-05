#!/usr/bin/env python3
"""Precompute auditable Top-5 recommendations with explicit slot semantics."""

import argparse
import json
from pathlib import Path

import lightgbm as lgb
import numpy as np
import pandas as pd

from hybrid_ranker import build_catalog, candidate_frame, patch_key
from profiles import STYLE_FEATURES, add_rates, available_style_features


def reason_codes(row, win_threshold, popularity_threshold):
    reasons = []
    if row.history_games >= 3:
        reasons.append("EXPERIENCED")
    if row.recency_days <= 14:
        reasons.append("RECENTLY_PLAYED")
    if row.champion_win_rate >= win_threshold:
        reasons.append("PATCH_STRONG")
    if row.champion_popularity >= popularity_threshold:
        reasons.append("PATCH_POPULAR")
    distances = {
        "STYLE_KILLS": row.style_absdiff_kills_pm,
        "STYLE_ASSISTS": row.style_absdiff_assists_pm,
        "STYLE_DAMAGE": row.style_absdiff_damage_pm,
        "STYLE_VISION": row.style_absdiff_vision_pm,
        "STYLE_GOLD": row.style_absdiff_gold_pm,
    }
    reasons.append(min(distances, key=distances.get))
    return reasons[:3]


def select_slots(group, similarity, id_to_index, relevance_weight=0.85):
    ranked = group.sort_values("score", ascending=False).copy()
    selected = []
    for row in ranked.itertuples():
        if len(selected) == 3:
            break
        selected.append((row, "RELEVANCE"))

    unseen = ranked[(ranked.history_games == 0) & ~ranked.champion_id.isin([row.champion_id for row, _ in selected])]
    if not unseen.empty:
        selected.append((next(unseen.itertuples()), "NEW_LOW_RISK"))

    remaining = unseen[~unseen.champion_id.isin([row.champion_id for row, _ in selected])].copy()
    if not remaining.empty:
        low, high = remaining.score.min(), remaining.score.max()
        remaining["normalized_score"] = (remaining.score - low) / max(high - low, 1e-9)
        best_row, best_value = None, -np.inf
        selected_ids = [int(row.champion_id) for row, _ in selected]
        for row in remaining.itertuples():
            item = int(row.champion_id)
            redundancy = max(
                (similarity[id_to_index[item], id_to_index[other]] for other in selected_ids),
                default=0,
            )
            value = relevance_weight * row.normalized_score - (1 - relevance_weight) * redundancy
            if value > best_value:
                best_row, best_value = row, value
        selected.append((best_row, "DIVERSE_EXPLORATION"))

    # Very broad historical pools can exhaust unseen candidates; backfill by score.
    if len(selected) < 5:
        used = {int(row.champion_id) for row, _ in selected}
        for row in ranked.itertuples():
            if int(row.champion_id) not in used:
                selected.append((row, "RELEVANCE_BACKFILL"))
                used.add(int(row.champion_id))
            if len(selected) == 5:
                break
    return selected


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/precomputed"))
    parser.add_argument("--min-games", type=int, default=5)
    parser.add_argument("--model-version", default="jungle-hybrid-20260721-v1")
    args = parser.parse_args()

    frame = add_rates(pd.read_csv(args.input, dtype={"patch": str}))
    frame["game_start_ts"] = pd.to_datetime(frame.game_start_ts, format="mixed", utc=True)
    patches = sorted(frame.patch.unique(), key=patch_key)
    current_patch = patches[-1]
    history = frame[frame.patch != current_patch].sort_values("game_start_ts").copy()
    active = frame[frame.patch == current_patch].puuid.drop_duplicates()
    counts = history.groupby("puuid").size()
    users = sorted(set(active) & set(counts[counts >= args.min_games].index))
    style_features = available_style_features(frame)
    catalog = build_catalog(history, style_features)
    dummy_target = int(catalog.iloc[0].champion_id)
    targets = pd.DataFrame({"puuid": users, "champion_id": dummy_target})
    candidates = candidate_frame(history, targets, catalog, style_features, all_candidates=True)
    booster = lgb.Booster(model_file=str(args.model))
    feature_columns = booster.feature_name()
    candidates["score"] = booster.predict(candidates[feature_columns])

    champion_columns = [f"champion_{feature}" for feature in STYLE_FEATURES]
    vectors = catalog.set_index("champion_id")[champion_columns].copy()
    vectors = (vectors - vectors.mean()) / vectors.std().replace(0, 1)
    matrix = vectors.to_numpy()
    matrix /= np.maximum(np.linalg.norm(matrix, axis=1, keepdims=True), 1e-9)
    similarity = matrix @ matrix.T
    id_to_index = {int(item): index for index, item in enumerate(vectors.index)}
    win_threshold = catalog.champion_win_rate.quantile(0.75)
    popularity_threshold = catalog.champion_popularity.quantile(0.75)

    output_rows = []
    for puuid, group in candidates.groupby("puuid", sort=False):
        confidence = min(1.0, np.log1p(group.iloc[0].user_games) / np.log1p(50))
        for rank, (row, slot) in enumerate(select_slots(group, similarity, id_to_index), 1):
            reasons = reason_codes(row, win_threshold, popularity_threshold)
            if slot == "NEW_LOW_RISK":
                reasons.insert(0, "NEW_TO_PLAYER")
            elif slot == "DIVERSE_EXPLORATION":
                reasons.insert(0, "DIVERSE_EXPLORATION")
            output_rows.append({
                "puuid": puuid, "position": "JUNGLE", "champion_id": int(row.champion_id),
                "champion_name": row.champion_name, "rank": rank, "slot_type": slot,
                "final_score": float(row.score), "fit_confidence": confidence,
                "patch_score": float(row.champion_win_rate),
                "history_games": int(row.history_games), "reason_codes": "|".join(reasons[:3]),
                "model_version": args.model_version, "feature_patch": patches[-2],
                "target_patch": current_patch,
            })
    output = pd.DataFrame(output_rows)
    args.output_dir.mkdir(parents=True, exist_ok=True)
    output.to_csv(args.output_dir / "recommendations.csv", index=False)
    per_user = output.groupby("puuid").agg(
        recommendations=("champion_id", "size"), unique_champions=("champion_id", "nunique"),
        new_slots=("slot_type", lambda values: int(values.isin(["NEW_LOW_RISK", "DIVERSE_EXPLORATION"]).sum())),
    )
    report = {
        "model_version": args.model_version, "users": int(output.puuid.nunique()),
        "rows": len(output), "catalog": int(catalog.champion_id.nunique()),
        "users_with_five_unique": int(((per_user.recommendations == 5) & (per_user.unique_champions == 5)).sum()),
        "users_with_two_exploration_slots": int((per_user.new_slots == 2).sum()),
        "mean_confidence": float(output.groupby("puuid").fit_confidence.first().mean()),
        "slot_counts": {str(k): int(v) for k, v in output.slot_type.value_counts().items()},
    }
    (args.output_dir / "metrics.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
