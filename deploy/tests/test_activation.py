"""Exercise release ordering and rollback with fake Docker operations.

Real containers, sockets, rendering and limits are covered by verify-compose.sh.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"


class Activation(unittest.TestCase):
    def scenario(self, previous=False, failure="", environment="production", approved=True):
        with tempfile.TemporaryDirectory(prefix="art-activation-") as directory:
            root = Path(directory).resolve()
            art, etc, commands = root / "opt/art", root / "etc", root / "commands"
            commands.mkdir()
            (etc / "art").mkdir(parents=True)
            (root / "run/lock").mkdir(parents=True)
            (etc / "art/operator.env").write_text(f"ART_ENVIRONMENT={environment}\nART_ORIGIN=https://example.test\n")
            if approved:
                (etc / ("art/staging-approved" if environment == "staging" else "art/launch-approved")).touch()
            for revision in ("a" * 40, "b" * 40):
                release = art / "releases" / revision
                (release / "deploy/scripts").mkdir(parents=True)
                (release / "images.env").write_text("ART_APP_IMAGE=sha256:" + "c" * 64 + "\nART_EDGE_IMAGE=sha256:" + "d" * 64 + "\n")
                wrapper = release / "deploy/scripts/compose-release.sh"
                wrapper.write_text((SCRIPTS / "compose-release.sh").read_text().replace("/etc/", str(etc) + "/"))
                wrapper.chmod(0o755)
                smoke = release / "deploy/scripts/smoke-release.sh"
                smoke.write_text('#!/bin/sh\necho smoke >> "$EVENTS"\n[ "$FAILURE" != smoke ]\n')
                smoke.chmod(0o755)
            if previous:
                (art / "current").symlink_to(art / "releases" / ("a" * 40))
            stub = r'''#!/bin/sh
name=${0##*/}
echo "$name $*" >> "$EVENTS"
case "$name:$*" in
 sha256sum:*) [ "$FAILURE" != checksum ]; exit $? ;;
 flock:*) [ "$FAILURE" != lock ]; exit $? ;;
 docker:*metrics*)
   [ "$FAILURE" != metrics ] || exit 1
   echo 'art_jobs_running 0'
   if [ "$FAILURE" = drain ]; then echo 'art_jobs_queued 1'; else echo 'art_jobs_queued 0'; fi ;;
 docker:*ready*)
   case "$*" in *bbbbbbbb*) [ "$FAILURE" != ready ]; exit $? ;; esac ;;
 curl:*) [ "$FAILURE" != https ]; exit $? ;;
esac
exit 0
'''
            for name in ("docker", "flock", "curl", "sleep", "sha256sum"):
                command = commands / name
                command.write_text(stub)
                command.chmod(0o755)
            readlink = commands / "readlink"
            readlink.write_text('#!/usr/bin/env python3\nimport os,sys\np=sys.argv[-1]\nif not os.path.exists(p): sys.exit(1)\nprint(os.path.realpath(p))\n')
            readlink.chmod(0o755)
            script = (SCRIPTS / "activate-release.sh").read_text().replace("/opt/art", str(art)).replace("/etc/", str(etc) + "/").replace("/run/lock", str(root / "run/lock"))
            script = "\n".join(line for line in script.splitlines() if not line.startswith("[[ $EUID"))
            local = root / "activate.sh"
            local.write_text(script)
            events = root / "events"
            result = subprocess.run(["bash", str(local), "b" * 40], env={**os.environ, "PATH": str(commands) + os.pathsep + os.environ["PATH"], "EVENTS": str(events), "FAILURE": failure}, capture_output=True, text=True)
            current = (art / "current").resolve() if (art / "current").is_symlink() else None
            return result, events.read_text() if events.exists() else "", current, art

    def test_success_selects_the_verified_release(self):
        result, events, current, art = self.scenario(previous=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertLess(events.index("generation-off"), events.index("stop web renderer"))
        self.assertEqual(current, art / "releases" / ("b" * 40))
        self.assertIn("--pull never", events)

    def test_failed_smoke_does_not_pause_existing_generation(self):
        result, events, current, art = self.scenario(previous=True, failure="smoke")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("generation-off", events)
        self.assertEqual(current, art / "releases" / ("a" * 40))

    def test_first_deploy_failure_removes_candidate_pointer(self):
        result, events, current, _ = self.scenario(failure="ready")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(" down", events)
        self.assertNotIn("--volumes", events)
        self.assertIsNone(current)

    def test_failed_upgrade_restores_previous_images(self):
        for failure in ("ready", "https"):
            with self.subTest(failure=failure):
                result, events, current, art = self.scenario(previous=True, failure=failure)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(current, art / "releases" / ("a" * 40))
                self.assertGreaterEqual(events.count(" up -d"), 2)

    def test_failed_drain_does_not_replace_running_services(self):
        for failure in ("metrics", "drain"):
            with self.subTest(failure=failure):
                result, events, current, art = self.scenario(previous=True, failure=failure)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("stop web renderer", events)
                self.assertIn("generation-on", events)
                self.assertEqual(current, art / "releases" / ("a" * 40))

    def test_checksums_and_deploy_lock_precede_docker_mutation(self):
        for failure in ("checksum", "lock"):
            result, events, _, _ = self.scenario(failure=failure)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("docker", events)

    def test_staging_approval_is_separate_from_publication(self):
        result, _, _, _ = self.scenario(environment="staging")
        self.assertEqual(result.returncode, 0, result.stderr)
        result, events, _, _ = self.scenario(approved=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("docker", events)


if __name__ == "__main__":
    unittest.main()
