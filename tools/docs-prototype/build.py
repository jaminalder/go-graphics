#!/usr/bin/env python3
"""Throwaway Markdown → local HTML experiment. No Python dependencies."""
from pathlib import Path
import hashlib
import html
import json
import os
import shutil
import subprocess
from urllib.parse import urlsplit, unquote
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
OUT = ROOT / 'out/docs-prototype'
SOURCE = ROOT / 'docs/learning/infra/SYSTEM.md'
MERMAID_VERSION = '11.12.0'
MERMAID_URL = f'https://cdn.jsdelivr.net/npm/mermaid@{MERMAID_VERSION}/dist/mermaid.min.js'
PANDOC = shutil.which('pandoc')
if not PANDOC:
    raise SystemExit('Install Pandoc first: https://pandoc.org/installing.html')


def pandoc(text, *args):
    return subprocess.run([PANDOC, *args], input=text, text=True,
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
    if parts.scheme or parts.netloc or not parts.path:
        return None
    target = (source.parent / unquote(parts.path)).resolve()
    if not target.is_relative_to(ROOT):
        return None
    return target


OUT.mkdir(parents=True, exist_ok=True)
asset_dir = OUT / 'assets'
asset_dir.mkdir(exist_ok=True)
mermaid = asset_dir / f'mermaid-{MERMAID_VERSION}.min.js'
if not mermaid.exists():
    print(f'Downloading pinned Mermaid {MERMAID_VERSION} (cached for offline builds).')
    with urlopen(MERMAID_URL, timeout=60) as response:
        data = response.read()
    mermaid.write_bytes(data)
# Keep only direct Markdown destinations in scope; this is a format prototype.
doc = json.loads(pandoc(SOURCE.read_text(), '-f', 'gfm', '-t', 'json'))
pages = {SOURCE: OUT / 'index.html'}

def collect(node):
    if node.get('t') == 'Link':
        target = local_target(SOURCE, node['c'][2][0])
        if target and target.is_file() and target.suffix.lower() == '.md':
            pages[target] = OUT / 'pages' / target.relative_to(ROOT).with_suffix('.html')
    return node

walk(doc, collect)
for name in ['style.css', 'app.js']:
    shutil.copyfile(HERE / name, asset_dir / name)

for source, output in pages.items():
    output.parent.mkdir(parents=True, exist_ok=True)
    document = doc if source == SOURCE else json.loads(pandoc(source.read_text(), '-f', 'gfm', '-t', 'json'))

    def adapt(node):
        if node.get('t') in ['Link', 'Image']:
            url = node['c'][2][0]
            target = local_target(source, url)
            if target:
                parts = urlsplit(url)
                # HTML destinations are relative; source files use file: URLs.
                # This keeps direct-open browsing independent of output depth.
                mapped = pages.get(target)
                node['c'][2][0] = (os.path.relpath(mapped, output.parent) if mapped else target.as_uri())
                if parts.query:
                    node['c'][2][0] += '?' + parts.query
                if parts.fragment:
                    node['c'][2][0] += '#' + parts.fragment
        if node.get('t') == 'CodeBlock' and 'mermaid' in node['c'][0][1]:
            return {'t': 'RawBlock', 'c': ['html', '<div class="diagram" tabindex="0" aria-label="Diagram; scroll horizontally if needed"><pre class="mermaid">' + html.escape(node['c'][1]) + '</pre></div>']}
        return node

    document = walk(document, adapt)
    asset_path = os.path.relpath(asset_dir, output.parent)
    template = (HERE / 'template.html').read_text().replace('@ASSETS@', asset_path)
    template = template.replace('@HOME@', os.path.relpath(OUT / 'index.html', output.parent))
    template = template.replace('@SOURCE@', source.as_uri())
    template = template.replace('@ORIGINAL@', (ROOT / 'docs/learning/infra/index.html').as_uri())
    template = template.replace('@MERMAID@', mermaid.name)
    # Pandoc's template is presentation tooling; docs remain plain GFM.
    temp_template = OUT / 'template.html'
    temp_template.write_text(template)
    rendered = pandoc(json.dumps(document), '-f', 'json', '-t', 'html5', '--standalone',
                      '--section-divs', '--toc', '--toc-depth=2', '--template', str(temp_template),
                      '--metadata', 'pagetitle=Singular Seed — documentation prototype')
    output.write_text(rendered)
(OUT / 'template.html').unlink()
print(f'Built {len(pages)} pages. Open {OUT / "index.html"}')
print('Layouts: ?variant=A (Editorial), ?variant=B (Reading), ?variant=C (Reference).')
print(f'Mermaid SHA-256: {hashlib.sha256(mermaid.read_bytes()).hexdigest()}')
