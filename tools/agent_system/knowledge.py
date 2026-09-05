"""Validate authoritative facts and search them through a transient memory index.

Hash verification proves source integrity, not that a claim is true. No source
contents, external URLs, repository trees, or archive trees are indexed.
"""

from __future__ import annotations

from collections import Counter
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import datetime, timezone
import errno
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import sqlite3
import stat
import subprocess
from typing import Any, Iterator
from urllib.parse import urlsplit


FACTS_PATH = "engineering/knowledge/facts.json"
STATUSES = {"candidate", "verified", "stale", "superseded", "rejected"}
SCOPES = {"current", "legacy"}
INACTIVE_STATUSES = {"stale", "superseded", "rejected"}
_ID = re.compile(r"[A-Za-z][A-Za-z0-9._:-]{0,127}\Z")
_HASH = re.compile(r"[0-9a-f]{64}\Z")
_TIMESTAMP = re.compile(
    r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?(?:Z|[+-]\d{2}:\d{2})\Z"
)
_HAN = re.compile(r"[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\U00020000-\U000323af]")
_PRIVATE_NAMES = {
    "private", "secrets", "credentials", "id_rsa", "id_ed25519",
    "data", "artifacts", "logs", "tmp", "temp", "backups", "backup",
}
_PRIVATE_SUFFIXES = {".pem", ".key", ".p12", ".pfx"}
_CONFIG_SUFFIXES = {".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".properties"}
_CONFIG_DIRECTORIES = {"config", "configs", "configuration"}


class KnowledgeError(Exception):
    """A user-actionable validation, safety, or search failure."""


@dataclass(frozen=True)
class Fact:
    id: str
    status: str
    scope: str
    claim: str
    sources: tuple[dict[str, str], ...]
    verified_at: datetime | None
    valid_until: datetime | None

    def result(self) -> dict[str, Any]:
        result: dict[str, Any] = {
            "id": self.id,
            "status": self.status,
            "scope": self.scope,
            "claim": self.claim,
            "sources": [dict(source) for source in self.sources],
            "verified_at": _iso(self.verified_at) if self.verified_at else None,
        }
        if self.valid_until is not None:
            result["valid_until"] = _iso(self.valid_until)
        return result


@dataclass(frozen=True)
class Snapshot:
    root: Path
    facts: tuple[Fact, ...]
    fingerprint: str
    warnings: tuple[str, ...]


def _iso(value: datetime) -> str:
    return value.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def _now(value: datetime | None) -> datetime:
    value = value or datetime.now(timezone.utc)
    if value.tzinfo is None or value.utcoffset() is None:
        raise KnowledgeError("the evaluation time must have an explicit timezone")
    return value.astimezone(timezone.utc)


def _timestamp(value: Any, label: str) -> datetime:
    if not isinstance(value, str) or not _TIMESTAMP.fullmatch(value):
        raise KnowledgeError(f"{label}: expected an ISO 8601 timestamp with timezone")
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)
    except (ValueError, OverflowError) as exc:
        raise KnowledgeError(f"{label}: invalid timestamp") from exc


def _root(root: str | Path) -> Path:
    try:
        path = Path(root).resolve(strict=True)
    except (OSError, RuntimeError) as exc:
        raise KnowledgeError(f"cannot resolve repository root: {exc}") from exc
    if not path.is_dir():
        raise KnowledgeError(f"repository root is not a directory: {path}")
    return path


def _relative_path(value: Any, label: str) -> tuple[str, ...]:
    if not isinstance(value, str) or not value or "\\" in value or "\x00" in value:
        raise KnowledgeError(f"{label}: expected a nonempty relative POSIX path")
    path = PurePosixPath(value)
    parts = value.split("/")
    if path.is_absolute() or any(part in {"", ".", ".."} for part in parts):
        raise KnowledgeError(f"{label}: absolute paths, traversal, and noncanonical paths are forbidden")
    for part in parts:
        lowered = part.casefold()
        if (
            part.startswith(".")
            or lowered in _PRIVATE_NAMES
            or lowered.startswith(("secrets.", "credentials."))
            or PurePosixPath(lowered).suffix in _PRIVATE_SUFFIXES
        ):
            raise KnowledgeError(f"{label}: private or hidden source paths are forbidden")
    filename = parts[-1].casefold()
    suffix = PurePosixPath(filename).suffix
    config_file = filename.startswith(("config.", "configuration.", "settings.")) or any(
        component.casefold() in _CONFIG_DIRECTORIES for component in parts[:-1]
    )
    if config_file and suffix in _CONFIG_SUFFIXES and not filename.endswith(".example" + suffix):
        raise KnowledgeError(f"{label}: private configuration sources are forbidden; use a public *.example template")
    return tuple(parts)


def _reject_gitignored_sources(root: Path, paths: list[str]) -> None:
    """Apply Git's actual ignore rules without reading referenced file contents.

    --no-index is intentional: a tracked credential does not become an allowed
    source just because it was accidentally added to Git. A non-Git repository
    still uses the conservative naming rules in _relative_path.
    """
    # Do not let inherited GIT_DIR/GIT_WORK_TREE/config injection select another
    # repository or disable this repository's privacy rules. Normal on-disk Git
    # configuration (including user excludes) remains in effect.
    environment = {name: value for name, value in os.environ.items() if not name.startswith("GIT_")}
    environment["GIT_OPTIONAL_LOCKS"] = "0"
    environment["LC_ALL"] = "C"
    try:
        discovery = subprocess.run(
            ["git", "-C", str(root), "rev-parse", "--is-inside-work-tree"],
            capture_output=True, timeout=10, env=environment,
        )
        if discovery.returncode != 0:
            # Some sandboxes expose empty protected /tmp/.git placeholders.
            # Existence in an ancestor is not proof of a repository. A direct
            # declaration at --root, however, must not silently degrade when
            # that repository is broken or inaccessible.
            if discovery.stderr.startswith(b"fatal: not a git repository") and not os.path.lexists(root / ".git"):
                return
            raise KnowledgeError("cannot safely validate Git ignore rules; repair the repository/configuration before reading sources")
        if discovery.stdout.strip() != b"true":
            raise KnowledgeError("cannot safely validate Git ignore rules without an active Git worktree")
        result = subprocess.run(
            ["git", "-C", str(root), "check-ignore", "--no-index", "--stdin", "-z"],
            input="\x00".join(paths).encode("utf-8") + b"\x00",
            capture_output=True, timeout=10, env=environment,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise KnowledgeError("cannot safely validate Git ignore rules; Git is required for sources in a Git repository") from exc
    if result.returncode not in {0, 1}:
        raise KnowledgeError("cannot safely validate Git ignore rules; repair the repository/configuration before reading sources")
    ignored = [path.decode("utf-8") for path in result.stdout.split(b"\x00") if path]
    if ignored:
        raise KnowledgeError(f"Git-ignored source paths are forbidden: {', '.join(repr(path) for path in ignored)}")


@contextmanager
def _safe_file(root: Path, parts: tuple[str, ...]) -> Iterator[Any]:
    """Open relative to directory descriptors; never follow a source symlink.

    O_NOFOLLOW on every component also avoids a check/open symlink race. Fail
    closed on platforms without the POSIX primitives instead of weakening this.
    """
    if not hasattr(os, "O_NOFOLLOW") or os.open not in os.supports_dir_fd:
        raise KnowledgeError("safe source reads require POSIX O_NOFOLLOW and dir_fd support")
    descriptors: list[int] = []
    name = "/".join(parts)
    try:
        directory_flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
        descriptors.append(os.open(root, directory_flags))
        for component in parts[:-1]:
            descriptors.append(os.open(component, directory_flags, dir_fd=descriptors[-1]))
        descriptor = os.open(
            parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK,
            dir_fd=descriptors[-1],
        )
        descriptors.append(descriptor)
        if not stat.S_ISREG(os.fstat(descriptor).st_mode):
            raise KnowledgeError(f"source {name!r} is not a regular file")
        with os.fdopen(os.dup(descriptor), "rb") as source:
            yield source
    except FileNotFoundError:
        raise
    except OSError as exc:
        if exc.errno in {errno.ELOOP, errno.ENOTDIR}:
            raise KnowledgeError(f"unsafe source {name!r}: symlinks and non-directory components are forbidden") from exc
        raise KnowledgeError(f"cannot safely read source {name!r}: {exc.strerror}") from exc
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


def _source_hash(root: Path, parts: tuple[str, ...]) -> str:
    digest = hashlib.sha256()
    with _safe_file(root, parts) as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise KnowledgeError(f"malformed facts JSON: duplicate object key {key!r}")
        result[key] = value
    return result


def _invalid_constant(value: str) -> None:
    raise KnowledgeError(f"malformed facts JSON: non-JSON constant {value!r}")


def _validate_unicode(document: Any) -> None:
    pending = [document]
    while pending:
        value = pending.pop()
        if isinstance(value, dict):
            pending.extend(value.keys())
            pending.extend(value.values())
        elif isinstance(value, list):
            pending.extend(value)
        elif isinstance(value, str) and any(0xD800 <= ord(character) <= 0xDFFF for character in value):
            raise KnowledgeError("malformed facts JSON: unpaired Unicode surrogate")


def _keys(value: Any, required: set[str], optional: set[str], label: str) -> None:
    if not isinstance(value, dict):
        raise KnowledgeError(f"{label}: expected an object")
    missing = required - value.keys()
    extra = value.keys() - required - optional
    if missing or extra:
        raise KnowledgeError(f"{label}: missing fields {sorted(missing)}; unknown fields {sorted(extra)}")


def _fact(value: Any, position: int) -> Fact:
    label = f"facts[{position}]"
    _keys(value, {"id", "status", "scope", "claim", "sources", "verified_at"}, {"valid_until"}, label)
    fact_id = value["id"]
    if not isinstance(fact_id, str) or not _ID.fullmatch(fact_id):
        raise KnowledgeError(f"{label}.id: expected a stable 1-128 character identifier")
    label = f"fact {fact_id!r}"
    status = value["status"]
    scope = value["scope"]
    if not isinstance(status, str) or status not in STATUSES:
        raise KnowledgeError(f"{label}: invalid status")
    if not isinstance(scope, str) or scope not in SCOPES:
        raise KnowledgeError(f"{label}: scope must be 'current' or 'legacy'")
    if not isinstance(value["claim"], str) or not value["claim"].strip() or "\x00" in value["claim"]:
        raise KnowledgeError(f"{label}: claim must be a nonempty string without NUL")
    verified_at = None if value["verified_at"] is None else _timestamp(value["verified_at"], f"{label}.verified_at")
    valid_until = _timestamp(value["valid_until"], f"{label}.valid_until") if "valid_until" in value else None
    if status == "verified" and verified_at is None:
        raise KnowledgeError(f"{label}: verified facts require verified_at")
    if verified_at and valid_until and valid_until <= verified_at:
        raise KnowledgeError(f"{label}: valid_until must be later than verified_at")
    if not isinstance(value["sources"], list):
        raise KnowledgeError(f"{label}: sources must be an array")
    sources: list[dict[str, str]] = []
    source_ids: set[str] = set()
    local_count = 0
    legacy_count = 0
    for position, source in enumerate(value["sources"]):
        source_label = f"{label}.sources[{position}]"
        if not isinstance(source, dict):
            raise KnowledgeError(f"{source_label}: expected an object")
        if "path" in source:
            _keys(source, {"path", "sha256"}, set(), source_label)
            parts = _relative_path(source["path"], source_label)
            if not isinstance(source["sha256"], str) or not _HASH.fullmatch(source["sha256"]):
                raise KnowledgeError(f"{source_label}: sha256 must be 64 lowercase hexadecimal characters")
            local_count += 1
            if parts[0] == "legecy":
                legacy_count += 1
                if scope != "legacy":
                    raise KnowledgeError(f"{label}: legecy/ evidence requires explicit legacy scope")
            source_id = "path:" + source["path"]
        elif "url" in source:
            _keys(source, {"url"}, set(), source_label)
            if not isinstance(source["url"], str) or any(character.isspace() or ord(character) < 32 or ord(character) == 127 for character in source["url"]):
                raise KnowledgeError(f"{source_label}: expected a public HTTP(S) URL")
            try:
                url = urlsplit(source["url"])
                valid = url.scheme in {"http", "https"} and url.hostname and not url.username and not url.password
                _ = url.port
            except ValueError:
                valid = False
            if not valid:
                raise KnowledgeError(f"{source_label}: expected a public HTTP(S) URL without embedded credentials")
            source_id = "url:" + source["url"]
        else:
            raise KnowledgeError(f"{source_label}: expected path+sha256 or url")
        if source_id in source_ids:
            raise KnowledgeError(f"{label}: duplicate source reference")
        source_ids.add(source_id)
        sources.append(dict(source))
    if status == "verified" and local_count == 0:
        raise KnowledgeError(f"{label}: verified facts require local hashed evidence; URLs are not automatically verified")
    if scope == "legacy" and legacy_count == 0:
        raise KnowledgeError(f"{label}: legacy scope requires explicit legecy/ source evidence")
    return Fact(fact_id, status, scope, value["claim"], tuple(sources), verified_at, valid_until)


def load_snapshot(root: str | Path, *, now: datetime | None = None) -> Snapshot:
    """Validate all explicit facts and hash only their explicit local sources."""
    repository = _root(root)
    instant = _now(now)
    try:
        with _safe_file(repository, tuple(FACTS_PATH.split("/"))) as source:
            raw = source.read()
    except FileNotFoundError as exc:
        raise KnowledgeError(f"missing authoritative facts file: {repository / FACTS_PATH}") from exc
    try:
        document = json.loads(raw.decode("utf-8"), object_pairs_hook=_unique_object, parse_constant=_invalid_constant)
    except (UnicodeError, ValueError, RecursionError) as exc:
        raise KnowledgeError(f"malformed {FACTS_PATH}: {exc}") from exc
    _validate_unicode(document)
    _keys(document, {"schema_version", "facts"}, set(), "facts document")
    if type(document["schema_version"]) is not int or document["schema_version"] != 1:
        raise KnowledgeError("unsupported facts schema_version; expected 1")
    if not isinstance(document["facts"], list):
        raise KnowledgeError("facts document: facts must be an array")
    facts = tuple(_fact(value, position) for position, value in enumerate(document["facts"]))
    counts = Counter(fact.id for fact in facts)
    duplicates = sorted(fact_id for fact_id, count in counts.items() if count > 1)
    if duplicates:
        raise KnowledgeError(f"duplicate fact IDs: {', '.join(duplicates)}")
    _reject_gitignored_sources(repository, sorted({
        FACTS_PATH,
        *(source["path"] for fact in facts for source in fact.sources if "path" in source),
    }))
    warnings: list[str] = []
    source_hashes: dict[str, str | None] = {}
    for fact in facts:
        expired = fact.valid_until is not None and fact.valid_until <= instant
        if expired:
            warnings.append(f"{fact.id}: expired; excluded from search")
        for source in fact.sources:
            if "url" in source:
                warnings.append(f"{fact.id}: external URL is a reference only; not fetched or automatically verified")
                continue
            path = source["path"]
            if path not in source_hashes:
                try:
                    source_hashes[path] = _source_hash(repository, tuple(path.split("/")))
                except FileNotFoundError:
                    source_hashes[path] = None
            actual = source_hashes[path]
            if actual != source["sha256"]:
                reason = "missing source" if actual is None else "source SHA-256 changed"
                problem = f"{fact.id}: {reason}: {path}"
                if fact.status in INACTIVE_STATUSES or (fact.status == "verified" and expired):
                    warnings.append(problem + "; inactive or expired fact retained, excluded from search")
                else:
                    raise KnowledgeError(problem + "; re-evaluate evidence and update the fact before searching")
    binding = {
        "root": str(repository),
        "facts_sha256": hashlib.sha256(raw).hexdigest(),
        "actual_source_sha256": source_hashes,
    }
    fingerprint = hashlib.sha256(json.dumps(binding, sort_keys=True, separators=(",", ":")).encode("utf-8")).hexdigest()
    return Snapshot(repository, facts, fingerprint, tuple(warnings))


def check(root: str | Path, *, now: datetime | None = None) -> dict[str, Any]:
    snapshot = load_snapshot(root, now=now)
    return {
        "ok": True,
        "root": str(snapshot.root),
        "facts": len(snapshot.facts),
        "statuses": dict(sorted(Counter(fact.status for fact in snapshot.facts).items())),
        "scopes": dict(sorted(Counter(fact.scope for fact in snapshot.facts).items())),
        "fingerprint": snapshot.fingerprint,
        "warnings": list(snapshot.warnings),
        "verification": "schema and local source integrity only; claim truth and external URLs are not automatically verified",
    }


def _literal_query(query: str) -> tuple[str, tuple[str, ...]]:
    if (
        not isinstance(query, str) or not query.strip() or len(query) > 4096 or "\x00" in query
        or any(0xD800 <= ord(character) <= 0xDFFF for character in query)
    ):
        raise KnowledgeError("search query must contain 1-4096 valid Unicode characters without NUL")
    # Every token is quoted. FTS operators, column selectors, quotes and SQL
    # syntax from the caller are data, never executable query expressions.
    terms = re.findall(r"[^\W_]+", _script_boundaries(query), flags=re.UNICODE)
    if not terms:
        raise KnowledgeError("search query must contain at least one letter or number")
    if len(terms) > 128:
        raise KnowledgeError("search query has too many terms; maximum is 128")
    # unicode61 treats a continuous Han phrase as one token. For this small
    # explicit-facts corpus, Han-containing tokens use bound, literal substrings
    # in claim text instead. This is not Chinese segmentation or semantic search.
    han_terms = tuple(term for term in terms if _HAN.search(term))
    fts_terms = [term for term in terms if not _HAN.search(term)]
    return " AND ".join('"' + term.replace('"', '""') + '"' for term in fts_terms), han_terms


def _script_boundaries(text: str) -> str:
    """Separate adjacent Han/non-Han letters consistently in queries and claims."""
    result: list[str] = []
    previous = ""
    previous_han = False
    for character in text:
        current_han = bool(_HAN.fullmatch(character))
        if previous and current_han != previous_han and character.isalnum() and previous.isalnum():
            result.append(" ")
        result.append(character)
        previous, previous_han = character, current_han
    return "".join(result)


def _memory_index(snapshot: Snapshot, instant: datetime) -> sqlite3.Connection:
    """Build query data only from verified, unexpired authoritative facts."""
    memory = sqlite3.connect(":memory:")
    try:
        memory.execute("PRAGMA temp_store=MEMORY")
        memory.execute(
            "CREATE VIRTUAL TABLE facts_fts USING fts5("
            "fact_id UNINDEXED, scope UNINDEXED, claim UNINDEXED, "
            "search_text, tokenize='unicode61')"
        )
        memory.executemany("INSERT INTO facts_fts VALUES (?, ?, ?, ?)", [
            (fact.id, fact.scope, fact.claim, _script_boundaries(fact.claim))
            for fact in snapshot.facts
            if fact.status == "verified" and (fact.valid_until is None or fact.valid_until > instant)
        ])
        return memory
    except BaseException:
        memory.close()
        raise


def search(
    root: str | Path, query: str, *, scope: str = "current",
    limit: int = 20, now: datetime | None = None,
) -> dict[str, Any]:
    if scope not in SCOPES | {"all"}:
        raise KnowledgeError("search scope must be current, legacy, or all")
    if type(limit) is not int or not 1 <= limit <= 100:
        raise KnowledgeError("search limit must be an integer between 1 and 100")
    expression, han_terms = _literal_query(query)
    instant = _now(now)
    snapshot = load_snapshot(root, now=instant)
    memory: sqlite3.Connection | None = None
    try:
        memory = _memory_index(snapshot, instant)
        clauses = ["(? = 'all' OR scope = ?)"]
        parameters: list[Any] = [scope, scope]
        if expression:
            clauses.append("facts_fts MATCH ?")
            parameters.append(expression)
        for term in han_terms:
            clauses.append("instr(claim, ?) > 0")
            parameters.append(term)
        parameters.append(limit)
        ordering = "bm25(facts_fts), fact_id" if expression else "fact_id"
        rows = memory.execute(
            "SELECT fact_id FROM facts_fts WHERE "
            + " AND ".join(clauses) + " ORDER BY " + ordering + " LIMIT ?",
            parameters,
        ).fetchall()
        by_id = {fact.id: fact for fact in snapshot.facts}
        results = [by_id[fact_id].result() for (fact_id,) in rows]
    except sqlite3.Error as exc:
        raise KnowledgeError(f"cannot search using SQLite FTS5: {exc}") from exc
    finally:
        if memory is not None:
            memory.close()
    # Recheck both the authoritative claims and their source bytes after the
    # query. Never return a result already known to have changed during search.
    if load_snapshot(snapshot.root, now=instant).fingerprint != snapshot.fingerprint:
        raise KnowledgeError("knowledge changed during search; retry after stabilizing the sources")
    return {
        "root": str(snapshot.root), "scope": scope, "fingerprint": snapshot.fingerprint,
        "results": results, "warnings": list(snapshot.warnings),
    }
