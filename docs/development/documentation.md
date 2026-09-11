# Building and maintaining the documentation

## Complete browser edition

The adopted formatting lives in `tools/docs-prototype`; its historical directory name is retained to keep the tool easy to find. The builder processes **every** Markdown file under `docs/`, not just pages reachable from the home page. Output goes to ignored `out/docs/`; `docs/README.md` becomes `index.html`, and other paths keep their shape with `.html` extensions.

```sh
make docs-check
make docs
```

Requirements are Python 3.9 or later and Pandoc on PATH. Mermaid 11.12.0 is pinned by version and SHA-256, downloaded from jsDelivr on the first uncached build, and cached in `out/docs-cache/`. Subsequent builds work offline. Keep the verified cache when using an offline machine; a mismatching cached/downloaded asset fails the build. No Python packages are installed.

Open `out/docs/index.html` directly or serve that directory with an ordinary static HTTP server. All pages have site navigation, page contents, source links, responsive tables/images and print styles. The layout control offers Editorial (sidebar), Reading (continuous chapter) and Reference (expandable sections); `?variant=A`, `B` or `C` selects a view and carries it across links. JavaScript renders Mermaid diagrams. Without JavaScript, the text and diagram source remain readable.

## Portable output and checks

Markdown links to documentation become relative HTML links. Linked images, datasets and explicitly referenced source files are copied into `out/docs/source/` with their repository paths; no whole-repository or secret-file export is performed. Markdown source is copied for each page. Linked HTML source templates are copied with a `.txt` suffix so browsers display source rather than executing an application template. Absolute `file:` URLs are not generated. The output can be moved as a directory and still read without the original checkout.

`make docs-check` runs behavior checks and validates all Markdown local destinations and fragment headings using Pandoc's parsed document identifiers. `make docs` also checks every generated local HTML link/anchor and script/style/image destination. External links are preserved and are not fetched by the link checker. Diagrams are rendered in the browser, so visual/console inspection is needed in addition to link checks. A successful rebuild replaces the generated tree, removing obsolete pages.

## Edit source, then rebuild

Use ordinary GitHub-flavored Markdown, relative links and fenced `mermaid` blocks. Each diagram states its title, scope, element types, technologies and key, following [C4 conventions](../architecture/model.md). Add a useful path from the [documentation home](../README.md); the builder's global navigation includes the page automatically. Keep facts linked to their owning source and put future plans outside the current documentation.

The per-sketch CLI help blocks are generated from the current application:

```sh
make docs-sketch-help
make docs-check
make docs
```

The refresh command builds a temporary CLI and changes only marked blocks. Algorithm prose and option interpretation remain reviewed documentation. Palette source datasets are consumed by code generators and must keep their format/path. See [source assets](../reference/source-assets.md).

Source: [builder](../../tools/docs-prototype/build.py), [template](../../tools/docs-prototype/template.html), [styles](../../tools/docs-prototype/style.css), [reading behavior](../../tools/docs-prototype/app.js), [checks](../../tools/docs-prototype/test_build.py).
