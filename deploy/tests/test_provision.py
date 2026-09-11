"""Artifact corruption must stop deployment; transient SSH failures can recover."""
import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location("provision", Path(__file__).resolve().parents[1] / "scripts/provision-app.py")
provision = importlib.util.module_from_spec(spec)
spec.loader.exec_module(provision)


class Provision(unittest.TestCase):
    def test_changed_or_unlisted_release_files_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("images.tar", "images.env", "deploy/scripts/activate-release.sh", "deploy/compose.yaml"):
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("fixture")
            (root / "manifest.txt").write_text("source=" + "a" * 40 + "\nplatform=linux/amd64\n")
            checks = root / "SHA256SUMS"
            checks.write_text("".join(f"{provision.checksum(p)}  {p.relative_to(root)}\n" for p in sorted(root.rglob("*")) if p.is_file()))
            digest = provision.checksum(checks)
            self.assertEqual(provision.verify_release(root, digest), "a" * 40)
            (root / "extra").touch()
            with self.assertRaisesRegex(ValueError, "not listed"):
                provision.verify_release(root, digest)
            (root / "extra").unlink()
            (root / "images.tar").write_text("corrupted")
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                provision.verify_release(root, digest)
            checks.write_text("changed after plan")
            with self.assertRaisesRegex(ValueError, "since terraform plan"):
                provision.verify_release(root, digest)

    def test_ssh_timeout_during_boot_can_recover(self):
        with patch.object(provision.subprocess, "run", side_effect=[subprocess.TimeoutExpired("ssh", 15), subprocess.CompletedProcess([], 0)]) as run, patch.object(provision.time, "sleep"):
            provision.wait_for_ssh(["ssh", "fixture"])
            self.assertEqual(run.call_count, 2)

    def test_bad_artifact_never_connects_to_host(self):
        settings = dict(ART_SERVER_IP="192.0.2.1", ART_SERVER_ID="123", ART_SITE_ORIGIN="http://192.0.2.1", ART_ENVIRONMENT="staging", ART_RELEASE_PATH="missing", ART_RELEASE_DIGEST="x")
        with patch.object(provision, "verify_release", side_effect=ValueError("corrupt")), patch.object(provision.subprocess, "run") as run:
            with self.assertRaisesRegex(ValueError, "corrupt"):
                provision.provision(settings)
            run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
