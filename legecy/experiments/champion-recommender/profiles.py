#!/usr/bin/env python3
"""Build player/champion gameplay profiles and explanatory clusters."""

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.cluster import KMeans
from sklearn.decomposition import PCA
from sklearn.metrics import silhouette_score
from sklearn.preprocessing import StandardScaler

STYLE_FEATURES = ["kills_pm", "deaths_pm", "assists_pm", "damage_pm", "vision_pm", "gold_pm"]
TIMELINE_STYLE_FEATURES = [
    "jungle_cs_10", "jungle_cs_gain_10_15", "lane_cs_10", "lane_cs_gain_10_15",
    "damage_10", "damage_gain_10_15", "damage_taken_10", "damage_taken_gain_10_15",
    "cc_10", "cc_gain_10_15", "wards_killed_10", "wards_killed_gain_10_15",
]


def available_style_features(frame):
    return STYLE_FEATURES + [feature for feature in TIMELINE_STYLE_FEATURES if feature in frame.columns]


def add_rates(frame):
    result = frame.copy()
    minutes = (result.time_played / 60).clip(lower=10)
    sources = {
        "kills_pm": "kills", "deaths_pm": "deaths", "assists_pm": "assists",
        "damage_pm": "total_damage_dealt_to_champions", "vision_pm": "vision_score",
        "gold_pm": "gold_earned",
    }
    for output, source in sources.items():
        result[output] = result[source].fillna(0) / minutes
    result["win"] = result.win.astype(str).str.lower().map({"true": 1, "false": 0, "1": 1, "0": 0})
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/profiles"))
    parser.add_argument("--min-games", type=int, default=20)
    args = parser.parse_args()
    frame = add_rates(pd.read_csv(args.input, dtype={"patch": str}))
    style_features = available_style_features(frame)
    patches = sorted(frame.patch.unique(), key=lambda p: tuple(map(int, p.split("."))))
    history = frame[frame.patch != patches[-1]].copy()
    global_win = history.win.mean()
    normalized_history = history.copy()
    for feature in style_features:
        grouped = history.groupby("champion_id")[feature]
        mean = grouped.transform("mean")
        std = grouped.transform("std").replace(0, np.nan)
        normalized_history[feature] = ((history[feature] - mean) / std).fillna(0).clip(-5, 5)
    players = normalized_history.groupby("puuid").agg(
        games=("match_id", "size"), wins=("win", "sum"),
        champion_pool_size=("champion_id", "nunique"),
        **{feature: (feature, "mean") for feature in style_features},
    ).reset_index()
    players["smoothed_win_rate"] = (players.wins + 20 * global_win) / (players.games + 20)
    players["profile_confidence"] = players.games / (players.games + 20)
    eligible = players[players.games >= args.min_games].copy()
    champions = history.groupby(["champion_id", "champion_name"]).agg(
        games=("match_id", "size"), wins=("win", "sum"),
        **{feature: (feature, "mean") for feature in style_features},
    ).reset_index()
    champions["smoothed_win_rate"] = (champions.wins + 100 * global_win) / (champions.games + 100)
    champions["pick_share"] = champions.games / champions.games.sum()
    champions["profile_confidence"] = champions.games / (champions.games + 100)

    values = StandardScaler().fit_transform(eligible[style_features])
    pca = PCA(n_components=4, random_state=421)
    embedded = pca.fit_transform(values)
    sample = embedded if len(embedded) <= 10000 else embedded[np.random.default_rng(421).choice(len(embedded), 10000, replace=False)]
    scores = {}
    for clusters in range(3, 9):
        labels = KMeans(n_clusters=clusters, n_init=20, random_state=421).fit_predict(sample)
        scores[clusters] = float(silhouette_score(sample, labels))
    best_k = max(scores, key=scores.get)
    eligible["cluster"] = KMeans(n_clusters=best_k, n_init=30, random_state=421).fit_predict(embedded)
    for index in range(embedded.shape[1]):
        eligible[f"profile_pc{index + 1}"] = embedded[:, index]

    args.output_dir.mkdir(parents=True, exist_ok=True)
    players.to_csv(args.output_dir / "player_profiles.csv", index=False)
    eligible.to_csv(args.output_dir / "clustered_player_profiles.csv", index=False)
    champions.to_csv(args.output_dir / "champion_profiles.csv", index=False)
    summary = eligible.groupby("cluster").agg(
        players=("puuid", "size"), games=("games", "median"),
        **{feature: (feature, "mean") for feature in style_features},
    )
    summary.to_csv(args.output_dir / "cluster_summary.csv")
    report = {
        "history_patches": patches[:-1], "held_out_patch": patches[-1],
        "all_players": len(players), "eligible_players": len(eligible),
        "champions": len(champions), "min_games": args.min_games,
        "selected_clusters": best_k,
        "silhouette_by_k": {str(k): v for k, v in scores.items()},
        "pca_explained_variance": pca.explained_variance_ratio_.tolist(),
    }
    (args.output_dir / "metrics.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    print(summary.round(3).to_string())


if __name__ == "__main__":
    main()
