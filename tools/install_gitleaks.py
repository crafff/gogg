"""Install a checksum-pinned official Linux x64 Gitleaks build inside .local."""
import hashlib
import io
import os
from pathlib import Path
import platform
import tarfile
import tempfile
from urllib.request import urlopen

VERSION = '8.30.1'
URL = 'https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_linux_x64.tar.gz'
SHA256 = '551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb'


def main():
    if platform.system() != 'Linux' or platform.machine() not in ('x86_64', 'AMD64'):
        raise SystemExit('This pinned installer supports Linux x64; use an official build and GITLEAKS_BIN elsewhere.')
    root = Path(__file__).resolve().parents[1]
    directory = root / '.local/tools/gitleaks' / VERSION
    for component in (*reversed(directory.parents), directory):
        if component.is_symlink():
            raise SystemExit('Refusing a symlink installation path')
    with urlopen(URL, timeout=60) as response:
        data = response.read(32 * 1024 * 1024 + 1)
    if hashlib.sha256(data).hexdigest() != SHA256:
        raise SystemExit('Official release checksum mismatch; nothing installed')
    directory.mkdir(parents=True, mode=0o700, exist_ok=True)
    with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
        for name in ('gitleaks', 'LICENSE'):
            member = archive.getmember(name)
            if not member.isfile() or member.size > 64 * 1024 * 1024:
                raise SystemExit('Unexpected release member; refusing extraction')
            target = directory / name
            if target.is_symlink():
                raise SystemExit('Refusing to overwrite a symlink')
            descriptor, temporary = tempfile.mkstemp(prefix='.install-', dir=directory)
            try:
                with os.fdopen(descriptor, 'wb') as stream:
                    stream.write(archive.extractfile(member).read())
                    stream.flush()
                    os.fsync(stream.fileno())
                os.chmod(temporary, 0o700 if name == 'gitleaks' else 0o600)
                os.replace(temporary, target)
            finally:
                Path(temporary).unlink(missing_ok=True)
    print(f'Installed verified Gitleaks {VERSION} in {directory}')


if __name__ == '__main__':
    main()
