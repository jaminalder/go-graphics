#!/usr/bin/env python3
"""Refresh only marked CLI-help blocks from current sketch definitions."""
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
with tempfile.TemporaryDirectory(prefix="staticart-help-") as directory:
    binary = str(Path(directory) / "staticart")
    subprocess.run(["go", "build", "-o", binary, "./cmd/staticart"], cwd=ROOT, check=True)
    for source in sorted((ROOT / "docs/sketches").glob("*.md")):
        text = source.read_text()
        marker = "<!-- sketch-help:start -->"
        if marker not in text:
            continue
        name = source.stem.split("-", 1)[1]
        if name == "contour-noise":
            name = "contour"
        result = subprocess.run([binary, "render", name, "--help"], text=True, capture_output=True)
        if "Usage of render:" not in result.stderr:
            raise SystemExit(f"Could not read help for {name}: {result.stderr}")
        help_text = result.stderr.replace("staticart: flag: help requested\n", "").expandtabs(8).strip()
        before, rest = text.split(marker, 1)
        _, after = rest.split("<!-- sketch-help:end -->", 1)
        source.write_text(before + marker + "\n```text\n" + help_text + "\n```\n<!-- sketch-help:end -->" + after)
        print(source.relative_to(ROOT))
