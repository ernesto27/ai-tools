const filterButtons = document.querySelectorAll('[data-filter]');
const projects = document.querySelectorAll('[data-project]');
filterButtons.forEach(button => {
  button.addEventListener('click', () => {
    filterButtons.forEach(item => {
      const selected = item === button;
      item.classList.toggle('active', selected);
      item.setAttribute('aria-pressed', String(selected));
    });
    projects.forEach(project => {
      project.hidden = button.dataset.filter !== 'all' && project.dataset.category !== button.dataset.filter;
    });
  });
});
const details = {
  sol: { title: 'Sol Soda', description: 'A self-initiated identity for a sunshine-inspired soda brand. Warm citrus colors, expressive lettering, and a playful visual system turn an everyday drink into a little moment of joy.', tags: ['Brand strategy', 'Visual identity', 'Packaging concept'] },
  form: { title: 'Form & Field', description: 'A digital storefront concept for thoughtfully made home objects. Quiet colors, tactile shapes, and an editorial layout create room for the objects and their stories to breathe.', tags: ['Art direction', 'Web design', 'Native development'] },
  after: { title: 'After Hours', description: 'An independent concept for a music and culture space. Electric blue, oversized type, and vinyl-inspired graphics build a flexible identity with a rhythm of its own.', tags: ['Brand identity', 'Art direction', 'Digital design'] }
};
const dialog = document.querySelector('#project-dialog');
dialog.setAttribute('aria-labelledby', 'dialog-title');
projects.forEach(project => project.addEventListener('click', () => {
  const detail = details[project.dataset.project];
  document.querySelector('#dialog-title').textContent = detail.title;
  document.querySelector('#dialog-description').textContent = detail.description;
  document.querySelector('#dialog-tags').replaceChildren(...detail.tags.map(tag => {
    const span = document.createElement('span');
    span.textContent = tag;
    return span;
  }));
  dialog.showModal();
}));
document.querySelectorAll('.close, .close-bottom').forEach(button => button.addEventListener('click', () => dialog.close()));
dialog.addEventListener('click', event => { if (event.target === dialog) { const bounds = dialog.getBoundingClientRect(); if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dialog.close(); } });
document.querySelector('#year').textContent = new Date().getFullYear();
