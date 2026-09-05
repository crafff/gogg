#!/usr/bin/env python3
"""Export one jungle participant per match for the role-wide experiment."""

import argparse
import os
from pathlib import Path

import pandas as pd
import psycopg


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/all_junglers_15m.csv"))
    parser.add_argument("--queue-id", type=int, default=420)
    parser.add_argument("--min-game-start", default="1970-01-01")
    parser.add_argument("--max-game-start", default="2100-01-01")
    args = parser.parse_args()
    database_url = os.environ.get("DATABASE_URL")
    if not database_url:
        raise SystemExit("DATABASE_URL is required")
    sql = Path(__file__).with_name("extract_all_junglers.sql").read_text(encoding="utf-8")
    params = {
        "queue_id": args.queue_id,
        "min_game_start": args.min_game_start,
        "max_game_start": args.max_game_start,
    }
    with psycopg.connect(database_url) as connection, connection.cursor() as cursor:
        cursor.execute(sql, params)
        frame = pd.DataFrame(cursor.fetchall(), columns=[column.name for column in cursor.description])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    frame.to_csv(args.output, index=False)
    print(f"exported {len(frame)} rows, {frame.champion_id.nunique()} champions to {args.output}")


if __name__ == "__main__":
    main()
