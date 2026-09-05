"""Command line interface; all successful outputs are JSON."""

import argparse
import json
import sys

from .knowledge import KnowledgeError, check, search


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Offline, explicit-source knowledge integrity and SQLite FTS5 tools.")
    parser.add_argument("--root", default=".", help="repository root (default: current directory)")
    commands = parser.add_subparsers(dest="command", required=True)
    for name, description in (
        ("check", "validate facts, scopes, and local source integrity; no writes"),
        ("search", "validate sources and search verified, unexpired facts without disk writes"),
    ):
        command = commands.add_parser(name, help=description)
        command.add_argument("--root", default=argparse.SUPPRESS, help="repository root")
        if name == "search":
            command.add_argument("query", help="literal search terms, not FTS or SQL syntax")
            command.add_argument("--scope", choices=("current", "legacy", "all"), default="current")
            command.add_argument("--limit", type=int, default=20)
    return parser


def main(argv: list[str] | None = None) -> int:
    arguments = _parser().parse_args(argv)
    try:
        if arguments.command == "check":
            result = check(arguments.root)
        else:
            result = search(arguments.root, arguments.query, scope=arguments.scope, limit=arguments.limit)
    except (KnowledgeError, OSError) as exc:
        print(f"knowledge error: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
