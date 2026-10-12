(() => {
  const answer = document.querySelector('[data-practice-answer]');
  if (!answer) return;
  const summary = answer.querySelector('summary');
  const rating = document.querySelector('[data-practice-rating]');
  const buttons = Array.from(document.querySelectorAll('[data-rating]'));
  const status = document.querySelector('[data-rating-status]');
  answer.addEventListener('toggle', () => {
    summary.textContent = answer.open ? summary.dataset.openLabel : summary.dataset.closedLabel;
    if (rating) rating.hidden = !answer.open;
  });
  buttons.forEach(button => button.addEventListener('click', () => {
    buttons.forEach(item => item.setAttribute('aria-pressed', String(item === button)));
    if (status) status.textContent = status.dataset.prefix + ': ' + button.textContent;
  }));
  document.querySelector('[data-practice-reset]')?.addEventListener('click', () => {
    answer.open = false;
    buttons.forEach(item => item.setAttribute('aria-pressed', 'false'));
    if (status) status.textContent = '';
    answer.querySelectorAll('details').forEach(item => { item.open = false; });
    summary.focus();
  });
})();
