"""Check active contracts, skills and roles without running archived code."""
import json
from pathlib import Path
import re
import sys
import tomllib

MODEL = 'gpt-6-astra'
EFFORT = 'xhigh'
ROLES = {'analyst', 'curator', 'researcher', 'reviewer', 'verifier'}
READ_ONLY_ROLES = ROLES - {'verifier'}
SKILLS = {'gogg-delivery', 'gogg-knowledge'}
TASK_FIELDS = {'schema_version', 'id', 'status', 'objective', 'authorization', 'out_of_scope', 'acceptance'}


def _unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f'duplicate JSON key: {key}')
        result[key] = value
    return result


def _invalid_constant(value):
    raise ValueError(f'invalid JSON constant: {value}')


def validate(root):
    errors = []

    def read(path):
        try:
            return (root / path).read_text(encoding='utf-8')
        except (OSError, UnicodeError) as exc:
            errors.append(f'{path}: cannot read required file: {exc}')
            return ''

    def toml(path):
        try:
            return tomllib.loads(read(path))
        except tomllib.TOMLDecodeError as exc:
            errors.append(f'{path}: invalid TOML: {exc}')
            return {}

    def require_values(value, expected, label):
        if not isinstance(value, dict):
            errors.append(f'{label}: expected a table')
            return
        for key, wanted in expected.items():
            actual = value.get(key)
            if type(actual) is not type(wanted) or actual != wanted:
                errors.append(f'{label}: {key} must be {wanted!r}')

    config = toml('.codex/config.toml')
    protections = {'sandbox_mode': 'workspace-write', 'approval_policy': 'on-request', 'approvals_reviewer': 'auto_review'}
    require_values(config, {'model': MODEL, 'model_reasoning_effort': EFFORT, **protections}, '.codex/config.toml')
    agents = config.get('agents', {})
    require_values(agents, {'enabled': True, 'default_subagent_model': MODEL,
                           'default_subagent_reasoning_effort': EFFORT}, 'agents')
    limit = agents.get('max_concurrent_threads_per_session') if isinstance(agents, dict) else None
    if type(limit) is not int or not 1 <= limit <= 3:
        errors.append('agents: max_concurrent_threads_per_session must be an integer between 1 and 3')
    require_values(config.get('features', {}), {'multi_agent': True}, 'features')
    role_names = set()
    for path in sorted((root / '.codex/agents').glob('*.toml')):
        relative = path.relative_to(root)
        role = toml(relative)
        for key in ('name', 'description', 'developer_instructions'):
            if not isinstance(role.get(key), str) or not role[key].strip():
                errors.append(f'{relative}: {key} must be a nonempty string')
        name = role.get('name')
        if isinstance(name, str):
            if name in role_names:
                errors.append(f'{relative}: duplicate agent name {name}')
            role_names.add(name)
            if name != path.stem:
                errors.append(f'{relative}: name must match the filename')
        require_values(role, {'model': MODEL, 'model_reasoning_effort': EFFORT}, str(relative))
        for key, value in protections.items():
            if key in role:
                expected = value
                if key == 'sandbox_mode' and isinstance(name, str) and name in READ_ONLY_ROLES:
                    expected = 'read-only'
                require_values(role, {key: expected}, str(relative))
        if isinstance(name, str) and name in READ_ONLY_ROLES:
            require_values(role, {'sandbox_mode': 'read-only'}, str(relative))
    for name in sorted(ROLES - role_names):
        errors.append(f'missing required agent: {name}')

    for name in sorted(SKILLS):
        path = f'.agents/skills/{name}/SKILL.md'
        content = read(path)
        frontmatter = re.match(r'\A---\n(.*?)\n---\n', content, re.S)
        if not frontmatter:
            errors.append(f'{path}: missing YAML frontmatter')
            continue
        # Focused check for the authored single-line required fields, not a
        # general YAML parser or a substitute for behavioral evaluation.
        for key in ('name', 'description'):
            values = re.findall(rf'^{key}:[ \t]*(\S[^\n]*)$', frontmatter[1], re.M)
            if len(values) != 1 or (key == 'name' and values[0] != name):
                errors.append(f'{path}: invalid required frontmatter field {key}')

    contracts = sorted((root / 'engineering/tasks').glob('*/contract.json'))
    if not contracts:
        errors.append('engineering/tasks: missing task contracts')
    for directory in sorted((root / 'engineering/tasks').glob('*/')):
        if not directory.name.startswith('.') and not (directory / 'contract.json').is_file():
            errors.append(f'{directory.relative_to(root)}: missing contract.json')
    task_ids, active = set(), []
    for path in contracts:
        relative = path.relative_to(root)
        try:
            task = json.loads(read(relative), object_pairs_hook=_unique_object, parse_constant=_invalid_constant)
        except ValueError as exc:
            errors.append(f'{relative}: invalid task JSON: {exc}')
            continue
        if not isinstance(task, dict):
            errors.append(f'{relative}: task contract must be an object')
            continue
        missing = TASK_FIELDS - task.keys()
        if missing:
            errors.append(f'{relative}: missing task fields: {", ".join(sorted(missing))}')
        if type(task.get('schema_version')) is not int or task['schema_version'] != 1:
            errors.append(f'{relative}: schema_version must be 1')
        task_id = task.get('id')
        if not isinstance(task_id, str) or not re.fullmatch(r'[a-z][a-z0-9-]*', task_id):
            errors.append(f'{relative}: invalid task id')
        elif task_id in task_ids:
            errors.append(f'{relative}: duplicate task id {task_id}')
        else:
            task_ids.add(task_id)
        if task.get('status') not in ('active', 'completed'):
            errors.append(f'{relative}: status must be active or completed')
        if task.get('status') == 'active':
            active.append(str(relative))
        if not isinstance(task.get('objective'), str) or not task['objective'].strip():
            errors.append(f'{relative}: objective must be a nonempty string')
        for key in ('authorization', 'out_of_scope', 'acceptance'):
            value = task.get(key)
            if not isinstance(value, list) or not value or any(not isinstance(item, str) or not item.strip() for item in value):
                errors.append(f'{relative}: {key} must be a nonempty array of nonempty strings')
        if task.get('status') == 'completed' and not path.with_name('outcome.md').is_file():
            errors.append(f'{relative}: completed task requires outcome.md')
    if len(active) > 1:
        errors.append(f'Only one task can be active: {", ".join(active)}')

    docs = {root / name for name in ('AGENTS.md', 'README.md', 'engineering/README.md', 'engineering/tasks/README.md')}
    docs.update((root / 'engineering').rglob('*.md'))
    docs.update((root / '.agents/skills').rglob('SKILL.md'))
    for path in sorted(docs):
        relative = path.relative_to(root)
        content = read(relative)
        if not content.endswith('\n'):
            errors.append(f'{relative}: missing final newline')
        if any(line.rstrip() != line for line in content.splitlines()):
            errors.append(f'{relative}: trailing whitespace')
        if len(re.findall(r'^```', content, re.M)) % 2:
            errors.append(f'{relative}: unbalanced fence')
        for target in re.findall(r'\[[^\]]*\]\(([^)]+)\)', content):
            if re.match(r'^[a-z]+:', target, re.I) or target.startswith('#'):
                continue
            resolved = (path.parent / target.split('#', 1)[0]).resolve()
            if not resolved.is_relative_to(root) or not resolved.exists():
                errors.append(f'{relative}: missing or escaping local link {target}')
    summary = f'{len(role_names)} roles, {len(SKILLS)} skills, {len(contracts)} contracts, {len(docs)} documents'
    summary += f'; active task: {active[0] if active else "none"}'
    return errors, summary


def main():
    errors, summary = validate(Path(__file__).resolve().parents[1])
    if errors:
        print('\n'.join(errors), file=sys.stderr)
        return 1
    print(f'Foundation wiring: {summary}')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
