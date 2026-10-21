const $ = (selector) => document.querySelector(selector);
let tasks = [], status = 'all', category = null, editing = null, deleting = null, busy = false, loaded = false;
const icons = (name) => `<svg aria-hidden="true"><use href="#${name}"/></svg>`;
function toast(message) { $('#toast').textContent = message; $('#toast').classList.add('visible'); clearTimeout(toast.timer); toast.timer = setTimeout(() => $('#toast').classList.remove('visible'), 3000); }
async function api(path = '', options = {}) {
  const response = await fetch('/api/tasks' + path, { ...options, headers: { 'Content-Type': 'application/json' } });
  if (!response.ok) { let message = 'Something went wrong. Please try again.'; try { const result = await response.json(); message = result.error || message; } catch {} throw new Error(message); }
  return response.status === 204 ? null : response.json();
}
async function load() {
  try { tasks = await api(); loaded = true; render(); }
  catch (error) { loaded = false; $('#task-list').innerHTML = '<div class="empty-state load-error"><h3>Could not connect to your tasks.</h3><p>Your saved tasks are still in your database.</p><button id="retry">Try again</button></div>'; $('#retry').onclick = load; $('.panel-footer>span:last-child').textContent = 'Connection unavailable'; $('.statusbar>span:first-child').textContent = 'DATABASE UNAVAILABLE'; toast(error.message); }
}
function setStatus(next) { status = next; category = null; render(); }
function render() {
  const completed = tasks.filter(t => t.completed).length;
  const counts = { all: tasks.length, active: tasks.length - completed, completed };
  for (const [key, value] of Object.entries(counts)) { $('#nav-' + key).textContent = value; $('#tab-' + key).textContent = value; }
  $('#total-count').textContent = tasks.length; $('#active-count').textContent = counts.active; $('#completed-count').textContent = completed;
  document.querySelectorAll('[data-status]').forEach(button => { const selected = !category && button.dataset.status === status; button.classList.toggle('selected', selected); if (button.classList.contains('tab')) button.setAttribute('aria-selected', String(button.dataset.status === status)); });
  document.querySelectorAll('[data-category]').forEach(button => { button.classList.toggle('selected', button.dataset.category === category); $('#count-' + button.dataset.category).textContent = tasks.filter(t => t.category === button.dataset.category).length; });
  const title = category || {all: 'All tasks', active: 'Active tasks', completed: 'Completed tasks'}[status];
  $('#page-title').textContent = title; const dot = document.createElement('span'); dot.textContent = '.'; $('#page-title').append(dot); $('#breadcrumb-current').textContent = title;
  const term = $('#task-search').value.trim().toLowerCase();
  const filtered = tasks.filter(t => (status === 'all' || t.completed === (status === 'completed')) && (!category || t.category === category) && `${t.title} ${t.notes}`.toLowerCase().includes(term));
  const rank = { High: 0, Medium: 1, Low: 2 };
  filtered.sort((a,b) => $('#sort').value === 'priority' ? rank[a.priority] - rank[b.priority] || b.id - a.id : $('#sort').value === 'oldest' ? a.id - b.id : b.id - a.id);
  $('#task-list').replaceChildren();
  filtered.forEach(task => {
    const row = document.createElement('div'); row.className = 'task-row' + (task.completed ? ' completed' : ''); row.dataset.id = task.id;
    row.innerHTML = `<div class="task-main"><button class="task-check" aria-label="${task.completed ? 'Mark active' : 'Complete task'}" aria-pressed="${task.completed}">${task.completed ? '<svg viewBox="0 0 24 24"><path d="m5 12 4 4 10-10"/></svg>' : ''}</button><div class="task-copy"><button class="task-title"></button><p class="task-notes"></p></div></div><span class="task-category"><i class="category-dot ${task.category.toLowerCase()}"></i>${task.category}</span><span class="priority ${task.priority.toLowerCase()}">${task.priority}</span><div class="row-actions"><button class="edit-button" aria-label="Edit task">${icons('edit')}</button><button class="delete-button" aria-label="Delete task">${icons('trash')}</button></div>`;
    row.querySelector('.task-title').textContent = task.title;
    row.querySelector('.task-title').title = task.title;
    row.querySelector('.task-notes').textContent = task.notes || (task.completed ? 'One less thing on your mind.' : 'A small step in the right direction.');
    row.querySelector('.task-check').onclick = async () => { if (busy) return; busy = true; try { const updated = await api('/' + task.id, {method:'PUT',body:JSON.stringify({...task, completed: !task.completed})}); tasks = tasks.map(t => t.id === updated.id ? updated : t); render(); toast(updated.completed ? 'A little progress. Task completed.' : 'Task moved back to active.'); } catch (error) { toast(error.message); } finally { busy = false; } };
    row.querySelector('.task-title').onclick = row.querySelector('.edit-button').onclick = () => openTask(task);
    row.querySelector('.delete-button').onclick = () => { deleting = task.id; $('#delete-dialog').showModal(); };
    $('#task-list').append(row);
  });
  if (!filtered.length) {
    const empty = document.createElement('div'); empty.className = 'empty-state';
    const noTasks = tasks.length === 0;
    empty.innerHTML = `${icons(noTasks ? 'sun' : 'check')}<h3>${noTasks ? 'A fresh start. A little possibility.' : term ? 'No tasks found.' : status === 'completed' ? 'Good things take a little doing.' : 'A little breathing room.'}</h3><p>${noTasks ? 'Get it out of your head and onto your list.<br>Your first small step starts here.' : term ? 'Try a different search or switch to another list.' : status === 'completed' ? 'Complete your first task and celebrate it here.' : 'No tasks in this view. Make space for what comes next.'}</p>${noTasks ? '<button class="primary" id="first-task">'+icons('plus')+'Create your first task</button>' : ''}`;
    $('#task-list').append(empty); if (noTasks) $('#first-task').onclick = () => openTask();
  }
  $('#results-count').textContent = `${filtered.length} ${filtered.length === 1 ? 'task' : 'tasks'}${term || category || status !== 'all' ? ' shown' : ' in your workspace'}`;
  $('.panel-footer>span:last-child').innerHTML = '<span class="live-dot"></span>All changes saved locally';
  $('.statusbar>span:first-child').innerHTML = '<i class="live-dot"></i> SYSTEM READY <span class="status-divider">/</span> SQLITE CONNECTED';
}
function openTask(task = null) {
  editing = task; $('#task-form').reset(); $('#form-error').textContent = '';
  $('#dialog-title').textContent = task ? 'A little fine-tuning.' : 'New task';
  $('#task-title').value = task?.title || ''; $('#task-notes').value = task?.notes || ''; $('#task-category').value = task?.category || category || 'Personal'; $('#task-priority').value = task?.priority || 'Medium';
  $('#save-task').innerHTML = (task ? 'Save changes ' : 'Create task ') + icons('arrow');
  $('#task-dialog').showModal(); $('#task-title').focus();
}
$('#task-form').onsubmit = async event => {
  event.preventDefault(); if (busy) return;
  const title = $('#task-title').value.trim(); if (!title) { $('#form-error').textContent = 'Give your task a name.'; return; }
  busy = true; $('#save-task').disabled = true;
  try {
    const task = await api(editing ? '/' + editing.id : '', { method: editing ? 'PUT' : 'POST', body: JSON.stringify({title, notes: $('#task-notes').value.trim(), category: $('#task-category').value, priority: $('#task-priority').value, completed: editing?.completed || false}) });
    tasks = editing ? tasks.map(t => t.id === task.id ? task : t) : [task, ...tasks];
    $('#task-dialog').close(); loaded = true; render(); toast(editing ? 'Task updated. A little clearer.' : 'Task added. You’ve got this.');
  } catch (error) { $('#form-error').textContent = error.message; } finally { busy = false; $('#save-task').disabled = false; }
};
$('#confirm-delete').onclick = async () => { if (busy) return; busy = true; $('#confirm-delete').disabled = true; try { await api('/' + deleting, {method:'DELETE'}); tasks = tasks.filter(t => t.id !== deleting); $('#delete-dialog').close(); render(); toast('Task deleted. Making a little space.'); } catch (error) { toast(error.message); } finally {busy = false; $('#confirm-delete').disabled = false;} };
$('#new-task').onclick = $('#add-task').onclick = () => openTask();
document.querySelectorAll('[data-close]').forEach(button => button.onclick = () => button.closest('dialog').close());
document.querySelectorAll('[data-status]').forEach(button => button.onclick = () => { if (loaded) setStatus(button.dataset.status); });
document.querySelectorAll('[data-category]').forEach(button => button.onclick = () => { if (!loaded) return; category = category === button.dataset.category ? null : button.dataset.category; status = 'all'; render(); });
$('#task-search').oninput = $('#sort').onchange = () => { if (loaded) render(); };
$('#help-button').onclick = $('#shortcuts-button').onclick = $('#about-button').onclick = () => $('#help-dialog').showModal();
document.addEventListener('keydown', event => { if (event.ctrlKey || event.metaKey || event.altKey || document.querySelector('dialog[open]') || ['INPUT','TEXTAREA','SELECT'].includes(document.activeElement.tagName)) return; if (event.key.toLowerCase() === 'n') { event.preventDefault(); openTask(); } else if (event.key === '/') { event.preventDefault(); $('#task-search').focus(); } else if (event.key === '?') $('#help-dialog').showModal(); });
load();
