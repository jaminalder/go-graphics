"""Exercise activation control flow in a temporary filesystem with fake host commands.

This verifies failure ordering, not systemd, TLS, checksums or a real deployment.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1] / "scripts" / "activate-release.sh"


class Activation(unittest.TestCase):
    def scenario(self, previous=False, failure=""):
        with tempfile.TemporaryDirectory(prefix="art-activation-") as directory:
            root = Path(directory).resolve()
            art = root / "opt/art"
            etc = root / "etc"
            commands = root / "commands"
            commands.mkdir()
            (etc / "art").mkdir(parents=True)
            (etc / "systemd/system").mkdir(parents=True)
            (etc / "caddy").mkdir()
            (etc / "art/launch-approved").touch()
            (etc / "art/domain.env").write_text("ART_DOMAIN=example.test\n")
            (etc / "art/web.env").write_text("ART_ORIGIN=https://example.test\n")
            (etc / "caddy/Caddyfile").write_text("old proxy")
            for revision in ("a" * 40, "b" * 40):
                release = art / "releases" / revision
                (release / "deploy/scripts").mkdir(parents=True)
                (release / "deploy/systemd").mkdir()
                (release / "deploy/caddy").mkdir()
                for name in ("artweb", "artrender"):
                    binary = release / name
                    binary.write_text("#!/bin/sh\nexit 0\n")
                    binary.chmod(0o755)
                    (release / "deploy/systemd" / (name + ".service")).write_text(revision)
                (release / "deploy/caddy/Caddyfile").write_text(revision)
                smoke = release / "deploy/scripts/smoke-release.sh"
                smoke.write_text('#!/bin/sh\necho smoke >> "$EVENTS"\n[ "$FAILURE" != smoke ]\n')
                smoke.chmod(0o755)
            if previous:
                (art / "current").symlink_to(art / "releases" / ("a" * 40))
            stub = '''#!/bin/sh
name=${0##*/}
echo "$name $*" >> "$EVENTS"
case "$name" in
 curl)
  case "$*" in
   *8181*) exit 0 ;;
   *8081/ready*) [ "$FAILURE" != ready ]; exit $? ;;
   *metrics*) echo 'art_jobs_queued 0'; echo 'art_jobs_running 0' ;;
  esac ;;
esac
exit 0
'''
            for name in ("caddy", "systemd-analyze", "systemctl", "curl", "sleep", "sha256sum"):
                command = commands / name
                command.write_text(stub)
                command.chmod(0o755)
            # Portable equivalent for the existing-target readlink -e used on Linux.
            readlink = commands / "readlink"
            readlink.write_text('#!/usr/bin/env python3\nimport os,sys\np=sys.argv[-1]\nif not os.path.exists(p): sys.exit(1)\nprint(os.path.realpath(p))\n')
            readlink.chmod(0o755)
            script = SOURCE.read_text().replace("/opt/art", str(art)).replace("/etc/", str(etc) + "/")
            script = "\n".join(line for line in script.splitlines() if not line.startswith("[[ $EUID"))
            local = root / "activate.sh"
            local.write_text(script)
            events = root / "events"
            result = subprocess.run(["bash", str(local), "b" * 40], env={**os.environ, "PATH": str(commands) + os.pathsep + os.environ["PATH"], "EVENTS": str(events), "FAILURE": failure}, capture_output=True, text=True)
            return result, events.read_text(), (art / "current").resolve(), art

    def test_success_enables_both_services_at_boot(self):
        result, events, current, art = self.scenario()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("systemctl enable artrender artweb", events)
        self.assertEqual(current, art / "releases" / ("b" * 40))

    def test_failed_smoke_does_not_pause_the_existing_release(self):
        result, events, current, art = self.scenario(previous=True, failure="smoke")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("generation/off", events)
        self.assertEqual(current, art / "releases" / ("a" * 40))

    def test_first_deploy_failure_stops_candidate_without_a_self_link(self):
        result, events, current, art = self.scenario(failure="ready")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("systemctl stop artweb artrender", events)
        self.assertIn("systemctl disable artweb artrender", events)
        self.assertNotEqual(current, art / "current")

    def test_failed_upgrade_restores_the_previous_release(self):
        result, events, current, art = self.scenario(previous=True, failure="ready")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(current, art / "releases" / ("a" * 40))
        self.assertGreaterEqual(events.count("systemctl restart artrender artweb"), 2)


if __name__ == "__main__":
    unittest.main()
