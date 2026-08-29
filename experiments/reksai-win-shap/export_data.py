#!/usr/bin/env python3
"""Export the leakage-safe Rek'Sai modelling cohort from PostgreSQL."""

import argparse
import os
from pathlib import Path

import pandas as pd
import psycopg


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/reksai_15m.csv"))
    parser.add_argument("--queue-id", type=int, default=420)
    parser.add_argument("--cutoff-minute", type=int, default=15, choices=[15])
    parser.add_argument("--min-game-start", default="1970-01-01")
    parser.add_argument("--max-game-start", default="2100-01-01")
    args = parser.parse_args()

    database_url = os.environ.get("DATABASE_URL")
    if not database_url:
        raise SystemExit("DATABASE_URL is required")

    sql_path = Path(__file__).with_name("extract.sql")
    # psycopg does not understand psql meta commands, so use the query body only.
    sql = sql_path.read_text(encoding="utf-8")
    sql = sql[sql.index("WITH candidates AS"):]
    sql = sql.replace(":'min_game_start'::timestamptz", "%(min_game_start)s::timestamptz")
    sql = sql.replace(":'max_game_start'::timestamptz", "%(max_game_start)s::timestamptz")
    sql = sql.replace(":queue_id", "%(queue_id)s")
    sql = sql.replace(":cutoff_minute", "%(cutoff_minute)s")
    params = vars(args) | {"queue_id": args.queue_id}
    params.pop("output")

    with psycopg.connect(database_url) as connection, connection.cursor() as cursor:
        cursor.execute(sql, params)
        frame = pd.DataFrame(cursor.fetchall(), columns=[column.name for column in cursor.description])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    frame.to_csv(args.output, index=False)
    print(f"exported {len(frame)} rows to {args.output}")


if __name__ == "__main__":
    main()
