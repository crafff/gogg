"""Check an immutable staged candidate, then scan the original index; never commits."""
from pathlib import Path, PurePosixPath
import json
import os
import shutil
import subprocess
import sys
import tempfile


class PrecommitError(Exception):
    """A candidate cannot be checked safely."""


def git(root, *arguments, env=None, data=None):
    result = subprocess.run(
        ['git', *arguments], cwd=root, env=env, input=data,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
    )
    if result.returncode:
        raise PrecommitError('Git could not prepare or verify the staged candidate')
    return result.stdout


def scanner_path(root):
    configured = os.environ.get('GITLEAKS_BIN')
    search_path = os.pathsep.join(
        str(root / part) if not Path(part).is_absolute() else part
        for part in os.environ.get('PATH', os.defpath).split(os.pathsep)
    )
    if configured and '/' in configured:
        scanner = root / configured
    else:
        found = shutil.which(configured or 'gitleaks', path=search_path)
        scanner = Path(found) if found else None
        if scanner is None and not configured:
            scanner = root / '.local/tools/gitleaks/8.30.1/gitleaks'
    if scanner is None or not scanner.is_file() or not os.access(scanner, os.X_OK):
        raise PrecommitError('Secret scanner unavailable. Run make install-tools or set GITLEAKS_BIN.')
    return str(scanner.resolve())


def safe_path(name):
    path = PurePosixPath(name)
    parts = name.split('/')
    if path.is_absolute() or any(part in {'', '.', '..', '.git', '.local'} for part in parts):
        raise PrecommitError('Staged candidate contains an unsafe or private path')
    for part in parts:
        if (part in {'.claude', 'secrets.json', 'config.yaml'}
                or part == '.env' or (part.startswith('.env.') and part != '.env.example')
                or part.endswith(('.key', '.pem'))):
            raise PrecommitError('Staged candidate contains a private path')
    if parts[0] in {'data', 'artifacts', 'tmp'} or (
        parts[0] == 'legecy' and any(part in {'data', 'artifacts', 'tmp'} for part in parts[1:-1])
    ):
        raise PrecommitError('Staged candidate contains private runtime data')
    return path


def index_bytes(index):
    try:
        return index.read_bytes()
    except OSError as exc:
        raise PrecommitError('Cannot read the original Git index') from exc


def assert_unchanged(index, original):
    if index_bytes(index) != original:
        raise PrecommitError('Git index changed during checks; stage the intended candidate and rerun precommit')


def export_candidate(root, temporary, index, original, source_env):
    """Read regular blobs only: no checkout filters, archive rules or working files."""
    copied_index = temporary / 'index'
    copied_index.write_bytes(original)
    object_dir = temporary / 'objects'
    object_dir.mkdir()
    original_objects = os.fsdecode(git(root, 'rev-parse', '--git-path', 'objects', env=source_env)).strip()
    original_objects = str((root / original_objects).resolve())
    alternates = json.dumps(original_objects, ensure_ascii=False)
    if source_env.get('GIT_ALTERNATE_OBJECT_DIRECTORIES'):
        alternates += os.pathsep + source_env['GIT_ALTERNATE_OBJECT_DIRECTORIES']
    snapshot_env = dict(source_env, GIT_INDEX_FILE=str(copied_index),
                        GIT_OBJECT_DIRECTORY=str(object_dir),
                        GIT_ALTERNATE_OBJECT_DIRECTORIES=alternates)
    tree = git(root, 'write-tree', env=snapshot_env).decode('ascii').strip()
    assert_unchanged(index, original)
    entries = []
    for entry in git(root, 'ls-tree', '-rz', '--full-tree', tree, env=snapshot_env).split(b'\0'):
        if not entry:
            continue
        metadata, raw_name = entry.split(b'\t', 1)
        mode, kind, oid = metadata.split()
        if kind != b'blob' or mode not in {b'100644', b'100755'}:
            raise PrecommitError('Staged symlinks and submodules are unsupported; no files were checked')
        name = os.fsdecode(raw_name)
        safe_path(name)
        entries.append((name, mode, oid))
    required = {'Makefile', 'engineering/README.md', 'tools/check_foundation.py'}
    if not required.issubset({entry[0] for entry in entries}):
        raise PrecommitError('Staged candidate is missing the active engineering foundation; stage the required new-root files')
    candidate = temporary / 'candidate'
    candidate.mkdir()

    def export(entry):
        name, mode, oid = entry
        destination = candidate / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(git(root, 'cat-file', 'blob', oid.decode('ascii'), env=snapshot_env))
        destination.chmod(0o755 if mode == b'100755' else 0o644)

    # Evaluate staged ignore rules with the original info/exclude and global rules.
    # No source data outside Git objects is copied, including private local mounts.
    for entry in entries:
        if PurePosixPath(entry[0]).name == '.gitignore':
            export(entry)
    if entries:
        result = subprocess.run(
            ['git', '--work-tree=' + str(candidate), 'check-ignore', '--no-index', '--stdin', '-z'],
            cwd=root, env=source_env,
            input=b'\0'.join(os.fsencode(entry[0]) for entry in entries) + b'\0',
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
        )
        if result.returncode not in {0, 1}:
            raise PrecommitError('Cannot verify ignore rules for the staged candidate')
        if result.stdout:
            raise PrecommitError('Staged candidate contains Git-ignored paths; unstage private/generated files')
    for entry in entries:
        if PurePosixPath(entry[0]).name != '.gitignore':
            export(entry)
    # Validators may inspect Git ignore rules; this metadata is entirely disposable.
    check_env = {key: value for key, value in os.environ.items()
                 if not key.startswith('GIT_') and key not in {
                     'MAKEFLAGS', 'GNUMAKEFLAGS', 'MAKEFILES', 'MFLAGS', 'MAKELEVEL', 'PYTHONPATH',
                 }}
    git(candidate, '-c', 'init.templateDir=', 'init', '--quiet', env=check_env)
    print(f'Checking staged tree {tree}', flush=True)
    return candidate, check_env


def main():
    root = Path(__file__).resolve().parents[1]
    try:
        scanner = scanner_path(root)
        source_env = dict(os.environ, GIT_OPTIONAL_LOCKS='0')
        actual_root = os.fsdecode(git(root, 'rev-parse', '--show-toplevel', env=source_env)).strip()
        if Path(actual_root).resolve() != root:
            raise PrecommitError('Git context does not match the project root')
        index_name = os.fsdecode(git(root, 'rev-parse', '--git-path', 'index', env=source_env)).strip()
        index = (root / index_name).resolve()
        original = index_bytes(index)
        with tempfile.TemporaryDirectory(prefix='gogg-staged-') as directory:
            candidate, check_env = export_candidate(root, Path(directory), index, original, source_env)
            result = subprocess.run(['make', 'check'], cwd=candidate, env=check_env, check=False)
            assert_unchanged(index, original)
            if result.returncode:
                return result.returncode
            for command in (
                ['git', 'diff', '--cached', '--check', '--', '.', ':(exclude)legecy'],
                [scanner, 'git', '--staged', '--redact', '--no-banner', str(root)],
            ):
                assert_unchanged(index, original)
                result = subprocess.run(command, cwd=root, env=source_env, check=False)
                assert_unchanged(index, original)
                if result.returncode:
                    return result.returncode
        return 0
    except (PrecommitError, OSError) as exc:
        print(str(exc), file=sys.stderr)
        return 2


if __name__ == '__main__':
    raise SystemExit(main())
