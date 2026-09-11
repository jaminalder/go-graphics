// Three layouts of the same generated document. Throwaway UI; no saved state.
const variants = ['A', 'B', 'C'];
const names = { A: 'Editorial · sidebar + full guide', B: 'Reading · continuous chapter', C: 'Reference · expandable sections' };
const label = document.querySelector('#variant-label');
const chapters = [...document.querySelectorAll('main section.level2')];
let variant;

// The reference view changes document structure, not only its palette.
for (const section of chapters) {
  const heading = section.querySelector(':scope > h2');
  if (!heading) continue;
  const details = document.createElement('details');
  const summary = document.createElement('summary');
  summary.textContent = heading.textContent;
  const content = document.createElement('div');
  content.className = 'reference-content';
  while (heading.nextSibling) content.append(heading.nextSibling);
  details.append(summary, content);
  section.append(details);
}

function revealAnchor() {
  if (!location.hash) return;
  const target = document.getElementById(decodeURIComponent(location.hash.slice(1)));
  if (!target) return;
  const details = target.matches('section') ? target.querySelector(':scope > details') : target.closest('details');
  if (details) details.open = true;
  target.scrollIntoView();
}

function showVariant(next, updateURL = true) {
  variant = variants.includes(next) ? next : 'A';
  document.body.dataset.variant = variant;
  label.textContent = `${variant} · ${names[variant]}`;
  for (const section of chapters) {
    const details = section.querySelector(':scope > details');
    if (details) details.open = variant !== 'C';
  }
  if (updateURL) {
    const url = new URL(location.href);
    url.searchParams.set('variant', variant);
    // file: history replacement works in Chromium/Safari; reload is a fallback.
    try { history.replaceState(null, '', url); } catch { location.href = url; }
  }
  revealAnchor();
}

function cycle(direction) {
  showVariant(variants[(variants.indexOf(variant) + direction + variants.length) % variants.length]);
}

document.querySelector('#previous').addEventListener('click', () => cycle(-1));
document.querySelector('#next').addEventListener('click', () => cycle(1));
document.addEventListener('keydown', event => {
  if (event.target.closest('input, textarea, select, [contenteditable], .diagram')) return;
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault();
    cycle(event.key === 'ArrowLeft' ? -1 : 1);
  }
});
window.addEventListener('hashchange', revealAnchor);
window.addEventListener('popstate', () => showVariant(new URLSearchParams(location.search).get('variant'), false));

// Render while all sections are open, so hidden diagrams have measurable width.
for (const details of document.querySelectorAll('main details')) details.open = true;
if (window.mermaid) {
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'base',
    themeVariables: { fontFamily: 'system-ui, sans-serif', fontSize: '16px', primaryColor: '#e7efea',
      primaryTextColor: '#23343b', primaryBorderColor: '#12695e', lineColor: '#12695e',
      secondaryColor: '#fffefa', tertiaryColor: '#f7f6f1' },
    flowchart: { htmlLabels: false, useMaxWidth: false, wrappingWidth: 200 } });
  mermaid.run({ querySelector: '.mermaid' }).then(() => {
    showVariant(new URLSearchParams(location.search).get('variant'), false);
  }).catch(error => {
    console.error('Diagram rendering failed', error);
    showVariant(new URLSearchParams(location.search).get('variant'), false);
  });
} else {
  showVariant(new URLSearchParams(location.search).get('variant'), false);
}
