"""Reject broken task/config/skill candidates in isolated foundation fixtures."""
import json
from pathlib import Path
import shutil
import tempfile
import unittest

from tools.check_foundation import validate


class FoundationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        source = Path(__file__).resolve().parents[1]
        for name in ('.codex', '.agents'):
            shutil.copytree(source / name, self.root / name)
        for name in ('AGENTS.md', 'README.md', 'engineering/README.md', 'engineering/tasks/README.md'):
            self.write(name, '# Fixture\n')
        self.contract = {
            'schema_version': 1, 'id': 'repair', 'status': 'active',
            'objective': 'Reject broken candidates', 'authorization': ['Local repair'],
            'out_of_scope': ['Production'], 'acceptance': ['Known bad candidates fail'],
        }
        self.save_contract()

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def save_contract(self, directory='repair'):
        self.write(f'engineering/tasks/{directory}/contract.json', json.dumps(self.contract) + '\n')

    def errors(self):
        return '\n'.join(validate(self.root)[0])

    def test_valid_candidate_identifies_active_contract(self):
        errors, summary = validate(self.root)
        self.assertEqual(errors, [])
        self.assertIn('engineering/tasks/repair/contract.json', summary)

    def test_required_skill_disappearance_is_rejected(self):
        (self.root / '.agents/skills/gogg-delivery/SKILL.md').unlink()
        self.assertIn('gogg-delivery/SKILL.md: cannot read required file', self.errors())

    def test_missing_or_misnamed_skill_frontmatter_is_rejected(self):
        name = '.agents/skills/gogg-delivery/SKILL.md'
        for content in ('# No frontmatter\n', '---\nname: wrong\ndescription: Useful\n---\n',
                        '---\nname: gogg-delivery\ndescription:\n---\n'):
            with self.subTest(content=content):
                self.write(name, content)
                self.assertIn('frontmatter', self.errors())

    def test_required_role_disappearance_is_rejected(self):
        (self.root / '.codex/agents/reviewer.toml').unlink()
        self.assertIn('missing required agent: reviewer', self.errors())

    def test_model_and_effort_drift_is_rejected(self):
        path = self.root / '.codex/config.toml'
        original = path.read_text()
        for key, before, after in (
            ('model', 'gpt-6-astra', 'other'),
            ('model_reasoning_effort', 'xhigh', 'low'),
            ('default_subagent_model', 'gpt-6-astra', 'other'),
            ('default_subagent_reasoning_effort', 'xhigh', 'low'),
        ):
            with self.subTest(key=key):
                path.write_text(original.replace(f'{key} = "{before}"', f'{key} = "{after}"'))
                self.assertIn(f'{key} must be', self.errors())

    def test_disabled_agents_and_excessive_parallelism_are_rejected(self):
        path = self.root / '.codex/config.toml'
        original = path.read_text()
        for before, after, error in (
            ('enabled = true', 'enabled = false', 'enabled must be True'),
            ('max_concurrent_threads_per_session = 3', 'max_concurrent_threads_per_session = 4', 'between 1 and 3'),
        ):
            with self.subTest(error=error):
                path.write_text(original.replace(before, after))
                self.assertIn(error, self.errors())

    def test_read_only_role_and_role_effort_are_checked(self):
        path = self.root / '.codex/agents/reviewer.toml'
        original = path.read_text()
        path.write_text(original.replace('sandbox_mode = "read-only"\n', ''))
        self.assertIn('sandbox_mode must be', self.errors())
        path.write_text(original.replace('"xhigh"', '"low"'))
        self.assertIn('model_reasoning_effort must be', self.errors())

    def test_malformed_configuration_reports_validation_error(self):
        self.write('.codex/config.toml', '[broken\n')
        self.assertIn('invalid TOML', self.errors())

    def test_invalid_role_name_type_reports_validation_error(self):
        path = self.root / '.codex/agents/reviewer.toml'
        path.write_text(path.read_text().replace('name = "reviewer"', 'name = []'))
        self.assertIn('name must be a nonempty string', self.errors())

    def test_malformed_task_json_is_rejected(self):
        for content in ('{invalid\n', '{"id": "one", "id": "two"}\n', '{"schema_version": NaN}\n', '[]\n'):
            with self.subTest(content=content):
                self.write('engineering/tasks/repair/contract.json', content)
                self.assertTrue(self.errors())

    def test_task_missing_acceptance_or_invalid_types_are_rejected(self):
        original = dict(self.contract)
        for key, value in (('schema_version', True), ('id', []), ('objective', ''),
                           ('status', 'done'), ('authorization', []), ('acceptance', [''])):
            with self.subTest(key=key):
                self.contract = {**original, key: value}
                self.save_contract()
                self.assertTrue(self.errors())
        self.contract = dict(original)
        del self.contract['acceptance']
        self.save_contract()
        self.assertIn('missing task fields: acceptance', self.errors())

    def test_multiple_active_tasks_and_duplicate_ids_are_rejected(self):
        self.save_contract('other')
        self.assertIn('duplicate task id', self.errors())
        self.contract['id'] = 'other'
        self.save_contract('other')
        self.assertIn('Only one task can be active', self.errors())

    def test_completed_task_requires_result_and_stops_being_current(self):
        self.contract['status'] = 'completed'
        self.save_contract()
        self.assertIn('completed task requires outcome.md', self.errors())
        self.write('engineering/tasks/repair/outcome.md', '# Historical outcome\n')
        errors, summary = validate(self.root)
        self.assertEqual(errors, [])
        self.assertIn('active task: none', summary)

    def test_missing_task_contract_cannot_hide_an_existing_task(self):
        self.contract['id'] = 'other'
        self.save_contract('other')
        (self.root / 'engineering/tasks/repair/contract.json').unlink()
        self.assertIn('engineering/tasks/repair: missing contract.json', self.errors())

    def test_broken_and_escaping_document_links_are_rejected(self):
        self.write('engineering/README.md', '[missing](missing.md)\n[escape](../../../outside)\n')
        self.assertIn('missing or escaping local link missing.md', self.errors())
        self.assertIn('missing or escaping local link ../../../outside', self.errors())


if __name__ == '__main__':
    unittest.main()
