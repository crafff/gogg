"""Behavior tests for offline installer verification and the non-committing hook."""
from contextlib import contextmanager
import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

from tools import install_gitleaks


def release(link=False):
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode='w:gz') as archive:
        for name, content in [('gitleaks', b'test executable'), ('LICENSE', b'test license')]:
            info = tarfile.TarInfo(name)
            if link and name == 'gitleaks':
                info.type = tarfile.SYMTYPE
                info.linkname = '/outside'
                archive.addfile(info)
            else:
                info.size = len(content)
                archive.addfile(info, io.BytesIO(content))
    return buffer.getvalue()


class InstallTests(unittest.TestCase):
    @contextmanager
    def environment(self, root, data, checksum=None):
        with patch.object(install_gitleaks, '__file__', str(root / 'tools/install_gitleaks.py')), \
             patch.object(install_gitleaks.platform, 'system', return_value='Linux'), \
             patch.object(install_gitleaks.platform, 'machine', return_value='x86_64'), \
             patch.object(install_gitleaks, 'urlopen', return_value=io.BytesIO(data)), \
             patch.object(install_gitleaks, 'SHA256', checksum or hashlib.sha256(data).hexdigest()):
            yield

    def test_bad_checksum_does_not_install(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.environment(root, release(), checksum='0' * 64):
                with self.assertRaises(SystemExit):
                    install_gitleaks.main()
            self.assertFalse((root / '.local').exists())

    def test_installs_only_regular_verified_binary_and_license(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.environment(root, release()):
                install_gitleaks.main()
            installed = root / '.local/tools/gitleaks/8.30.1'
            self.assertEqual((installed / 'gitleaks').read_bytes(), b'test executable')
            self.assertEqual((installed / 'LICENSE').read_bytes(), b'test license')
            self.assertEqual((installed / 'gitleaks').stat().st_mode & 0o777, 0o700)
            self.assertEqual({p.name for p in installed.iterdir()}, {'gitleaks', 'LICENSE'})

    def test_release_symlink_cannot_be_installed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.environment(root, release(link=True)):
                with self.assertRaises(SystemExit):
                    install_gitleaks.main()
            self.assertFalse((root / '.local/tools/gitleaks/8.30.1/gitleaks').exists())

    def test_installation_symlink_is_rejected_before_download(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / '.local').symlink_to(root / 'outside', target_is_directory=True)
            with self.environment(root, release()):
                with self.assertRaises(SystemExit):
                    install_gitleaks.main()
            self.assertFalse((root / 'outside').exists())


if __name__ == '__main__':
    unittest.main()
