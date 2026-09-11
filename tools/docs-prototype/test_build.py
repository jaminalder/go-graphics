"""Behavior checks for portable links and complete document discovery."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("docs_build", Path(__file__).with_name("build.py"))
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


class DocumentationChecks(unittest.TestCase):
    def test_all_nested_markdown_is_discovered_even_without_incoming_links(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "nested").mkdir()
            (root / "README.md").write_text("# Home")
            (root / "nested/isolated.md").write_text("# Standalone")
            with patch.object(build, "DOCS", root):
                self.assertEqual(len(build.read_documents()), 2)

    def test_html_checker_rejects_missing_pages_and_missing_anchors(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            page = root / "index.html"
            page.write_text('<a href="missing.html">Missing</a>')
            with self.assertRaisesRegex(ValueError, "broken generated link"):
                build.validate_html(root)
            page.write_text('<a href="#missing">Missing heading</a>')
            with self.assertRaisesRegex(ValueError, "broken generated anchor"):
                build.validate_html(root)
            page.write_text('<a href="#present">Heading</a><h1 id="present">Present</h1>')
            build.validate_html(root)

    def test_relative_source_link_cannot_escape_repository(self):
        with self.assertRaisesRegex(ValueError, "escapes repository"):
            build.local_target(build.ROOT / "docs/README.md", "../../../outside.txt")
        self.assertIsNone(build.local_target(build.ROOT / "docs/README.md", "https://c4model.com/"))


if __name__ == "__main__":
    unittest.main()
