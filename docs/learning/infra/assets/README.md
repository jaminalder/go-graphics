# Learning components

All presentation and interaction stays inside `docs/learning/infra/`.

- `course.css`: shared layout, typography, native SVG styling, narrow-screen
  diagram scrolling and print layout. All pages link this file.
- `course.js`: progressive enhancement for scenario buttons, quiz feedback,
  recall notes, export and field-guide filtering. No libraries or network calls.
- Each HTML page owns its diagram geometry and teaching content. Use native SVG
  with a title/description and a text explanation; no CDN renderer is required.

Copy the structure of an existing lesson when adding one. Keep unique IDs per
page, use relative links, give every quiz alternative the same word count and
include an expandable answer key for readers without JavaScript. Scenario
buttons select `data-panel` sections and highlight `data-show-in` SVG groups.

Recall notes are optional and stored under `singular-seed-infra-learning-v1` in
browser localStorage. File URLs may have browser-specific storage isolation;
exported Markdown is the portable copy. Storage failure leaves the current
page usable. These notes and review dates are self-assessments, not launch
verification or automatic learning records.

When updating runtime claims, read the linked source and update the visible
snapshot date/revision. Review desktop, 390px/320px, print, keyboard interaction,
no-JavaScript and blocked-storage behaviour. Check local links and quiz feedback.
Keep screenshots and generated PDFs in the worktree’s ignored `out/` directory.
