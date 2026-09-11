"""Exercise the root installer with real archive/checksum tools in Ubuntu.

All installation paths point into a temporary directory; activation is a stub.
"""
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class InstallRelease(unittest.TestCase):
    def scenario(self, fail=False, corrupt=False, origin="http://192.0.2.1"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            release = root / "fixture"
            scripts = release / "deploy/scripts"
            scripts.mkdir(parents=True)
            etc = root / "etc/art"
            etc.mkdir(parents=True)
            old_env = "ART_ORIGIN=http://192.0.2.9\n"
            (etc / "operator.env").write_text(old_env)
            activate = scripts / "activate-release.sh"
            activate.write_text("#!/bin/sh\nexit " + ("17" if fail else "0") + "\n")
            activate.chmod(0o755)
            revision = "a" * 40
            (release / "manifest.txt").write_text(f"source={revision}\nplatform=linux/amd64\n")
            (release / "SHA256SUMS").write_text("".join(f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.relative_to(release)}\n" for p in release.rglob("*") if p.is_file()))
            archive = root / "release.tar"
            with tarfile.open(archive, "w") as bundle:
                for entry in release.iterdir():
                    bundle.add(entry, arcname=entry.name)
            digest = "0" * 64 if corrupt else hashlib.sha256(archive.read_bytes()).hexdigest()
            script = (ROOT / "deploy/scripts/install-release.sh").read_text()
            for path in ("/opt/art", "/etc/art", "/run/lock"):
                target = root / path.lstrip("/")
                target.mkdir(parents=True, exist_ok=True)
                script = script.replace(path, str(target))
            installer = root / "install.sh"
            installer.write_text(script)
            result = subprocess.run(["bash", str(installer), str(archive), digest, revision, origin], capture_output=True, text=True)
            env = (etc / "operator.env").read_text()
            if fail or corrupt:
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual(env, old_env)
            else:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn(f"ART_ORIGIN={origin}\n", env)
                self.assertIn(f"ART_DOMAIN={origin}\n", env)
                self.assertEqual((etc / "operator.env").stat().st_mode & 0o777, 0o600)
                self.assertTrue((etc / "deployment-approved").exists())

    def test_http_install_records_selected_deployment_without_manual_approval(self):
        self.scenario()

    def test_https_install_records_selected_deployment_without_manual_approval(self):
        self.scenario(origin="https://example.test")

    def test_activation_failure_restores_previous_environment(self):
        self.scenario(fail=True)

    def test_corrupt_upload_does_not_change_environment(self):
        self.scenario(corrupt=True)


if __name__ == "__main__":
    if os.environ.get("ART_CLOUD_INIT_TEST_CONTAINER") != "1":
        raise SystemExit("Run inside the cloud-init test container")
    unittest.main()
