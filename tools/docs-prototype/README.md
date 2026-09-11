# Documentation format prototype

**Question:** can we keep the existing infrastructure guide's comfortable
browser presentation while maintaining its content and diagrams in Markdown?
This branch is a throwaway format experiment; no format has been selected yet.

## Try it

From the repository root:

```sh
make docs-prototype
open out/docs-prototype/index.html
```

Requires Python 3.9+ and [Pandoc](https://pandoc.org/installing.html)
(already installed on the author's machine). The first build downloads the
pinned Mermaid 11.12.0 browser bundle from jsDelivr into ignored `out/`.
Subsequent builds and viewing work offline while that cached asset exists.
Deleting `out/` means the next build needs the network again. No server,
Node installation or additional Go dependencies are needed.

Use the floating arrows or keyboard left/right arrows to compare:

| URL parameter | Layout | What it tests |
| --- | --- | --- |
| `?variant=A` | Editorial | Warm paper, green diagrams, persistent sidebar; closest to today's page. |
| `?variant=B` | Reading | Contents above a centred chapter; more generous prose and no sidebar. |
| `?variant=C` | Reference | Compact contents and expandable sections for looking things up. |

The switcher is intentionally confined to this prototype's generated output.
It is not a production document feature. Edit the Markdown and rebuild to
refresh the browser edition; there is no watcher or live preview server.

## What is maintained

- [SYSTEM.md](../../docs/learning/infra/SYSTEM.md): all nine sections of the
  existing guide, including its dated status snapshot, tables, commands and
  source links. Four hand-drawn SVGs become four `mermaid` fenced blocks.
- `template.html`, `style.css` and `app.js`: shared presentation, separate
  from documentation prose. No HTML is embedded in the new Markdown.
- `build.py`: a small Pandoc wrapper. It renders the guide and its eleven
  directly linked Markdown documents, rewrites links and copies assets.
- Generated HTML and downloaded JavaScript: only in ignored
  `out/docs-prototype/`; never edited or committed.

The original [index.html](../../docs/learning/infra/index.html) is retained
unchanged for side-by-side comparison. Once a format is approved, remove that
duplicate source rather than maintaining both versions. This prototype does
not audit or update infrastructure facts.

On [GitHub, Mermaid fences render natively](https://docs.github.com/en/get-started/writing-on-github/working-with-advanced-formatting/creating-diagrams).
Headings, tables, code and relative source links work as ordinary Markdown.
The custom sidebar, typography and variant switcher belong to the local
browser edition. GitHub controls its own theme and Mermaid version, so
pixel-identical diagrams across the two viewers are not expected.

## Small, useful choices

| Option | Maintenance and browser experience | Tradeoff |
| --- | --- | --- |
| **Pandoc + one shared template** (this prototype) | Plain Markdown + Mermaid, familiar custom appearance, generated static files that open directly in a browser. | Small local build wrapper to maintain; no site search. Best fit for testing this page and keeping the current look. |
| **MkDocs + Material** | Markdown pages with managed navigation, built-in search, theme and live development server. | Python packages, theme/configuration and Mermaid fence setup. Stronger choice if the goal becomes a whole documentation site. |
| **Docsify** | Markdown read dynamically by a browser shell; no HTML build step. | Local HTTP server and runtime JavaScript/plugins, including Mermaid setup. Less natural for today's double-click workflow. |

Sources: [Pandoc manual](https://pandoc.org/MANUAL.html),
[MkDocs](https://www.mkdocs.org/),
[Material diagrams](https://squidfunk.github.io/mkdocs-material/reference/diagrams/),
[Docsify](https://docsify.js.org/).
MkDocs can also emit direct-open pages with
[`use_directory_urls: false`](https://www.mkdocs.org/user-guide/configuration/#use_directory_urls);
offline search/assets need additional theme configuration. Pandoc is chosen
here for the small scope and existing installation, not because it is the
only tool that can produce standalone files.

## Prototype limits

- Browser diagrams need JavaScript; without it, their Mermaid source remains
  readable. Large diagrams and tables scroll horizontally on narrow screens.
- Only directly linked Markdown pages get an HTML edition. Other source links
  point to real files in this checkout (Markdown may appear as raw text or
  download, according to the browser); this is intentionally not a recursive
  whole-repository site generator.
- Source links use `file:` URLs, so rebuild after moving the checkout. Generated
  output is for local browsing, not publication to a web host.
- Content dates, operational commands and pending checks are copied from the
  original snapshot. Running the docs build executes none of those commands.
- The Mermaid bundle is version-pinned; GitHub's renderer is independently
  versioned. The build prints the downloaded bundle's SHA-256 for inspection.
