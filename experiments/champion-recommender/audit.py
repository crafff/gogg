#!/usr/bin/env python3
"""Create the Stage-0 coverage and cold-start audit."""

import argparse
import json
from pathlib import Path

import matplotlib.pyplot as plt
import pandas as pd


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, default=Path("artifacts/audit"))
    args = parser.parse_args()
    frame = pd.read_csv(args.input, dtype={"patch": str})
    frame["game_start_ts"] = pd.to_datetime(frame.game_start_ts, format="mixed", utc=True)
    args.output_dir.mkdir(parents=True, exist_ok=True)
    games = frame.groupby("puuid").size()
    champions = frame.groupby("puuid").champion_id.nunique()
    patches = sorted(frame.patch.dropna().unique(), key=lambda value: tuple(map(int, value.split("."))))
    latest = patches[-1]
    earlier = frame[frame.patch != latest]
    latest_frame = frame[frame.patch == latest]
    earlier_players = set(earlier.puuid)
    latest_players = set(latest_frame.puuid)
    report = {
        "rows": len(frame), "matches": int(frame.match_id.nunique()),
        "players": int(frame.puuid.nunique()), "champions": int(frame.champion_id.nunique()),
        "date_min": str(frame.game_start_ts.min()), "date_max": str(frame.game_start_ts.max()),
        "patches": {str(k): int(v) for k, v in frame.patch.value_counts().sort_index().items()},
        "latest_patch": latest, "latest_rows": len(latest_frame),
        "latest_known_player_rate": len(latest_players & earlier_players) / max(1, len(latest_players)),
        "players_games_ge_5": int((games >= 5).sum()),
        "players_games_ge_20": int((games >= 20).sum()),
        "players_games_ge_50": int((games >= 50).sum()),
        "median_games_per_player": float(games.median()),
        "median_champions_per_player": float(champions.median()),
    }
    (args.output_dir / "audit.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    pd.DataFrame({"games": games, "champions": champions}).to_csv(args.output_dir / "player_coverage.csv")
    fig, axes = plt.subplots(1, 2, figsize=(11, 4))
    games.clip(upper=100).hist(bins=50, ax=axes[0])
    axes[0].set(title="Games per player (clipped at 100)", xlabel="games")
    champions.hist(bins=40, ax=axes[1])
    axes[1].set(title="Distinct jungle champions per player", xlabel="champions")
    fig.tight_layout()
    fig.savefig(args.output_dir / "coverage.png", dpi=180)
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
