const copyStatus = document.querySelector('#copy-status');
const queryInput = document.querySelector('#utility-query');
const utilityCount = document.querySelector('#utility-count');
const emptyState = document.querySelector('#utility-empty');
const utilityCards = [...document.querySelectorAll('[data-utility]')];

document.querySelectorAll('.utility-header').forEach(button => {
  button.addEventListener('click', () => {
    const details = document.querySelector(`#${button.getAttribute('aria-controls')}`);
    const expanded = button.getAttribute('aria-expanded') === 'true';
    button.setAttribute('aria-expanded', String(!expanded));
    button.querySelector('.utility-open-text').textContent = expanded ? 'Open' : 'Close';
    details.hidden = expanded;
  });
});

queryInput.addEventListener('input', () => {
  const query = queryInput.value.trim().toLowerCase();
  let visibleCount = 0;

  utilityCards.forEach(card => {
    const matches = card.textContent.toLowerCase().includes(query);
    card.hidden = !matches;
    visibleCount += Number(matches);
  });

  utilityCount.textContent = `${visibleCount} ${visibleCount === 1 ? 'utility' : 'utilities'}`;
  emptyState.hidden = visibleCount !== 0;
});

document.querySelectorAll('.utility-copy').forEach(button => {
  button.addEventListener('click', async () => {
    const source = document.querySelector(`#${button.dataset.copy}`);
    try {
      await navigator.clipboard.writeText(source.textContent);
      button.textContent = 'Copied';
      copyStatus.textContent = 'Copied utility source';
      window.setTimeout(() => {
        button.textContent = 'Copy code';
        copyStatus.textContent = '';
      }, 1600);
    } catch {
      copyStatus.textContent = 'Browser clipboard permission was denied';
    }
  });
});
