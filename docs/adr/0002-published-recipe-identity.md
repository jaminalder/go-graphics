---
status: proposed
---

# Published results have edition-scoped recipes

Identify a generated public artwork by a canonical recipe containing its
artwork edition, seed and effective artistic configuration. Identify an image
rendition additionally by output/encoding settings and renderer build. Local
experiments may continue changing seed output across revisions.

A public edition needs an explicit compatibility or retirement policy; storing
an edition label or source hash alone cannot regenerate an old algorithm.
Baseline v1 sharing is the downloaded image file. Durable artwork links are an
optional extension requiring retained compatible renderers or durable images,
with honest cache-miss and retirement behaviour.

This avoids making an accidental permanence promise while keeping future links
possible without an application database. See
[recipe identity and optional links](../web/architecture.md).
