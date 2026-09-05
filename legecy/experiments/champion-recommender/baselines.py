#!/usr/bin/env python3
"""Temporal recommendation baselines: popularity, history, Item-KNN and ALS."""

import argparse
import json
import math
from pathlib import Path

import numpy as np
import pandas as pd
from scipy.sparse import csr_matrix
from sklearn.decomposition import TruncatedSVD
from sklearn.preprocessing import normalize


def patch_key(value):
    return tuple(map(int, str(value).split(".")))


def dcg(recommended, relevant, k):
    return sum(1 / math.log2(rank + 2) for rank, item in enumerate(recommended[:k]) if item in relevant)


def evaluate(recommendations, targets, k, catalog_size):
    recalls, ndcgs, hits, recommended_items = [], [], [], set()
    for user, relevant in targets.items():
        recs = recommendations.get(user, [])[:k]
        recommended_items.update(recs)
        matched = len(set(recs) & relevant)
        recalls.append(matched / len(relevant))
        hits.append(float(matched > 0))
        ideal = sum(1 / math.log2(i + 2) for i in range(min(k, len(relevant))))
        ndcgs.append(dcg(recs, relevant, k) / ideal)
    return {
        f"recall@{k}": float(np.mean(recalls)), f"hit_rate@{k}": float(np.mean(hits)),
        f"ndcg@{k}": float(np.mean(ndcgs)),
        f"catalog_coverage@{k}": len(recommended_items) / catalog_size,
        "evaluated_users": len(targets),
    }


def top_unseen(ranking, seen, k):
    return [item for item in ranking if item not in seen][:k]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/baselines"))
    parser.add_argument("--top-k", type=int, default=5)
    parser.add_argument("--min-train-games", type=int, default=5)
    parser.add_argument("--factors", type=int, default=32)
    args = parser.parse_args()
    frame = pd.read_csv(args.input, dtype={"patch": str})
    frame["game_start_ts"] = pd.to_datetime(frame.game_start_ts, format="mixed", utc=True)
    patches = sorted(frame.patch.dropna().unique(), key=patch_key)
    test_patch = patches[-1]
    train = frame[frame.patch != test_patch].copy()
    test = frame[frame.patch == test_patch].sort_values("game_start_ts").copy()

    train_counts = train.groupby("puuid").size()
    eligible = set(train_counts[train_counts >= args.min_train_games].index)
    test = test[test.puuid.isin(eligible)]
    # The first future choice is the strict next-item target.
    first_test = test.groupby("puuid", sort=False).first().reset_index()
    targets = {row.puuid: {int(row.champion_id)} for row in first_test.itertuples()}
    users = sorted(eligible & set(targets))
    champions = sorted(train.champion_id.unique())
    user_to_idx = {value: idx for idx, value in enumerate(users)}
    item_to_idx = {int(value): idx for idx, value in enumerate(champions)}
    idx_to_item = {idx: item for item, idx in item_to_idx.items()}

    relevant_train = train[train.puuid.isin(users) & train.champion_id.isin(champions)].copy()
    age_days = (train.game_start_ts.max() - relevant_train.game_start_ts).dt.total_seconds() / 86400
    relevant_train["confidence"] = np.exp(-np.log(2) * age_days / 30)
    grouped = relevant_train.groupby(["puuid", "champion_id"], as_index=False).confidence.sum()
    rows = grouped.puuid.map(user_to_idx).to_numpy()
    cols = grouped.champion_id.map(item_to_idx).to_numpy()
    matrix = csr_matrix((grouped.confidence.to_numpy(), (rows, cols)), shape=(len(users), len(champions)))
    seen = {
        user: set(group.astype(int))
        for user, group in train[train.puuid.isin(users)].groupby("puuid").champion_id
    }

    popularity = train.groupby("champion_id").size().sort_values(ascending=False).index.astype(int).tolist()
    popularity_recs = {u: popularity[:args.top_k] for u in users}
    popularity_unseen_recs = {u: top_unseen(popularity, seen[u], args.top_k) for u in users}
    history_recs = {
        user: group.sort_values("confidence", ascending=False).champion_id.astype(int).tolist()[:args.top_k]
        for user, group in grouped.groupby("puuid", sort=False)
    }

    all_recommendations = {"popularity": popularity_recs, "history_frequency": history_recs}
    unseen_recommendations = {"popularity": popularity_unseen_recs, "history_frequency": history_recs}

    item_vectors = normalize(matrix.T, norm="l2", axis=1)
    item_similarity = (item_vectors @ item_vectors.T).toarray()
    np.fill_diagonal(item_similarity, 0)
    item_knn_scores = matrix @ item_similarity

    # Lightweight implicit matrix-factorization baseline. This is SVD rather
    # than confidence-weighted ALS because implicit has no Python 3.12 wheel
    # in the current environment. The evaluation contract remains identical.
    components = min(args.factors, min(matrix.shape) - 1)
    svd = TruncatedSVD(n_components=components, random_state=421)
    user_factors = svd.fit_transform(matrix)
    svd_scores = user_factors @ svd.components_

    for name, scores in {"item_knn": item_knn_scores, "svd": svd_scores}.items():
        ranked = np.argsort(-np.asarray(scores), axis=1)
        all_recommendations[name] = {}
        unseen_recommendations[name] = {}
        for user in users:
            row = ranked[user_to_idx[user]]
            all_recommendations[name][user] = [idx_to_item[int(index)] for index in row[:args.top_k]]
            unseen_recommendations[name][user] = [
                idx_to_item[int(index)] for index in row if idx_to_item[int(index)] not in seen[user]
            ][:args.top_k]

    discovery_targets = {
        user: relevant for user, relevant in targets.items() if not (relevant & seen[user])
    }

    report = {
        "train_patches": patches[:-1], "test_patch": test_patch,
        "train_rows": len(train), "test_rows": len(test),
        "catalog_size": len(champions), "min_train_games": args.min_train_games,
        "next_choice": {
            name: evaluate(recs, targets, args.top_k, len(champions))
            for name, recs in all_recommendations.items()
        },
        "new_champion_discovery": {
            name: evaluate(recs, discovery_targets, args.top_k, len(champions))
            for name, recs in unseen_recommendations.items()
        },
        "discovery_users": len(discovery_targets),
    }
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / "metrics.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    rows_out = []
    for model_name, recs_by_user in all_recommendations.items():
        for user, recs in recs_by_user.items():
            for rank, champion_id in enumerate(recs, 1):
                rows_out.append({"model": model_name, "puuid": user, "champion_id": champion_id, "rank": rank})
    pd.DataFrame(rows_out).to_csv(args.output_dir / "recommendations.csv", index=False)
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
