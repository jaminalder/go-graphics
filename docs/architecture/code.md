# Code view: recipe identity and rendering contracts

Scope: the Recipe execution component in the render child. The prose connects its contract to public job admission. Audience: developers changing configuration, caching or protocol validation.

```mermaid
---
title: "Code view: recipe identity and rendering contracts"
---
classDiagram
    class Recipe {
        -string canonical
        +Bytes() byte[]
        +Digest() string
        +Key(Rendition, build) string
        +Render(writer, Rendition, build) error
    }
    class Rendition {
        +int Width
        +int Height
        +int AA
        +string Format
    }
    class envelope {
        +int Version
        +string ID
        +string Edition
        +string Seed
        +string Palette
        +RawMessage Config
    }
    Recipe ..> Rendition : includes in output identity
    Recipe ..> envelope : validates and stores canonical encoding
```

Key: this selective UML-style code diagram decomposes only the Recipe execution component. Boxes are Go types in `artwork`; dashed arrows are labeled code dependencies. Fields/operations are selective rather than complete Go signatures. There are no network semantics in these lines.

## Validation order

`artwork.Decode` rejects unsupported version/edition, malformed seeds, unknown palettes and invalid concrete configs. It resolves complete traits, normalizes the decimal seed and palette, and re-encodes a canonical record. `publish.Validate` then restricts that record to the public trait space and curated palette list; rebuilding an allowed record and comparing canonical bytes rejects private numeric overrides.

`Request.Validate` checks protocol version and an exact nonempty renderer build before applying publication and tier validation. Both ends of the private transport enforce that contract. `Manager.Admit` derives each key from canonical recipe plus serialized rendition plus build, validates a whole batch, and reserves capacity atomically. The `Renderer` interface allows queue tests to use controlled renderers without invoking expensive artwork code.

`Recipe.Bytes` and `Traits` return owned copies. The registry creates fresh mutable sketch objects; a shared registry lookup does not leak CLI settings into another render. Tests defend canonicalization, override isolation, identity, admission, cancellation and transport failure.

## Source

[Recipe and rendition](../../internal/artwork/recipe.go), [public validation](../../internal/publish/catalog.go), [admission](../../internal/renderjob/manager.go), [renderer interface and client](../../internal/renderjob/protocol.go), [recipe tests](../../internal/artwork/recipe_test.go). See [data reference](../reference/data.md) for the persisted encodings and [package reference](../reference/packages.md) for the rest of the codebase.
