from __future__ import annotations

import copy
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from tools.agent_system import knowledge
from tools.agent_system.knowledge import (
    FACTS_PATH,
    KnowledgeError,
    check,
    search,
)


NOW = datetime(2026, 1, 10, 12, tzinfo=timezone.utc)


def timestamp(value: datetime) -> str:
    return value.isoformat().replace("+00:00", "Z")


class Repository:
    def __init__(self, root: Path):
        self.root = root
        self.root.mkdir(parents=True)
        self.facts: list[dict] = []
        self.save()

    def write(self, path: str, content: str | bytes) -> Path:
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content.encode("utf-8") if isinstance(content, str) else content)
        return target

    def fact(
        self, fact_id: str = "FACT-CURRENT", claim: str = "Riot retries are bounded",
        *, path: str = "src/collector.txt", status: str = "verified", scope: str = "current",
        valid_until: datetime | None = None,
    ) -> dict:
        source = self.write(path, "Explicit implementation evidence for " + fact_id)
        fact = {
            "id": fact_id, "status": status, "scope": scope, "claim": claim,
            "sources": [{"path": path, "sha256": hashlib.sha256(source.read_bytes()).hexdigest()}],
            "verified_at": timestamp(NOW - timedelta(days=5)) if status == "verified" else None,
        }
        if valid_until is not None:
            fact["valid_until"] = timestamp(valid_until)
        self.facts.append(fact)
        self.save()
        return fact

    def save(self) -> None:
        self.write(FACTS_PATH, json.dumps({"schema_version": 1, "facts": self.facts}, ensure_ascii=False, indent=2))


class KnowledgeTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory(prefix="agent-system-test-")
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.repo = Repository(self.base / "repository")
        self.root = self.repo.root

    def search(self, query: str, **kwargs) -> dict:
        return search(self.root, query, now=kwargs.pop("now", NOW), **kwargs)

    def git(self, *arguments: str) -> None:
        result = subprocess.run(
            ["git", "-c", "init.templateDir=", "-C", str(self.root), *arguments],
            capture_output=True, text=True, timeout=15,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_check_is_read_only_and_reports_integrity_boundary(self) -> None:
        self.repo.fact()
        result = check(self.root, now=NOW)
        self.assertTrue(result["ok"])
        self.assertEqual(result["statuses"], {"verified": 1})
        self.assertEqual(result["scopes"], {"current": 1})
        self.assertFalse((self.root / ".local").exists())

    def test_real_query_returns_authoritative_claim_scope_and_references(self) -> None:
        expected = self.repo.fact()
        self.repo.fact("FACT-ASSETS", "Assets are cached locally", path="src/assets.txt")
        results = self.search("Riot retries")["results"]
        self.assertEqual(len(results), 1)
        for field in ("id", "claim", "scope", "sources", "verified_at"):
            self.assertEqual(results[0][field], expected[field])
        self.assertEqual(self.search("Assets")["results"][0]["id"], "FACT-ASSETS")
        self.assertEqual(self.search("no_such_token")["results"], [])

    def test_only_claims_are_indexed_not_source_contents(self) -> None:
        fact = self.repo.fact()
        self.repo.write("src/collector.txt", "private_source_body_keyword")
        fact["sources"][0]["sha256"] = hashlib.sha256(b"private_source_body_keyword").hexdigest()
        self.repo.save()
        self.assertEqual(self.search("private_source_body_keyword")["results"], [])
        self.assertEqual(len(self.search("Riot")["results"]), 1)

    def test_chinese_short_substring_and_mixed_queries_match_real_claims(self) -> None:
        self.repo.fact("FACT-CHINESE", "重建知识工具支持 Riot 证据校验", path="src/chinese.txt")
        self.repo.fact("FACT-OTHER", "重建知识工具仍在开发", path="src/other.txt")
        self.repo.fact("FACT-LATIN", "Riot retries are bounded", path="src/latin.txt")
        self.assertEqual({item["id"] for item in self.search("知识")["results"]}, {"FACT-CHINESE", "FACT-OTHER"})
        self.assertEqual([item["id"] for item in self.search("知识 Riot")["results"]], ["FACT-CHINESE"])
        self.assertEqual([item["id"] for item in self.search("知识 校验")["results"]], ["FACT-CHINESE"])
        self.assertEqual(self.search("知识' OR 1=1 --")["results"], [])
        self.assertEqual(self.search("知识 不存在")["results"], [])

    def test_adjacent_latin_and_chinese_scripts_share_query_boundaries(self) -> None:
        self.repo.fact("FACT-ADJACENT", "Riot重试不会无限循环", path="src/adjacent.txt")
        self.repo.fact("FACT-OTHER", "Riot身份校验", path="src/other.txt")
        self.assertEqual({item["id"] for item in self.search("Riot")["results"]}, {"FACT-ADJACENT", "FACT-OTHER"})
        for query in ("riot重试", "RIOT重试", "Riot 重试", "重试Riot"):
            with self.subTest(query=query):
                self.assertEqual([item["id"] for item in self.search(query)["results"]], ["FACT-ADJACENT"])
        self.assertEqual(self.search("riot 不存在")["results"], [])

    def test_status_and_legacy_scope_are_independent(self) -> None:
        self.repo.fact()
        self.repo.fact("FACT-CANDIDATE", "Riot candidate", path="src/proposal.txt", status="candidate")
        self.repo.fact("FACT-LEGACY", "Riot historical behavior", path="legecy/apps/api.txt", scope="legacy")
        self.repo.fact("FACT-LEGACY-CANDIDATE", "Riot historical hypothesis", path="legecy/proposal.txt", scope="legacy", status="candidate")
        self.assertEqual([item["id"] for item in self.search("Riot")["results"]], ["FACT-CURRENT"])
        self.assertEqual([item["id"] for item in self.search("Riot", scope="legacy")["results"]], ["FACT-LEGACY"])
        self.assertEqual({item["id"] for item in self.search("Riot", scope="all")["results"]}, {"FACT-CURRENT", "FACT-LEGACY"})

    def test_legacy_sources_cannot_be_current_claims(self) -> None:
        self.repo.fact(path="legecy/apps/api.txt")
        with self.assertRaisesRegex(KnowledgeError, "legacy scope"):
            check(self.root)

    def test_legacy_scope_requires_explicit_archive_evidence(self) -> None:
        self.repo.fact(scope="legacy")
        with self.assertRaisesRegex(KnowledgeError, "legecy/ source"):
            check(self.root)

    def test_expiration_is_evaluated_at_search_time_and_exact_boundary(self) -> None:
        self.repo.fact(valid_until=NOW + timedelta(hours=1))
        self.assertEqual(len(self.search("Riot")["results"]), 1)
        self.assertEqual(self.search("Riot", now=NOW + timedelta(hours=1))["results"], [])
        self.assertEqual(self.search("Riot", now=NOW + timedelta(days=1))["results"], [])

    def test_expired_results_do_not_consume_limit(self) -> None:
        self.repo.fact("FACT-EXPIRED", "Riot", path="src/expired.txt", valid_until=NOW - timedelta(hours=1))
        self.repo.fact("FACT-VALID", "Riot has a valid current fact", path="src/valid.txt")
        self.assertEqual(self.search("Riot", limit=1)["results"][0]["id"], "FACT-VALID")
        self.assertTrue(any("expired" in warning for warning in check(self.root, now=NOW)["warnings"]))

    def test_duplicate_ids_are_rejected(self) -> None:
        fact = self.repo.fact()
        self.repo.facts.append(copy.deepcopy(fact))
        self.repo.save()
        with self.assertRaisesRegex(KnowledgeError, "duplicate fact IDs"):
            check(self.root)

    def test_all_inactive_statuses_are_excluded_and_drift_is_reported(self) -> None:
        for status in ("stale", "superseded", "rejected"):
            self.repo.fact("FACT-" + status, "Riot inactive", path="src/" + status, status=status)
            self.repo.write("src/" + status, "changed")
        result = check(self.root)
        self.assertEqual(len(result["warnings"]), 3)
        self.assertEqual(self.search("Riot", scope="all")["results"], [])

    def test_candidate_hash_drift_is_not_accepted_as_verified_evidence(self) -> None:
        self.repo.fact(status="candidate")
        self.repo.write("src/collector.txt", "changed")
        with self.assertRaisesRegex(KnowledgeError, "SHA-256 changed"):
            check(self.root)

    def test_unreferenced_private_and_archive_files_are_not_scanned(self) -> None:
        self.repo.fact()
        initial = self.search("Riot")
        self.repo.write(".local/private/secret.txt", b"\xff\x00riot secret")
        self.repo.write("legecy/unreferenced.txt", "Riot unreferenced archive")
        self.repo.write("private/players.json", "not a knowledge source")
        self.assertEqual(self.search("Riot"), initial)
        self.assertEqual(self.search("secret", scope="all")["results"], [])

    def test_private_and_hidden_sources_are_rejected_even_if_inactive(self) -> None:
        for source_path in (
            ".local/cache.txt", ".git/config", ".env", ".env.local", "src/.env.test",
            ".ssh/id_rsa", "private/players.json", "src/secrets.json", "src/credentials.toml", "src/key.pem",
        ):
            with self.subTest(source=source_path):
                self.repo.facts = []
                self.repo.fact(path=source_path, status="stale")
                with self.assertRaisesRegex(KnowledgeError, "private or hidden"):
                    check(self.root)

    def test_private_configuration_and_runtime_paths_are_blocked_before_source_read(self) -> None:
        for source_path in (
            "config.yaml", "config.json", "config.production.toml", "config/dev.yaml",
            "legecy/config/dev.yaml", "settings.ini", "data/players.json",
            "artifacts/export.json", "logs/players.txt", "private/players.json",
            "credentials/access.json", "legecy/experiments/run/data/players.json",
            "src/artifacts/output.txt", "backups/players.csv", "secrets.yaml",
        ):
            with self.subTest(source=source_path):
                self.repo.facts = []
                self.repo.fact(path=source_path, scope="legacy" if source_path.startswith("legecy/") else "current")
                with patch("tools.agent_system.knowledge._source_hash") as read_source:
                    with self.assertRaisesRegex(KnowledgeError, "private"):
                        check(self.root)
                    read_source.assert_not_called()

    def test_public_configuration_examples_and_source_code_remain_allowed(self) -> None:
        for position, source_path in enumerate((
            "config/dev.example.yaml", "config.example.yaml", "config/dev.example.toml",
            "legecy/config/dev.example.yaml", "src/config/parser.py",
        )):
            self.repo.fact(
                f"FACT-EXAMPLE-{position}", "Riot public configuration example", path=source_path,
                scope="legacy" if source_path.startswith("legecy/") else "current",
            )
        self.assertEqual(check(self.root)["facts"], 5)
        self.assertEqual(len(self.search("Riot", scope="all")["results"]), 5)

    def test_gitignored_sources_are_blocked_before_read_even_when_tracked(self) -> None:
        self.git("init", "--quiet")
        self.repo.fact(path="notes/local-only.txt")
        self.git("add", "notes/local-only.txt")
        self.repo.write(".gitignore", "notes/local-only.txt\n")
        with patch("tools.agent_system.knowledge._source_hash") as read_source:
            with self.assertRaisesRegex(KnowledgeError, "Git-ignored source"):
                check(self.root)
            read_source.assert_not_called()

    def test_nested_gitignore_rules_and_example_exceptions_are_respected(self) -> None:
        self.git("init", "--quiet")
        self.repo.write(".gitignore", "/config/*.yaml\n!/config/*.example.yaml\n")
        self.repo.fact("FACT-EXAMPLE", path="config/dev.example.yaml")
        self.assertTrue(check(self.root)["ok"])
        self.repo.fact("FACT-HOST", path="notes/host-only.txt", status="stale")
        self.repo.write("notes/.gitignore", "host-only.txt\n")
        with patch("tools.agent_system.knowledge._source_hash") as read_source:
            with self.assertRaisesRegex(KnowledgeError, "Git-ignored source"):
                check(self.root)
            read_source.assert_not_called()

    def test_changed_ignore_rules_block_subsequent_search(self) -> None:
        self.git("init", "--quiet")
        self.repo.fact(path="notes/host-only.txt")
        self.assertEqual(len(self.search("Riot")["results"]), 1)
        self.repo.write(".gitignore", "/notes/host-only.txt\n")
        with self.assertRaisesRegex(KnowledgeError, "Git-ignored source"):
            self.search("Riot")

    def test_git_context_cannot_be_redirected_by_inherited_environment(self) -> None:
        self.git("init", "--quiet")
        self.repo.fact(path="notes/local-only.txt")
        self.repo.write(".gitignore", "notes/local-only.txt\n")
        with patch.dict(os.environ, {"GIT_DIR": str(self.base / "does-not-exist"), "GIT_WORK_TREE": str(self.base)}):
            with self.assertRaisesRegex(KnowledgeError, "Git-ignored source"):
                check(self.root)

    def test_git_privacy_check_failure_does_not_fall_back_to_reading_sources(self) -> None:
        self.repo.fact()
        self.repo.write(".git", "not valid Git metadata")
        with patch("tools.agent_system.knowledge._source_hash") as read_source:
            with self.assertRaisesRegex(KnowledgeError, "cannot safely validate Git ignore rules"):
                check(self.root)
            read_source.assert_not_called()

    def test_empty_ancestor_git_placeholder_does_not_make_child_a_git_repository(self) -> None:
        # Reproduce a sandbox's empty protected ancestor metadata placeholder
        # inside our disposable fixture, never alter the host's /tmp/.git.
        (self.base / ".git").mkdir(mode=0o555)
        self.repo.fact()
        self.assertTrue(check(self.root)["ok"])
        self.assertEqual(self.search("Riot")["results"][0]["id"], "FACT-CURRENT")

    def test_path_escape_and_noncanonical_paths_are_rejected(self) -> None:
        fact = self.repo.fact()
        for path in ("../outside.txt", "/tmp/outside.txt", "src/../../outside", "src//collector.txt", "./src/collector.txt", "src\\collector.txt"):
            with self.subTest(path=path):
                fact["sources"][0]["path"] = path
                self.repo.save()
                with self.assertRaises(KnowledgeError):
                    check(self.root)

    def test_file_and_directory_symlinks_cannot_escape_or_alias_sources(self) -> None:
        outside = self.base / "outside.txt"
        outside.write_text("outside evidence", encoding="utf-8")
        fact = self.repo.fact()
        self.repo.write("safe.txt", "safe evidence")
        (self.root / "outside-link.txt").symlink_to(outside)
        (self.root / "inside-link.txt").symlink_to(self.root / "safe.txt")
        (self.root / "directory-link").symlink_to(self.base, target_is_directory=True)
        for path in ("outside-link.txt", "inside-link.txt", "directory-link/outside.txt"):
            with self.subTest(path=path):
                fact["sources"][0]["path"] = path
                self.repo.save()
                with self.assertRaisesRegex(KnowledgeError, "symlinks"):
                    check(self.root)

    def test_facts_manifest_cannot_itself_be_a_symlink(self) -> None:
        target = self.base / "outside-facts.json"
        target.write_bytes((self.root / FACTS_PATH).read_bytes())
        (self.root / FACTS_PATH).unlink()
        (self.root / FACTS_PATH).symlink_to(target)
        with self.assertRaisesRegex(KnowledgeError, "symlinks"):
            check(self.root)

    @unittest.skipUnless(hasattr(os, "mkfifo"), "POSIX FIFO test")
    def test_nonregular_source_is_rejected_without_reading_fifo(self) -> None:
        fact = self.repo.fact()
        os.mkfifo(self.root / "pipe")
        fact["sources"][0]["path"] = "pipe"
        self.repo.save()
        with self.assertRaisesRegex(KnowledgeError, "not a regular file"):
            check(self.root)

    def test_missing_active_source_fails_but_inactive_history_can_remain(self) -> None:
        fact = self.repo.fact()
        (self.root / "src/collector.txt").unlink()
        with self.assertRaisesRegex(KnowledgeError, "missing source"):
            check(self.root)
        fact["status"] = "stale"
        self.repo.save()
        self.assertTrue(any("missing source" in warning for warning in check(self.root)["warnings"]))

    def test_external_urls_are_never_fetched_or_sufficient_for_verified_status(self) -> None:
        fact = self.repo.fact()
        url = {"url": "https://example.invalid/does-not-exist"}
        fact["sources"].append(url)
        self.repo.save()
        self.assertEqual(len(check(self.root)["warnings"]), 1)
        self.assertIn(url, self.search("Riot")["results"][0]["sources"])
        fact["sources"] = [url]
        self.repo.save()
        with self.assertRaisesRegex(KnowledgeError, "local hashed evidence"):
            check(self.root)
        fact["status"] = "candidate"
        fact["verified_at"] = None
        self.repo.save()
        self.assertTrue(check(self.root)["ok"])

    def test_fts_operators_and_sql_are_literal_data(self) -> None:
        self.repo.fact("FACT-AND", "Riot OR archive", path="src/and.txt")
        self.repo.fact("FACT-RIOT", "Riot", path="src/riot.txt")
        self.repo.fact("FACT-ARCHIVE", "archive", path="src/archive.txt")
        self.repo.fact("FACT-PREFIX", "Rioters", path="src/prefix.txt")
        self.assertEqual([item["id"] for item in self.search("Riot OR archive")["results"]], ["FACT-AND"])
        self.assertEqual(self.search("Riot' OR 1=1 --")["results"], [])
        self.assertEqual(self.search('Riot\"; DROP TABLE facts_fts; --')["results"], [])
        self.assertEqual({item["id"] for item in self.search("Riot*")["results"]}, {"FACT-RIOT", "FACT-AND"})
        self.assertEqual(self.search("claim:Riot")["results"], [])
        self.assertEqual(len(self.search("Riot")["results"]), 2)

    def test_empty_or_excessive_query_and_bad_limit_fail_clearly(self) -> None:
        self.repo.fact()
        for query in ("", "  ", '"*():', "a\x00b", "\ud800", "x" * 4097, "word " * 129):
            with self.subTest(query=query[:30]):
                with self.assertRaises(KnowledgeError):
                    self.search(query)
        for limit in (0, 101, True):
            with self.assertRaises(KnowledgeError):
                self.search("Riot", limit=limit)

    def test_malformed_json_schema_and_timestamps_fail_check_and_search(self) -> None:
        original = self.repo.fact()
        invalid_documents = [
            "{", '{"schema_version":1,"schema_version":1,"facts":[]}',
            '{"schema_version":NaN,"facts":[]}', "[]",
            json.dumps({"schema_version": True, "facts": []}),
            json.dumps({"schema_version": 1, "facts": {}, "extra": 1}),
        ]
        for modifications in (
            {"status": "unknown"}, {"status": []}, {"scope": {}}, {"claim": ""},
            {"id": "../../bad"}, {"sources": "not an array"}, {"sources": [None]},
            {"verified_at": None}, {"verified_at": "2026-01-01"},
            {"verified_at": "0001-01-01T00:00:00+23:00"}, {"claim": "\ud800"},
            {"valid_until": "2025-01-01T00:00:00Z"}, {"valid_until": None},
            {"sources": [{"path": "src/collector.txt", "sha256": "not-a-hash"}]},
            {"sources": [{"url": "https://user:password@example.test/page"}]},
            {"sources": [{"url": "file:///etc/passwd"}]},
        ):
            fact = copy.deepcopy(original)
            fact.update(modifications)
            invalid_documents.append(json.dumps({"schema_version": 1, "facts": [fact]}))
        for document in invalid_documents:
            with self.subTest(document=document[:100]):
                self.repo.write(FACTS_PATH, document)
                with self.assertRaises(KnowledgeError):
                    check(self.root)
                with self.assertRaises(KnowledgeError):
                    self.search("Riot")

    def test_fresh_search_uses_memory_only_and_does_not_write_files(self) -> None:
        self.repo.fact()
        before = {path.relative_to(self.root): path.read_bytes() for path in self.root.rglob("*") if path.is_file()}
        with patch("tools.agent_system.knowledge.sqlite3.connect", wraps=sqlite3.connect) as connect:
            result = self.search("Riot")
        connect.assert_called_once_with(":memory:")
        self.assertEqual(result["results"][0]["id"], "FACT-CURRENT")
        self.assertEqual(result["warnings"], [])
        self.assertEqual(before, {path.relative_to(self.root): path.read_bytes() for path in self.root.rglob("*") if path.is_file()})
        self.assertFalse((self.root / ".local").exists())

    def test_obsolete_cache_and_private_directory_symlink_are_not_accessed(self) -> None:
        self.repo.fact()
        outside = self.base / "unrelated-private-files"
        outside.mkdir()
        artifact = outside / "knowledge.sqlite3"
        artifact.write_bytes(b"obsolete and invalid SQLite cache")
        (self.root / ".local").symlink_to(outside, target_is_directory=True)
        self.assertEqual(self.search("Riot")["results"][0]["id"], "FACT-CURRENT")
        self.assertEqual(list(outside.iterdir()), [artifact])
        self.assertEqual(artifact.read_bytes(), b"obsolete and invalid SQLite cache")

    def test_source_hash_change_blocks_check_and_search_without_refreshing_evidence(self) -> None:
        self.repo.fact()
        manifest = (self.root / FACTS_PATH).read_bytes()
        self.assertEqual(len(self.search("Riot")["results"]), 1)
        self.repo.write("src/collector.txt", "Different implementation")
        for action in (lambda: check(self.root), lambda: self.search("Riot")):
            with self.subTest(action=action):
                with self.assertRaisesRegex(KnowledgeError, "SHA-256 changed"):
                    action()
        self.assertEqual((self.root / FACTS_PATH).read_bytes(), manifest)

    def test_updated_authoritative_evidence_is_available_on_next_search(self) -> None:
        fact = self.repo.fact()
        initial = self.search("Riot")
        self.repo.write("src/collector.txt", "updated evidence")
        fact["sources"][0]["sha256"] = hashlib.sha256(b"updated evidence").hexdigest()
        self.repo.save()
        current = self.search("Riot")
        self.assertNotEqual(initial["fingerprint"], current["fingerprint"])
        self.assertEqual(current["results"][0]["sources"], fact["sources"])

    def test_changed_authoritative_claim_is_returned_without_rebuild(self) -> None:
        fact = self.repo.fact()
        self.assertEqual(self.search("retries")["results"][0]["claim"], fact["claim"])
        fact["claim"] = "Riot updates use stable identifiers"
        self.repo.save()
        self.assertEqual(self.search("identifiers")["results"][0]["claim"], fact["claim"])
        self.assertEqual(self.search("retries")["results"], [])

    def test_expired_source_drift_or_deletion_warns_without_blocking_valid_queries(self) -> None:
        self.repo.fact()
        fact = self.repo.fact("FACT-EXPIRED", "Riot expired evidence", path="src/expired.txt", valid_until=NOW)
        source = self.root / "src/expired.txt"
        original_hash = fact["sources"][0]["sha256"]
        for missing in (False, True):
            with self.subTest(missing=missing):
                if missing:
                    source.unlink()
                else:
                    source.write_text("changed evidence", encoding="utf-8")
                checked = check(self.root, now=NOW)
                result = self.search("Riot")
                self.assertEqual(result["warnings"], checked["warnings"])
                self.assertEqual([item["id"] for item in result["results"]], ["FACT-CURRENT"])
                self.assertTrue(any("expired" in warning for warning in result["warnings"]))
                reason = "missing source" if missing else "SHA-256 changed"
                self.assertTrue(any(reason in warning for warning in result["warnings"]))
                self.assertEqual(json.loads((self.root / FACTS_PATH).read_text())["facts"][1]["sources"][0]["sha256"], original_hash)

    def test_expired_fact_does_not_relax_another_active_claim_on_the_same_source(self) -> None:
        self.repo.fact(valid_until=NOW)
        fact = copy.deepcopy(self.repo.facts[0])
        fact["id"] = "FACT-STILL-ACTIVE"
        del fact["valid_until"]
        self.repo.facts.append(fact)
        self.repo.save()
        self.repo.write("src/collector.txt", "changed evidence")
        with self.assertRaisesRegex(KnowledgeError, "FACT-STILL-ACTIVE.*SHA-256 changed"):
            self.search("Riot")

    def test_inactive_drift_or_deletion_preserves_other_search_results(self) -> None:
        self.repo.fact()
        self.repo.fact("FACT-STALE", "old Riot fact", path="src/old.txt", status="stale")
        self.assertEqual(len(self.search("Riot")["results"]), 1)
        for missing in (False, True):
            with self.subTest(missing=missing):
                if missing:
                    (self.root / "src/old.txt").unlink()
                else:
                    self.repo.write("src/old.txt", "changed inactive source")
                result = self.search("Riot")
                self.assertEqual([item["id"] for item in result["results"]], ["FACT-CURRENT"])
                self.assertEqual(len(result["warnings"]), 1)
                self.assertIn("inactive", result["warnings"][0])

    def test_each_root_reads_its_own_sources_even_with_identical_fact_manifests(self) -> None:
        self.repo.fact()
        other = Repository(self.base / "other-repository")
        other.fact()
        self.assertEqual((self.root / FACTS_PATH).read_bytes(), (other.root / FACTS_PATH).read_bytes())
        initial = self.search("Riot")
        same = search(other.root, "Riot", now=NOW)
        self.assertEqual(same["root"], str(other.root))
        self.assertNotEqual(same["fingerprint"], initial["fingerprint"])
        other.write("src/collector.txt", "different source in another repository")
        with self.assertRaisesRegex(KnowledgeError, "SHA-256 changed"):
            search(other.root, "Riot", now=NOW)
        self.assertEqual(self.search("Riot"), initial)

    def test_source_mutation_during_query_blocks_results(self) -> None:
        self.repo.fact()
        build_memory = knowledge._memory_index

        def mutate(snapshot, instant):
            self.repo.write("src/collector.txt", "changed during search")
            return build_memory(snapshot, instant)

        with patch("tools.agent_system.knowledge._memory_index", side_effect=mutate):
            with self.assertRaisesRegex(KnowledgeError, "SHA-256 changed"):
                self.search("Riot")

    def test_authoritative_claim_mutation_during_query_blocks_results(self) -> None:
        fact = self.repo.fact()
        build_memory = knowledge._memory_index

        def mutate(snapshot, instant):
            fact["claim"] = "This changed while a search was running"
            self.repo.save()
            return build_memory(snapshot, instant)

        with patch("tools.agent_system.knowledge._memory_index", side_effect=mutate):
            with self.assertRaisesRegex(KnowledgeError, "knowledge changed during search"):
                self.search("Riot")

    def test_source_replaced_by_symlink_during_query_is_rejected(self) -> None:
        self.repo.fact()
        outside = self.base / "outside.txt"
        outside.write_bytes((self.root / "src/collector.txt").read_bytes())
        build_memory = knowledge._memory_index

        def mutate(snapshot, instant):
            source = self.root / "src/collector.txt"
            source.unlink()
            source.symlink_to(outside)
            return build_memory(snapshot, instant)

        with patch("tools.agent_system.knowledge._memory_index", side_effect=mutate):
            with self.assertRaisesRegex(KnowledgeError, "symlinks"):
                self.search("Riot")

    def test_missing_sqlite_fts5_is_reported_as_a_tool_limit(self) -> None:
        self.repo.fact()
        with patch("tools.agent_system.knowledge.sqlite3.connect", side_effect=sqlite3.OperationalError("no such module: fts5")):
            with self.assertRaisesRegex(KnowledgeError, "cannot search using SQLite FTS5.*no such module"):
                self.search("Riot")

    def test_cli_supports_explicit_root_from_an_unrelated_directory(self) -> None:
        self.repo.fact()
        environment = dict(os.environ)
        environment["PYTHONPATH"] = str(Path(__file__).resolve().parents[1])
        commands = (
            ["--root", str(self.root), "check"],
            ["search", "Riot", "--root", str(self.root)],
        )
        for command in commands:
            with self.subTest(command=command):
                result = subprocess.run(
                    [sys.executable, "-B", "-m", "tools.agent_system", *command],
                    cwd=self.base, env=environment, capture_output=True, text=True, timeout=15,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                output = json.loads(result.stdout)
                self.assertEqual(output.get("root", str(self.root)), str(self.root))
                if command[0] == "search":
                    self.assertEqual(output["results"][0]["id"], "FACT-CURRENT")
        self.repo.write(FACTS_PATH, "malformed")
        failed = subprocess.run(
            [sys.executable, "-B", "-m", "tools.agent_system", "check", "--root", str(self.root)],
            cwd=self.base, env=environment, capture_output=True, text=True, timeout=15,
        )
        self.assertEqual(failed.returncode, 2)
        self.assertIn("knowledge error:", failed.stderr)
        self.assertNotIn("Traceback", failed.stderr)
        self.assertEqual(failed.stdout, "")


if __name__ == "__main__":
    unittest.main()
