const filterButtons = document.querySelectorAll('[data-filter]');
const projects = document.querySelectorAll('.project');
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
  orbit: { title: 'Orbit Studio', description: 'A concept website for an independent sound studio. Expressive typography and orbital forms bring a music-first identity to life, with a focus on clear navigation and immersive art direction.', tags: ['HTML', 'CSS', 'JAVASCRIPT', 'ART DIRECTION'] },
  form: { title: 'Form & Function', description: 'An editorial storefront concept exploring the beauty of everyday objects. A restrained palette, sculptural forms, and a considered type system keep the products at the center of the experience.', tags: ['UI DESIGN', 'VISUAL IDENTITY', 'E-COMMERCE CONCEPT'] },
  wave: { title: 'Wave Machine', description: 'A creative coding exploration of rhythm, repetition, and optical movement. The study uses native web patterns to turn simple geometry into a bold visual playground.', tags: ['CREATIVE CODING', 'CSS', 'VISUAL EXPERIMENT'] },
  terrain: { title: 'Terrain', description: 'A travel journal concept for people who take the scenic route. Warm landscape-inspired colors and generous typography create a welcoming space for outdoor stories.', tags: ['HTML', 'CSS', 'JAVASCRIPT', 'WEB DESIGN'] }
};
const dialog = document.querySelector('#project-dialog');
document.querySelectorAll('[data-project]').forEach(button => {
  button.addEventListener('click', () => {
    const project = details[button.dataset.project];
    document.querySelector('#dialog-title').textContent = project.title;
    document.querySelector('#dialog-description').textContent = project.description;
    document.querySelector('#dialog-tags').replaceChildren(...project.tags.map(tag => {
      const element = document.createElement('span');
      element.textContent = tag;
      return element;
    }));
    dialog.showModal();
  });
});
document.querySelector('.close-dialog').addEventListener('click', () => dialog.close());
dialog.addEventListener('click', event => { if (event.target === dialog) {
  const rect = dialog.getBoundingClientRect();
  if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) dialog.close();
} });
document.querySelector('#year').textContent = new Date().getFullYear();
