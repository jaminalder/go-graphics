#!/usr/bin/env python3
"""Host name discovery survives scale/recreation without application Docker access."""
import importlib.util
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("watch", Path(__file__).resolve().parents[1] / "scripts/watch.py")
watch = importlib.util.module_from_spec(spec)
spec.loader.exec_module(watch)


def container(name, host, service, running=True):
    return {"Id": host, "Name": "/" + name, "Config": {"Hostname": host, "Labels": {"com.docker.compose.service": service}}, "State": {"Running": running}}


class WatchTests(unittest.TestCase):
    def test_local_and_hosted_names_refresh_after_scale_changes(self):
        for project in ("art-persistent-79509", "singular-seed"):
            with self.subTest(project=project):
                names = {}
                first = [container(project + "-web-1", "a", "web"), container(project + "-renderer-1", "b", "renderer")]
                second = [container(project + "-web-2", "c", "web"), container(project + "-renderer-2", "d", "renderer")]
                with patch.object(watch, "containers", side_effect=[first, second]), patch.object(watch, "docker", return_value="snapshot") as docker:
                    watch.refresh(project, names)
                    watch.refresh(project, names)
                    self.assertEqual(names["b"], project + "-renderer-1")
                    self.assertEqual(names["d"], project + "-renderer-2")
                    args = docker.call_args.args
                    self.assertIn("c", args)
                    self.assertEqual(json.loads(args[2].split("=", 1)[1]), names)

    def test_no_web_is_unavailable_not_an_old_snapshot(self):
        with patch.object(watch, "containers", return_value=[]):
            with self.assertRaises(RuntimeError):
                watch.refresh("singular-seed", {})

    def test_one_off_admin_commands_are_not_web_instances(self):
        oneoff = container("job", "abc", "web")
        oneoff["Config"]["Labels"]["com.docker.compose.oneoff"] = "True"
        with patch.object(watch, "containers", return_value=[oneoff]):
            with self.assertRaises(RuntimeError):
                watch.refresh("singular-seed", {})


if __name__ == "__main__":
    unittest.main()
