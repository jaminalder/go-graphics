#!/usr/bin/env python3
"""Check target isolation and Docker routing without sending load."""
import argparse
import io
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("load", Path(__file__).resolve().parents[1] / "scripts/load.py")
load = importlib.util.module_from_spec(spec)
spec.loader.exec_module(load)


class LoadTests(unittest.TestCase):
    def test_user_limit_matches_scenario_validation(self):
        for users in (201, 500, 2000):
            self.assertEqual(load.parser().parse_args(["--users", str(users)]).users, users)
        for users in ("0", "2001", "1.5"):
            with patch("sys.stderr", new=io.StringIO()), self.assertRaises(SystemExit):
                load.parser().parse_args(["--users", users])

    def test_duration_and_origin_reject_unsafe_or_unbounded_inputs(self):
        self.assertEqual(load.duration("5m"), 300)
        for value in ("0s", "2d", "999h", "infinite"):
            with self.assertRaises(argparse.ArgumentTypeError):
                load.duration(value)
        for value in ("https://user:secret@example.test", "https://example.test/path", "https://example.test/?token=x"):
            with self.assertRaises(argparse.ArgumentTypeError):
                load.origin(value)

    def test_remote_load_requires_explicit_opt_in(self):
        with patch("sys.stderr", new=io.StringIO()), self.assertRaises(SystemExit) as e:
            load.main(["--target", "https://example.test", "--dry-run"])
        self.assertEqual(e.exception.code, 2)

    def test_loopback_dial_override_preserves_canonical_url_and_port(self):
        containers = [
            {"Config": {"Env": ["ART_ORIGIN=http://127.0.0.1:8280"], "Labels": {"com.docker.compose.service": "web"}}},
            {"Config": {"Env": [], "Labels": {"com.docker.compose.service": "caddy"}}, "NetworkSettings": {"Networks": {"art-local_edge": {"IPAddress": "172.28.0.2"}}}},
        ]
        with patch.object(load, "docker", side_effect=["web caddy", json.dumps(containers)]):
            self.assertEqual(load.discover("art-local"), ("http://127.0.0.1:8280", "art-local_edge", {"127.0.0.1:8280": "172.28.0.2:80"}))

    def test_local_classification_does_not_trust_similar_hostnames(self):
        self.assertTrue(load.local_origin("http://127.0.0.1:8280"))
        self.assertTrue(load.local_origin("http://localhost:8280"))
        self.assertFalse(load.local_origin("https://localhost.example.test"))

    def test_server_evidence_uses_server_clock_and_labels_window_scope(self):
        start = {"at": "2026-09-24T10:00:00Z", "queue": {"running": 1}, "instances": [], "limits": {"profile": "local-capacity"}}
        end = {**start, "at": "2026-09-24T10:00:30Z"}
        with tempfile.TemporaryDirectory() as temp, patch.object(load, "private_command", side_effect=[start, end, []]) as command, patch("sys.stdout", new=io.StringIO()):
            evidence = load.ServerEvidence("local", Path(temp))
            evidence.sample()
            evidence.finish()
            report = json.loads((Path(temp) / "server.json").read_text())
            self.assertTrue(report["available"])
            self.assertIn("including unrelated clients", report["scope"])
            self.assertEqual(command.call_args.args, ("local", "load-stats", "--since", start["at"], "--until", end["at"]))

    def test_remote_evidence_is_explicitly_unavailable_not_zero_work(self):
        with tempfile.TemporaryDirectory() as temp, patch("sys.stdout", new=io.StringIO()):
            load.ServerEvidence(None, Path(temp)).finish()
            report = json.loads((Path(temp) / "server.json").read_text())
            self.assertFalse(report["available"])
            self.assertNotIn("timings", report)


if __name__ == "__main__":
    unittest.main()
