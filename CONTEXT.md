# Generative art and exploration

The project creates generative artworks and lets people explore their possible
outcomes. This vocabulary covers both local artistic practice and the proposed
public experience.

## Artworks

**Sketch**:
An executable generative artwork algorithm or an internal visual study.
_Avoid_: Product, published artwork (publication is a separate choice).

**Art form**:
A recognisable family of generative images that a visitor can choose visually.
_Avoid_: Template, filter, effect.

**Published artwork**:
An artist-selected art form offered for public exploration, with a defined
identity, permitted choices, and examples.
_Avoid_: Every registered sketch.

**Output space**:
The possible outcomes of an artwork, including both familiar examples and
unexpected combinations.

**Trait**:
A named dimension of an artwork's output space, such as arrangement or grain,
whose values describe families of outcomes.

**Style**:
A recognisable direction within an art form, illustrated by examples and
expressed through a coordinated set of artistic choices.
_Avoid_: A single finished image, CSS style.

**Colourway**:
A named colour direction for an artwork, including how its colours relate to
the ground and to one another.
_Avoid_: Palette (a palette supplies colours; a colourway interprets them).

**Seed**:
The value that selects the random decisions within a configured artwork.
_Avoid_: Artwork identity (a seed alone does not identify the artwork).

**Recipe**:
The complete instructions identifying a particular generated artwork,
including its artwork edition, seed, and artistic choices.
_Avoid_: Seed alone, filename.

**Edition**:
A named, fixed interpretation of a published artwork's recipes.
_Avoid_: Software release (the application can change without changing an edition).

**Rendition**:
An image of a recipe at a particular size and quality.
_Avoid_: New artwork (different rendition sizes represent the same work).

## Exploration

**Visual example**:
An artist-selected rendition that helps a visitor understand an art form or
choice before generating their own samples.

**Exploration**:
A visitor's evolving choices, generated samples, and favourites within one
published artwork.
_Avoid_: Account, collection (neither is required for exploration).

**Batch**:
A small group of samples generated for comparison in one exploration step.

**Sample**:
A candidate recipe and its visible rendition offered for the visitor's judgment.
_Avoid_: Pixel sample when discussing the public experience.

**Favourite**:
A sample the visitor has explicitly chosen to keep during exploration.
_Avoid_: Training label, public endorsement.

**Pin**:
An explicit artistic choice that remains fixed while other choices can vary.
_Avoid_: Preference (a preference guides suggestions but need not fix a choice).

**Refinement**:
A new exploration step guided by selected samples or explicit choices.
_Avoid_: Guaranteed improvement, seed interpolation.
