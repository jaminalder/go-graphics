#!/usr/bin/env python3
"""Build every docs Markdown page as a portable, checked browser edition."""
from pathlib import Path
from html.parser import HTMLParser
from urllib.parse import urlsplit, unquote
from urllib.request import urlopen
import argparse
import hashlib
import html
import json
import os
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
DOCS = ROOT / "docs"
OUT = ROOT / "out/docs"
MERMAID_VERSION = "11.12.0"
MERMAID_SHA256 = "07e37dfa97b337ccc85365d57eddf99b9706f09db3b59b260d0333b23b343c4b"
MERMAID_URL = f"https://cdn.jsdelivr.net/npm/mermaid@{MERMAID_VERSION}/dist/mermaid.min.js"


def pandoc(text, *args):
    executable = shutil.which("pandoc")
    if not executable:
        raise SystemExit("Install Pandoc: https://pandoc.org/installing.html")
    return subprocess.run([executable, *args], input=text, text=True,
                          capture_output=True, check=True).stdout


def walk(value, fn):
    if isinstance(value, dict):
        value = fn(value)
        return {k: walk(v, fn) for k, v in value.items()}
    if isinstance(value, list):
        return [walk(v, fn) for v in value]
    return value


def local_target(source, url):
    parts = urlsplit(url)
    if parts.scheme or parts.netloc:
        return None
    target = (source.parent / unquote(parts.path)).resolve() if parts.path else source
    if not target.is_relative_to(ROOT):
        raise ValueError(f"Local link escapes repository: {source}: {url}")
    return target


def document_info(document):
    headings = set()
    links = []
    def collect(node):
        if node.get("t") == "Header":
            headings.add(node["c"][1][0])
        if node.get("t") in ("Link", "Image"):
            links.append(node["c"][2][0])
        return node
    walk(document, collect)
    return headings, links


def read_documents():
    return {p: json.loads(pandoc(p.read_text(), "-f", "gfm", "-t", "json"))
            for p in sorted(DOCS.rglob("*.md"))}


def validate_sources(documents):
    info = {p: document_info(d) for p, d in documents.items()}
    errors = []
    for source, (_, links) in list(info.items()):
        for url in links:
            try:
                target = local_target(source, url)
            except ValueError as error:
                errors.append(str(error))
                continue
            if target is None:
                continue
            if not target.is_file():
                errors.append(f"{source.relative_to(ROOT)}: missing file: {url}")
                continue
            fragment = unquote(urlsplit(url).fragment)
            if fragment and target.suffix.lower() == ".md":
                if target not in info:
                    info[target] = document_info(json.loads(pandoc(target.read_text(), "-f", "gfm", "-t", "json")))
                if fragment not in info[target][0]:
                    errors.append(f"{source.relative_to(ROOT)}: missing heading: {url}")
    if errors:
        raise ValueError("\n".join(errors))


class PageLinks(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []
        self.ids = set()
    def handle_starttag(self, tag, attrs):
        values = dict(attrs)
        if "id" in values:
            self.ids.add(values["id"])
        for key in ("href", "src"):
            if key in values:
                self.links.append(values[key])


def validate_html(directory):
    directory = directory.resolve()
    pages = {}
    for page in directory.rglob("*.html"):
        parser = PageLinks()
        parser.feed(page.read_text())
        pages[page] = parser
    errors = []
    for page, parser in pages.items():
        for url in parser.links:
            parts = urlsplit(url)
            if parts.scheme or parts.netloc:
                continue
            target = (page.parent / unquote(parts.path)).resolve() if parts.path else page
            if not target.is_relative_to(directory.resolve()) or not target.is_file():
                errors.append(f"{page.relative_to(directory)}: broken generated link: {url}")
            elif parts.fragment and target in pages and unquote(parts.fragment) not in pages[target].ids:
                errors.append(f"{page.relative_to(directory)}: broken generated anchor: {url}")
    if errors:
        raise ValueError("\n".join(errors))


def title(source):
    return next((line[2:].strip() for line in source.read_text().splitlines() if line.startswith("# ")), source.stem)


def rel(target, output):
    return Path(os.path.relpath(target, output.parent)).as_posix()


def navigation(pages, output, source):
    groups = {}
    for path in pages:
        relative = path.relative_to(DOCS)
        group = relative.parts[0].title() if len(relative.parts) > 1 else "Overview"
        groups.setdefault(group, []).append(path)
    content = ['<nav class="site-nav" aria-label="Documentation"><a href="' + rel(pages[DOCS / "README.md"], output) + '">Documentation home</a>']
    preferred = ("Overview", "Architecture", "Guides", "Reference", "Sketches", "Operations", "Development")
    for group in (*preferred, *sorted(set(groups) - set(preferred))):
        if group not in groups:
            continue
        if group == "Architecture":
            sequence = ["context", "containers", "components", "code", "runtime", "deployment", "model"]
            groups[group].sort(key=lambda p: sequence.index(p.stem) if p.stem in sequence else len(sequence))
        elif group == "Overview":
            groups[group].sort(key=lambda p: (p.name != "README.md", p.name))
        opened = " open" if source in groups[group] else ""
        content.append(f"<details{opened}><summary>{group}</summary><ul>")
        for path in groups[group]:
            current = ' aria-current="page"' if source == path else ""
            content.append(f'<li><a{current} href="{html.escape(rel(pages[path], output))}">{html.escape(title(path))}</a></li>')
        content.append("</ul></details>")
    return "".join(content) + "</nav>"


def mermaid_asset():
    cache = ROOT / "out/docs-cache" / f"mermaid-{MERMAID_VERSION}.min.js"
    cache.parent.mkdir(parents=True, exist_ok=True)
    if not cache.is_file():
        print(f"Downloading pinned Mermaid {MERMAID_VERSION}; subsequent builds work offline.")
        with urlopen(MERMAID_URL, timeout=30) as response:
            data = response.read()
        if hashlib.sha256(data).hexdigest() != MERMAID_SHA256:
            raise ValueError("Downloaded Mermaid checksum mismatch")
        cache.write_bytes(data)
    if hashlib.sha256(cache.read_bytes()).hexdigest() != MERMAID_SHA256:
        raise ValueError(f"Mermaid cache checksum mismatch: {cache}")
    return cache


def build(documents):
    javascript = mermaid_asset()
    OUT.parent.mkdir(parents=True, exist_ok=True)
    # Stage a complete tree so stale deleted pages cannot survive a rebuild.
    with tempfile.TemporaryDirectory(prefix="docs-build-", dir=OUT.parent) as tmp:
        stage = Path(tmp)
        assets = stage / "assets"
        assets.mkdir()
        for name in ("style.css", "app.js"):
            shutil.copyfile(HERE / name, assets / name)
        shutil.copyfile(javascript, assets / javascript.name)
        pages = {p: stage / ("index.html" if p == DOCS / "README.md" else p.relative_to(DOCS).with_suffix(".html")) for p in documents}
        for source, document in documents.items():
            output = pages[source]
            output.parent.mkdir(parents=True, exist_ok=True)
            def copy_source(target):
                destination = stage / "source" / target.relative_to(ROOT)
                if destination.suffix.lower() in (".html", ".htm"):
                    destination = destination.with_suffix(destination.suffix + ".txt")
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(target, destination)
                return destination
            def adapt(node):
                if node.get("t") in ("Link", "Image"):
                    url = node["c"][2][0]
                    target = local_target(source, url)
                    if target:
                        parts = urlsplit(url)
                        mapped = pages.get(target)
                        if mapped is None:
                            mapped = copy_source(target)
                        node["c"][2][0] = rel(mapped, output)
                        if parts.query:
                            node["c"][2][0] += "?" + parts.query
                        if parts.fragment:
                            node["c"][2][0] += "#" + parts.fragment
                if node.get("t") == "CodeBlock" and "mermaid" in node["c"][0][1]:
                    return {"t": "RawBlock", "c": ["html", '<div class="diagram" tabindex="0" aria-label="Architecture diagram; scroll horizontally if needed"><pre class="mermaid">' + html.escape(node["c"][1]) + '</pre></div>']}
                return node
            adapted = walk(document, adapt)
            template = (HERE / "template.html").read_text()
            replacements = {
                "@ASSETS@": rel(assets, output), "@HOME@": rel(stage / "index.html", output),
                "@SOURCE@": rel(copy_source(source), output), "@MERMAID@": javascript.name,
                "@NAVIGATION@": navigation(pages, output, source),
            }
            for key, value in replacements.items():
                template = template.replace(key, value)
            temp_template = stage / "template.html"
            temp_template.write_text(template)
            output.write_text(pandoc(json.dumps(adapted), "-f", "json", "-t", "html5", "--standalone",
                                     "--section-divs", "--toc", "--toc-depth=2", "--template", str(temp_template),
                                     "--metadata", "pagetitle=" + title(source) + " — Singular Seed"))
        (stage / "template.html").unlink()
        validate_html(stage)
        if OUT.exists():
            shutil.rmtree(OUT)
        shutil.copytree(stage, OUT)
    print(f"Built and link-checked {len(documents)} pages: {OUT / 'index.html'}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Check Markdown destinations and headings without building/downloading assets")
    args = parser.parse_args()
    documents = read_documents()
    validate_sources(documents)
    print(f"Validated {len(documents)} Markdown pages and their local links/anchors.")
    if not args.check:
        build(documents)


if __name__ == "__main__":
    main()
