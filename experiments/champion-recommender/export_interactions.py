#!/usr/bin/env python3
"""Export point-in-time player/champion interactions from GOGG PostgreSQL."""

import argparse
import os
from pathlib import Path

import pandas as pd
import psycopg


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/jungle_interactions.csv"))
    parser.add_argument("--position", default="JUNGLE")
    parser.add_argument("--queue-id", type=int, default=420)
    parser.add_argument("--min-game-start", default="1970-01-01")
    parser.add_argument("--max-game-start", default="2100-01-01")
    args = parser.parse_args()
    dsn = os.environ.get("DATABASE_URL")
    if not dsn:
        raise SystemExit("DATABASE_URL is required")
    sql = Path(__file__).with_name("extract_interactions.sql").read_text(encoding="utf-8")
    params = {
        "position": args.position.upper(), "queue_id": args.queue_id,
        "min_game_start": args.min_game_start, "max_game_start": args.max_game_start,
    }
    with psycopg.connect(dsn) as connection, connection.cursor() as cursor:
        cursor.execute(sql, params)
        frame = pd.DataFrame(cursor.fetchall(), columns=[column.name for column in cursor.description])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    frame.to_csv(args.output, index=False)
    print(f"exported {len(frame)} interactions for {frame.puuid.nunique()} players")


if __name__ == "__main__":
    main()
