"""Integration tests against real Git indexes and executable staged checks."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import sys
import unittest

from tools import precommit


class PrecommitTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='gogg-precommit-test-')
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.root = self.base / 'repo'
        self.root.mkdir()
        self.env = {key: value for key, value in os.environ.items() if not key.startswith('GIT_')}
        self.env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull, GIT_OPTIONAL_LOCKS='0')
        self.git('init', '--quiet')
        self.write('tools/precommit.py', Path(precommit.__file__).read_text())
        self.write('.gitignore', '/.local/\nignored/\n')
        self.write('Makefile', 'check:\n\t@python3 -B check.py\n')
        self.write('engineering/README.md', 'Active fixture foundation\n')
        self.write('tools/check_foundation.py', '# Fixture entrypoint\n')
        self.write('check.py', "from pathlib import Path\nassert Path('value.txt').read_text() == 'valid\\n'\n")
        self.write('value.txt', 'valid\n')
        self.scanner = self.write('.local/scanner', '#!/usr/bin/env python3\nimport json, os, sys\n'
                                  'from pathlib import Path\n'
                                  "Path('.local/scan.json').write_text(json.dumps({'args': sys.argv[1:], 'cwd': os.getcwd()}))\n")
        self.scanner.chmod(0o700)
        self.env['GITLEAKS_BIN'] = str(self.scanner)
        self.git('add', '.gitignore', 'Makefile', 'engineering/README.md',
                 'tools/check_foundation.py', 'check.py', 'value.txt')

    def write(self, relative, text):
        target = self.root / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)
        return target

    def git(self, *arguments, data=None):
        return subprocess.run(
            ['git', '-c', 'init.templateDir=', *arguments], cwd=self.root, env=self.env,
            input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True,
        ).stdout

    def invoke(self, *, preserve=True):
        index = Path(self.env.get('GIT_INDEX_FILE', str(self.root / '.git/index')))
        before = index.read_bytes()
        status = self.git('status', '--porcelain=v1', '-z')
        result = subprocess.run(
            [sys.executable, '-B', str(self.root / 'tools/precommit.py')],
            cwd=self.root, env=self.env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            text=True, check=False,
        )
        if preserve:
            self.assertEqual(index.read_bytes(), before)
            self.assertEqual(self.git('status', '--porcelain=v1', '-z'), status)
        return result.returncode, result.stdout

    def test_repository_path_with_spaces_and_unicode(self):
        self.root = self.root.rename(self.base / 'repo space-项目')
        self.scanner = self.root / '.local/scanner'
        self.env['GITLEAKS_BIN'] = str(self.scanner)
        self.assertEqual(self.invoke()[0], 0)

    def test_staged_success_preserves_worktree_index_and_untracked_files(self):
        self.write('untracked.txt', 'keep me\n')
        self.write('.local/private.txt', 'private fixture\n')
        objects = {path.relative_to(self.root / '.git/objects'): path.read_bytes()
                   for path in (self.root / '.git/objects').rglob('*') if path.is_file()}
        self.assertEqual(self.invoke()[0], 0)
        self.assertEqual(objects, {path.relative_to(self.root / '.git/objects'): path.read_bytes()
                                   for path in (self.root / '.git/objects').rglob('*') if path.is_file()})
        self.assertEqual((self.root / 'untracked.txt').read_text(), 'keep me\n')
        self.assertEqual((self.root / '.local/private.txt').read_text(), 'private fixture\n')
        scan = json.loads((self.root / '.local/scan.json').read_text())
        self.assertEqual(scan, {'args': ['git', '--staged', '--redact', '--no-banner', str(self.root)],
                                'cwd': str(self.root)})
        self.assertEqual(self.git('rev-list', '--all'), b'')

    def test_broken_staged_and_correct_working_file_fails(self):
        self.write('value.txt', 'broken\n')
        self.git('add', 'value.txt')
        self.write('value.txt', 'valid\n')
        self.assertNotEqual(self.invoke()[0], 0)
        self.assertFalse((self.root / '.local/scan.json').exists())

    def test_valid_staged_and_broken_working_file_passes(self):
        self.write('value.txt', 'broken\n')
        self.assertEqual(self.invoke()[0], 0)
        self.assertEqual((self.root / 'value.txt').read_text(), 'broken\n')

    def test_untracked_required_check_file_cannot_fill_missing_staged_file(self):
        self.git('rm', '--cached', 'check.py')
        self.assertTrue((self.root / 'check.py').is_file())
        self.assertNotEqual(self.invoke()[0], 0)

    def test_pythonpath_cannot_supply_unstaged_source_to_checks(self):
        self.write('external_only.py', 'value = 42\n')
        self.write('check.py', 'import external_only\nassert external_only.value == 42\n')
        self.git('add', 'check.py')
        self.env['PYTHONPATH'] = str(self.root)
        code, output = self.invoke()
        self.assertNotEqual(code, 0)
        self.assertIn('ModuleNotFoundError', output)
        self.assertFalse((self.root / '.local/scan.json').exists())

    def test_makefiles_cannot_inject_unstaged_check_settings(self):
        self.write('Makefile', 'PYTHON ?= python3\ncheck:\n\t@$(PYTHON) -B check.py\n')
        self.write('value.txt', 'broken\n')
        self.git('add', 'Makefile', 'value.txt')
        self.write('value.txt', 'valid\n')
        settings = self.write('local-check-settings.mk', 'PYTHON := true\n')
        self.env['MAKEFILES'] = str(settings)
        self.assertNotEqual(self.invoke()[0], 0)
        self.assertFalse((self.root / '.local/scan.json').exists())

    def test_gnumakeflags_cannot_skip_staged_checks(self):
        self.write('value.txt', 'broken\n')
        self.git('add', 'value.txt')
        self.env['GNUMAKEFLAGS'] = '--just-print'
        self.assertNotEqual(self.invoke()[0], 0)
        self.assertFalse((self.root / '.local/scan.json').exists())

    def test_missing_staged_foundation_blocks_before_running_make(self):
        self.git('rm', '--cached', 'engineering/README.md')
        marker = self.base / 'make-ran'
        self.write('Makefile', 'check:\n\t@touch ' + str(marker) + '\n')
        self.git('add', 'Makefile')
        code, output = self.invoke()
        self.assertEqual(code, 2)
        self.assertIn('missing the active engineering foundation', output)
        self.assertFalse(marker.exists())

    def test_intent_to_add_file_is_not_part_of_candidate(self):
        self.git('rm', '--cached', 'check.py')
        self.git('add', '--intent-to-add', 'check.py')
        self.assertNotEqual(self.invoke()[0], 0)

    def test_staged_makefile_is_used(self):
        self.write('Makefile', 'check:\n\t@false\n')
        self.assertEqual(self.invoke()[0], 0)

    def test_missing_scanner_blocks(self):
        self.env['GITLEAKS_BIN'] = str(self.base / 'unavailable')
        code, output = self.invoke()
        self.assertEqual(code, 2)
        self.assertIn('Secret scanner unavailable', output)

    def test_nonexecutable_scanner_blocks(self):
        self.scanner.chmod(0o600)
        self.assertEqual(self.invoke()[0], 2)

    def test_relative_scanner_resolves_from_project_root(self):
        self.env['GITLEAKS_BIN'] = '.local/scanner'
        self.assertEqual(self.invoke()[0], 0)

    def test_scanner_basename_resolves_from_relative_path_entry(self):
        self.env['GITLEAKS_BIN'] = 'scanner'
        self.env['PATH'] = '.local' + os.pathsep + self.env['PATH']
        self.assertEqual(self.invoke()[0], 0)

    def test_missing_path_basename_does_not_fall_back(self):
        self.env['GITLEAKS_BIN'] = 'missing-fixture-scanner'
        self.assertEqual(self.invoke()[0], 2)

    def test_failed_secret_scan_blocks(self):
        self.scanner.write_text('#!/bin/sh\nexit 7\n')
        self.assertEqual(self.invoke()[0], 7)

    def test_staged_symlink_is_rejected_without_following_target(self):
        outside = self.base / 'outside'
        outside.write_text('outside private fixture\n')
        (self.root / 'linked.txt').symlink_to(outside)
        self.git('add', 'linked.txt')
        code, output = self.invoke()
        self.assertEqual(code, 2)
        self.assertIn('symlinks', output)
        self.assertEqual(outside.read_text(), 'outside private fixture\n')

    def test_staged_gitlink_is_rejected(self):
        self.git('update-index', '--add', '--cacheinfo', '160000,' + '1' * 40 + ',submodule')
        code, output = self.invoke()
        self.assertEqual(code, 2)
        self.assertIn('submodules', output)

    def test_private_paths_are_rejected_even_without_ignore_rules(self):
        self.write('.gitignore', '')
        self.git('add', '.gitignore')
        for name in ('.local/private.txt', 'nested/.env', 'deploy/key.pem', 'data/players.json',
                     'legecy/experiments/run/data/players.json'):
            with self.subTest(path=name):
                self.write(name, 'private fixture\n')
                self.git('add', '--force', name)
                code, output = self.invoke()
                self.assertEqual(code, 2)
                self.assertIn('private', output)
                self.git('rm', '--cached', name)

    def test_staged_gitignore_applies_even_when_working_ignore_is_changed(self):
        self.write('ignored/private.txt', 'private fixture\n')
        self.git('add', '--force', 'ignored/private.txt')
        self.write('.gitignore', '')
        code, output = self.invoke()
        self.assertEqual(code, 2)
        self.assertIn('Git-ignored', output)

    def test_unstaged_ignore_rule_does_not_change_valid_staged_candidate(self):
        self.write('.gitignore', '/.local/\n/value.txt\n')
        self.assertEqual(self.invoke()[0], 0)

    def test_nested_and_local_exclude_rules_are_respected(self):
        for rule_path in ('notes/.gitignore', '.git/info/exclude'):
            with self.subTest(rule=rule_path):
                self.write('notes/private.txt', 'private fixture\n')
                self.git('add', '--force', 'notes/private.txt')
                self.write(rule_path, 'private.txt\n')
                if rule_path.endswith('.gitignore'):
                    self.git('add', rule_path)
                self.assertEqual(self.invoke()[0], 2)
                self.write(rule_path, '')
                if rule_path.endswith('.gitignore'):
                    self.git('add', rule_path)

    def test_public_hidden_foundation_and_legacy_example_files_are_allowed(self):
        for name in ('.codex/config.toml', '.agents/skills/example/SKILL.md', '.githooks/pre-commit',
                     'legecy/.env.example', 'legecy/README.md'):
            self.write(name, 'public fixture\n')
            self.git('add', name)
        self.assertEqual(self.invoke()[0], 0)

    def test_scanner_changing_index_invalidates_check(self):
        self.write('later.txt', 'later\n')
        self.scanner.write_text('#!/bin/sh\ngit add later.txt\n')
        code, output = self.invoke(preserve=False)
        self.assertEqual(code, 2)
        self.assertIn('Git index changed', output)

    def test_make_changing_original_index_invalidates_check(self):
        self.write('later.txt', 'later\n')
        self.write('check.py', 'import subprocess\nsubprocess.run(' + repr(
            ['git', '-C', str(self.root), 'add', 'later.txt']) + ', check=True)\n')
        self.git('add', 'check.py')
        code, output = self.invoke(preserve=False)
        self.assertEqual(code, 2)
        self.assertIn('Git index changed', output)
        self.assertFalse((self.root / '.local/scan.json').exists())

    def test_alternate_index_is_honored_but_not_inherited_by_checks(self):
        alternate = self.base / 'alternate-index'
        alternate.write_bytes((self.root / '.git/index').read_bytes())
        self.env['GIT_INDEX_FILE'] = str(alternate)
        self.env['GIT_WORK_TREE'] = str(self.root)
        self.env['GIT_DIR'] = str(self.root / '.git')
        self.write('check.py', "import os, subprocess\nfrom pathlib import Path\n"
                   "assert not any(k.startswith('GIT_') for k in os.environ)\n"
                   "assert Path(subprocess.check_output(['git','rev-parse','--show-toplevel']).decode().strip()) == Path.cwd()\n")
        self.git('add', 'check.py')
        normal_before = (self.root / '.git/index').read_bytes()
        self.assertEqual(self.invoke()[0], 0)
        self.assertEqual((self.root / '.git/index').read_bytes(), normal_before)

    def test_export_does_not_apply_attributes_or_run_smudge_filter(self):
        self.write('.gitattributes', 'value.txt export-ignore filter=forbidden\n')
        self.git('add', '.gitattributes')
        marker = self.base / 'filter-ran'
        self.git('config', 'filter.forbidden.smudge', 'touch ' + str(marker))
        self.assertEqual(self.invoke()[0], 0)
        self.assertFalse(marker.exists())

    def test_unmerged_index_cannot_be_checked(self):
        oid = self.git('hash-object', '-w', '--stdin', data=b'unmerged\n').strip()
        self.git('update-index', '--index-info', data=(
            b'0 ' + b'0' * 40 + b'\tvalue.txt\n' +
            b'100644 ' + oid + b' 1\tvalue.txt\n' +
            b'100644 ' + oid + b' 2\tvalue.txt\n'))
        self.assertEqual(self.invoke()[0], 2)
        self.assertFalse((self.root / '.local/scan.json').exists())

    def test_split_index_can_be_checked_without_mutating_original(self):
        self.git('update-index', '--split-index')
        self.assertEqual(self.invoke()[0], 0)


if __name__ == '__main__':
    unittest.main()
