'use strict';
document.documentElement.classList.add('js');
const storageKey = 'singular-seed-infra-learning-v1';
let notebook = {};
let canSave = true;
try { notebook = JSON.parse(localStorage.getItem(storageKey) || '{}'); if (!notebook || typeof notebook !== 'object' || Array.isArray(notebook)) notebook = {}; } catch { canSave = false; }
function save() { try { localStorage.setItem(storageKey, JSON.stringify(notebook)); } catch { canSave = false; } }
function noteFor(id) { if (!notebook[id] || typeof notebook[id] !== 'object') notebook[id] = {}; return notebook[id]; }
function statusText(entry) { if (!entry.reviewed) return 'Not yet self-checked. Reading a page does not mark it learned.'; const when = new Date(entry.reviewed); if (!Number.isFinite(when.getTime())) return 'Not yet self-checked.'; const due = new Date(when.getTime() + 86400000); return 'Self-checked ' + when.toLocaleDateString() + '. Recall again ' + due.toLocaleDateString() + (Date.now() >= due.getTime() ? ' — due now.' : '.'); }
for (const widget of document.querySelectorAll('[data-simulator]')) {
  const buttons = [...widget.querySelectorAll('[data-mode]')];
  function select(mode) {
    buttons.forEach(button => button.setAttribute('aria-pressed', String(button.dataset.mode === mode)));
    widget.querySelectorAll('[data-panel]').forEach(panel => { panel.hidden = panel.dataset.panel !== mode; });
    widget.querySelectorAll('[data-show-in]').forEach(node => node.classList.toggle('dormant', !node.dataset.showIn.split(' ').includes(mode)));
  }
  buttons.forEach(button => button.addEventListener('click', () => select(button.dataset.mode)));
  if (buttons.length) select(buttons[0].dataset.mode);
}
for (const form of document.querySelectorAll('[data-quiz]')) {
  form.addEventListener('submit', event => {
    event.preventDefault();
    const chosen = form.querySelector('input:checked');
    const feedback = form.querySelector('[role=status]');
    if (!chosen) { feedback.textContent = 'Choose an answer first, then check your prediction.'; return; }
    const correct = chosen.value === form.dataset.answer;
    feedback.classList.toggle('wrong', !correct);
    feedback.textContent = (correct ? 'Yes. ' : 'Try again. ') + (correct ? form.dataset.explanation : chosen.dataset.feedback);
  });
}
for (const block of document.querySelectorAll('[data-practice]')) {
  const id = block.dataset.practice;
  const textarea = block.querySelector('textarea');
  const status = block.querySelector('[role=status]');
  const entry = noteFor(id);
  if (typeof entry.note === 'string') textarea.value = entry.note;
  const refresh = () => { status.textContent = statusText(entry) + (canSave ? ' Saved only in this browser.' : ' Browser storage unavailable; export your notes before closing.'); };
  textarea.addEventListener('input', () => { entry.note = textarea.value; save(); refresh(); });
  block.querySelector('[data-record]').addEventListener('click', () => {
    if (!textarea.value.trim()) { status.textContent = 'Write your explanation or observation first. A click alone is not evidence.'; textarea.focus(); return; }
    entry.note = textarea.value; entry.reviewed = new Date().toISOString(); save(); refresh();
  });
  block.querySelector('[data-export]').addEventListener('click', () => {
    const text = '# Practice notes: ' + document.title + '\n\nSelf-assessment only; operational claims need command output and a date.\n\n' + textarea.value + '\n\n' + statusText(entry) + '\n';
    const url = URL.createObjectURL(new Blob([text], {type:'text/markdown'}));
    const link = document.createElement('a'); link.href = url; link.download = id + '-practice.md'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  });
  refresh();
}
for (const marker of document.querySelectorAll('[data-progress]')) marker.textContent = statusText(noteFor(marker.dataset.progress));
const filter = document.querySelector('[data-filter]');
if (filter) filter.addEventListener('input', () => {
  const query = filter.value.toLowerCase().trim();
  let count = 0;
  document.querySelectorAll('[data-find]').forEach(row => { row.hidden = !row.textContent.toLowerCase().includes(query); if (!row.hidden) count++; });
  document.querySelector('[data-count]').textContent = count + ' matching entries';
});
