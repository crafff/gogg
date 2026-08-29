#!/usr/bin/env python3
"""Role-wide EBM using actionable metrics normalized within each champion."""

import argparse
import json
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from interpret.glassbox import ExplainableBoostingClassifier
from sklearn.metrics import accuracy_score, average_precision_score, brier_score_loss, roc_auc_score
from sklearn.model_selection import GroupShuffleSplit


BASE_METRICS = [
    "jungle_cs_10", "dmg_to_champs_10", "dmg_taken_10",
    "time_enemy_cc_10", "wards_placed_10", "wards_killed_10",
    "jungle_cs_gain_10_15", "dmg_to_champs_gain_10_15", "dmg_taken_gain_10_15",
    "time_enemy_cc_gain_10_15", "wards_placed_gain_10_15", "wards_killed_gain_10_15",
]


def prepare(raw):
    frame = raw.copy()
    for stem in ("jungle_cs", "dmg_to_champs", "dmg_taken", "time_enemy_cc", "wards_placed", "wards_killed"):
        frame[f"{stem}_gain_10_15"] = frame[f"{stem}_15"] - frame[f"{stem}_10"]
    frame["win"] = frame.win.astype(str).str.lower().map({"true": 1, "false": 0, "1": 1, "0": 0})
    # Percentiles make a 70th-percentile Ivern comparable to a 70th-percentile Karthus.
    for metric in BASE_METRICS:
        frame[f"{metric}_champion_pct"] = frame.groupby("champion_id")[metric].rank(pct=True)
    return frame


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/jungle-role-ebm"))
    parser.add_argument("--seed", type=int, default=421)
    parser.add_argument("--min-champion-games", type=int, default=100)
    args = parser.parse_args()
    frame = prepare(pd.read_csv(args.input))
    counts = frame.champion_id.value_counts()
    frame = frame[frame.champion_id.isin(counts[counts >= args.min_champion_games].index)].copy()
    features = [f"{metric}_champion_pct" for metric in BASE_METRICS]
    controls = ["champion_name", "region", "game_version"]
    data = frame.dropna(subset=features + controls + ["win", "puuid"]).copy()
    for column in controls:
        data[column] = data[column].astype(str)

    splitter = GroupShuffleSplit(n_splits=1, test_size=0.2, random_state=args.seed)
    train_idx, test_idx = next(splitter.split(data, data.win, groups=data.puuid))
    train, test = data.iloc[train_idx], data.iloc[test_idx]
    model_features = features + controls
    model = ExplainableBoostingClassifier(
        feature_names=model_features,
        feature_types=["continuous"] * len(features) + ["nominal"] * len(controls),
        interactions=0, max_bins=64, max_rounds=5000, learning_rate=0.03,
        validation_size=0.15, early_stopping_rounds=100, outer_bags=8,
        random_state=args.seed, n_jobs=-1,
    ).fit(train[model_features], train.win)
    probability = model.predict_proba(test[model_features])[:, 1]
    metrics = {
        "rows": len(data), "champions": int(data.champion_id.nunique()),
        "players": int(data.puuid.nunique()), "train_rows": len(train), "test_rows": len(test),
        "accuracy_at_0_5": float(accuracy_score(test.win, probability >= 0.5)),
        "roc_auc": float(roc_auc_score(test.win, probability)),
        "pr_auc": float(average_precision_score(test.win, probability)),
        "brier": float(brier_score_loss(test.win, probability)),
    }
    output = args.output_dir
    output.mkdir(parents=True, exist_ok=True)
    (output / "metrics.json").write_text(json.dumps(metrics, indent=2) + "\n")
    explanation = model.explain_global()
    importance = pd.DataFrame({
        "feature": explanation.data()["names"],
        "mean_abs_score": explanation.data()["scores"],
    }).sort_values("mean_abs_score", ascending=False)
    importance.to_csv(output / "importance.csv", index=False)

    ranked = [f for f in importance.feature if f in features][:10]
    fig, axes = plt.subplots(5, 2, figsize=(13, 17))
    for axis, feature in zip(axes.ravel(), ranked):
        term = explanation.data(model_features.index(feature))
        names, scores = np.asarray(term["names"]), np.asarray(term["scores"])
        size = min(len(names), len(scores))
        axis.plot(100 * names[:size], scores[:size], marker=".", linewidth=1)
        axis.axhline(0, color="grey", linewidth=0.8)
        axis.set(title=feature.replace("_champion_pct", ""), xlabel="within-champion percentile", ylabel="EBM log-odds")
    fig.tight_layout()
    fig.savefig(output / "universal_action_curves.png", dpi=180)
    plt.close(fig)

    # Decile table is deliberately descriptive and easy to audit.
    rows = []
    for metric in features:
        bins = pd.cut(data[metric], bins=np.linspace(0, 1, 11), include_lowest=True)
        for interval, group in data.groupby(bins, observed=True):
            rows.append({"feature": metric, "percentile_bin": str(interval), "n": len(group), "win_rate": group.win.mean()})
    pd.DataFrame(rows).to_csv(output / "decile_win_rates.csv", index=False)
    print(json.dumps(metrics, indent=2))
    print(importance.head(15).to_string(index=False))


if __name__ == "__main__":
    main()
