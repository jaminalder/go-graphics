# A simpler exploration, 2026-09-10

The owner asked to preserve the gallery and direction chooser, make the next
page immediately useful, and move all further generation onto one chosen
image. The interface now follows that request: directions → four images → one
image, with favourites available independently.

Steve Krug's discussion of navigation in *Don't Make Me Think, Revisited*,
chapter 4, emphasises the mental effort and uncertainty in a choice rather
than treating raw click count as the objective. Our interpretation is to give
each screen a clear decision, use destination-specific labels, and explain
only what the visitor needs at that point. [Author's sample chapter, pp. 43–47](https://sensible.com/downloads/DMMT-Revisited-sample-chapter.pdf),
[author's book page](https://sensible.com/dont-make-me-think/).

Nielsen Norman Group's recognition-over-recall heuristic supports keeping the
chosen directions visible as miniatures and names. Visibility of system status
supports a quiet actual count as the four images finish. Aesthetic and
minimalist design supports removing history, storage/recovery explanations,
and competing generation actions from the comparison grid.
[Ten usability heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/).

Progressive disclosure puts the next level of interaction where it becomes
relevant: download, share and generate similar images belong beside one
selected image. The grid therefore offers only opening and favouriting; the
Favourites page offers opening and removing. This is an application of the
principle, not a requirement that every action be hidden behind a menu.
[Progressive disclosure](https://www.nngroup.com/articles/progressive-disclosure/).

“Similar” now has a deliberately narrow promise: the selected image supplies
all traits and its palette for four fresh composition seeds. Mixing an
unrelated base-space candidate into the four would contradict that label.
No favourite selection or invisible preference accumulation is required.

These sources informed design decisions. Browser tests and visual inspection
check implementation behaviour; no human usability study has been claimed.
