"""Run only inside the disposable Ubuntu image from cloud-init.Dockerfile.

Uses real cloud-init account/file modules and an SSH login over container
loopback. No host keys, credentials, cloud APIs or published ports are used.
"""
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
from types import SimpleNamespace
import unittest

if os.environ.get("ART_CLOUD_INIT_TEST_CONTAINER") != "1" or not Path("/.dockerenv").exists():
    raise SystemExit("Run this test through deploy/scripts/verify-cloud-init.sh")

import grp
import yaml
from cloudinit import helpers
from cloudinit.config import cc_users_groups, cc_write_files
from cloudinit.distros.ubuntu import Distro

ROOT = Path(__file__).resolve().parents[2]


class CloudInitAccess(unittest.TestCase):
    def test_operator_can_log_in_with_the_preexisting_ubuntu_group(self):
        # Ubuntu's operator group already exists. Without an explicit primary
        # group, useradd fails and the configured root lockout leaves no login.
        self.assertEqual(grp.getgrnam("operator").gr_gid, 37)
        with tempfile.TemporaryDirectory() as directory:
            key = Path(directory) / "identity"
            subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key)], check=True)
            template = (ROOT / "deploy/cloud-init/user-data.yaml").read_text()
            cfg = yaml.safe_load(template.replace("${ssh_public_key}", key.with_suffix(".pub").read_text().strip()))
            system_cfg = yaml.safe_load(Path("/etc/cloud/cloud.cfg").read_text())
            cloud = SimpleNamespace(
                distro=Distro("ubuntu", system_cfg["system_info"], helpers.Paths({})),
                paths=helpers.Paths({}),
                get_public_ssh_keys=lambda: [],
            )
            cc_users_groups.handle("users_groups", cfg, cloud, [])
            cc_write_files.handle("write_files", cfg, cloud, [])

            Path("/run/sshd").mkdir(exist_ok=True)
            subprocess.run(["ssh-keygen", "-A"], check=True, capture_output=True)
            settings = subprocess.check_output(["/usr/sbin/sshd", "-T"], text=True)
            self.assertIn("permitrootlogin no\n", settings)
            self.assertIn("passwordauthentication no\n", settings)

            with (Path(directory) / "sshd.log").open("w+") as log:
                daemon = subprocess.Popen(
                    ["/usr/sbin/sshd", "-D", "-e", "-p", "2222", "-o", "ListenAddress=127.0.0.1"],
                    stdout=log, stderr=log,
                )
                try:
                    for _ in range(50):
                        try:
                            with socket.create_connection(("127.0.0.1", 2222), timeout=0.1):
                                break
                        except OSError:
                            if daemon.poll() is not None:
                                self.fail("sshd exited before accepting connections")
                            time.sleep(0.1)
                    result = subprocess.run(
                        ["ssh", "-F", "/dev/null", "-o", "BatchMode=yes",
                         "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=accept-new",
                         "-o", f"UserKnownHostsFile={directory}/known_hosts",
                         "-i", str(key), "-p", "2222", "operator@127.0.0.1", "id -un"],
                        capture_output=True, text=True, timeout=10,
                    )
                    log.flush()
                    log.seek(0)
                    self.assertEqual(result.returncode, 0, result.stderr + log.read())
                    self.assertEqual(result.stdout.strip(), "operator")
                finally:
                    daemon.terminate()
                    daemon.wait(timeout=5)


if __name__ == "__main__":
    unittest.main()
