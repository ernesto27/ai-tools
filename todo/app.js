(() => {
  'use strict';

  const STORAGE_KEY = 'daylist.tasks.v1';
  const form = document.querySelector('#task-form');
  const input = document.querySelector('#task-input');
  const list = document.querySelector('#task-list');
  const filters = document.querySelector('.filters');
  const clearButton = document.querySelector('#clear-completed');
  const emptyState = document.querySelector('#empty-state');
  const announcement = document.querySelector('#announcement');
  let tasks = [];
  let filter = 'all';

  function storageUnavailable() {
    document.querySelector('#storage-warning').hidden = false;
    document.querySelector('#storage-status').textContent = 'In-memory only';
  }

  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]');
    // Treat saved data as untrusted, including data changed outside the app.
    if (!Array.isArray(stored) || !stored.every(task =>
      task && typeof task.id === 'string' && task.id.length > 0 &&
      typeof task.text === 'string' && task.text.trim().length > 0 &&
      task.text.length <= 240 && typeof task.completed === 'boolean'
    ) || new Set(stored.map(task => task.id)).size !== stored.length) {
      throw new Error('Invalid saved tasks');
    }
    tasks = stored;
  } catch {
    storageUnavailable();
  }

  function save() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(tasks));
      document.querySelector('#storage-warning').hidden = true;
      document.querySelector('#storage-status').textContent = 'Saved on this device';
    } catch {
      storageUnavailable();
    }
  }

  function announce(message) {
    announcement.textContent = message;
  }

  function render(focusId) {
    const visible = tasks.filter(task => filter === 'all' || (filter === 'completed' ? task.completed : !task.completed));
    list.replaceChildren();

    for (const task of visible) {
      const row = document.createElement('li');
      row.className = `task-row${task.completed ? ' completed' : ''}`;

      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.className = 'task-check';
      checkbox.id = `task-${task.id}`;
      checkbox.checked = task.completed;
      checkbox.addEventListener('change', () => {
        task.completed = checkbox.checked;
        save();
        render(task.id);
        announce(task.completed ? 'Task completed. Nicely done!' : 'Task marked active.');
      });

      const label = document.createElement('label');
      label.className = 'task-label';
      label.htmlFor = checkbox.id;
      label.textContent = task.text;

      const deleteButton = document.createElement('button');
      deleteButton.type = 'button';
      deleteButton.className = 'delete-button';
      deleteButton.setAttribute('aria-label', `Delete task: ${task.text}`);
      deleteButton.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v5m4-5v5"/></svg>';
      deleteButton.addEventListener('click', () => {
        const nextTask = visible[visible.indexOf(task) + 1] || visible[visible.indexOf(task) - 1];
        tasks = tasks.filter(item => item.id !== task.id);
        save();
        render(nextTask?.id);
        if (!nextTask) input.focus();
        announce('Task deleted.');
      });

      row.append(checkbox, label, deleteButton);
      list.append(row);
    }

    const completed = tasks.filter(task => task.completed).length;
    const remaining = tasks.length - completed;
    const percent = tasks.length ? Math.round(completed / tasks.length * 100) : 0;
    document.querySelector('#task-count').textContent = tasks.length;
    document.querySelector('#remaining-count').textContent = tasks.length
      ? `${remaining} ${remaining === 1 ? 'task' : 'tasks'} left`
      : 'No tasks yet';
    clearButton.hidden = completed === 0;
    emptyState.hidden = visible.length > 0;

    let emptyTitle = 'A fresh start feels good.';
    let emptyCopy = 'Add your first task and give your day a little direction.';
    if (filter === 'completed') {
      emptyTitle = 'Good things take a first step.';
      emptyCopy = 'Tasks you complete will appear here.';
    } else if (filter === 'active' && tasks.length) {
      emptyTitle = 'All done. Take a little breather.';
      emptyCopy = 'You’ve checked off everything on your list.';
    }
    document.querySelector('#empty-title').textContent = emptyTitle;
    document.querySelector('#empty-copy').textContent = emptyCopy;
    document.querySelector('#progress-card').hidden = tasks.length === 0;
    document.querySelector('#progress-title').textContent = remaining === 0
      ? 'A little lighter. You did it!'
      : completed ? 'Look at you, making progress.' : 'Every little step counts.';
    document.querySelector('#progress-label').textContent = `${completed} of ${tasks.length} tasks complete`;
    document.querySelector('#task-progress').max = tasks.length || 1;
    document.querySelector('#task-progress').value = completed;
    document.querySelector('#progress-percent').textContent = `${percent}%`;

    if (focusId) {
      const target = document.getElementById(`task-${focusId}`);
      (target || filters.querySelector('[aria-pressed="true"]')).focus();
    }
  }

  form.addEventListener('submit', event => {
    event.preventDefault();
    const text = input.value.trim();
    if (!text) {
      input.setCustomValidity('Please enter a task.');
      input.reportValidity();
      return;
    }
    tasks.push({ id: crypto.randomUUID(), text, completed: false });
    save();
    // Make every newly added task visible, including from the Completed filter.
    setFilter('all');
    input.value = '';
    input.focus();
    announce('Task added.');
  });

  input.addEventListener('input', () => input.setCustomValidity(''));

  function setFilter(value) {
    filter = value;
    for (const button of filters.querySelectorAll('button')) {
      button.setAttribute('aria-pressed', String(button.dataset.filter === filter));
    }
    render();
  }

  filters.addEventListener('click', event => {
    const button = event.target.closest('button[data-filter]');
    if (button) setFilter(button.dataset.filter);
  });

  clearButton.addEventListener('click', () => {
    tasks = tasks.filter(task => !task.completed);
    save();
    render();
    filters.querySelector('[aria-pressed="true"]').focus();
    announce('Completed tasks cleared.');
  });

  const now = new Date();
  document.querySelector('#today').textContent = now.toLocaleDateString(undefined, {
    weekday: 'long', month: 'short', day: 'numeric'
  });
  render();
})();
