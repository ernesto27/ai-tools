'use strict';

const menuButton = document.querySelector('.menu-toggle');
const navigation = document.querySelector('#navigation');
function closeMenu() {
  navigation.classList.remove('open');
  menuButton.setAttribute('aria-expanded', 'false');
}
menuButton.addEventListener('click', () => {
  const open = navigation.classList.toggle('open');
  menuButton.setAttribute('aria-expanded', String(open));
});
navigation.querySelectorAll('a').forEach(link => link.addEventListener('click', closeMenu));
document.addEventListener('keydown', event => { if (event.key === 'Escape') closeMenu(); });
window.matchMedia('(min-width: 761px)').addEventListener('change', closeMenu);

const projectCards = [...document.querySelectorAll('.project')];
document.querySelectorAll('[data-filter]').forEach(button => {
  button.addEventListener('click', () => {
    document.querySelectorAll('[data-filter]').forEach(filter => {
      filter.classList.toggle('selected', filter === button);
      filter.setAttribute('aria-pressed', String(filter === button));
    });
    let visible = 0;
    projectCards.forEach(card => {
      card.hidden = button.dataset.filter !== 'all' && card.dataset.category !== button.dataset.filter;
      if (!card.hidden) visible++;
    });
    document.querySelector('#project-count').textContent = `${visible} PROJECTS ON DISPLAY`;
  });
});

const projects = {
  frequency: {
    title: 'Frequency®', category: 'WEB DESIGN & DEVELOPMENT / 2026',
    description: 'An independent music platform with the spirit of a record shop and the reach of the web. A concept for discovering your next favorite sound.',
    approach: 'Loud typography, analog textures, and a record-inspired visual language give digital discovery a tactile feel.',
    deliverables: 'Art direction, responsive website concept, interface design, and creative development.'
  },
  offbeat: {
    title: 'Offbeat Coffee', category: 'BRAND IDENTITY & PACKAGING / 2026',
    description: 'A playful identity for a coffee company that marches to its own rhythm. Good coffee deserves a little character.',
    approach: 'An expressive wordmark meets punchy color and imperfect shapes, creating an identity that feels alive on every surface.',
    deliverables: 'Brand concept, visual identity, packaging direction, and a flexible graphic toolkit.'
  },
  ordinary: {
    title: 'Ordinary Objects', category: 'E-COMMERCE & ART DIRECTION / 2025',
    description: 'A thoughtful storefront concept for everyday objects with a story. A quieter corner of the internet, built around the things we choose to live with.',
    approach: 'Warm neutrals, editorial typography, and generous space put the objects and their craftsmanship at the center.',
    deliverables: 'E-commerce concept, visual design, product art direction, and responsive layouts.'
  },
  daybreak: {
    title: 'Daybreak Outdoor Club', category: 'STRATEGY & BRAND IDENTITY / 2025',
    description: 'A welcoming outdoor club concept for people who care more about getting out there than getting there first.',
    approach: 'Sunrise-inspired graphics and a warm, grounded palette balance a sense of adventure with an open invitation.',
    deliverables: 'Positioning, naming concept, visual identity, and outdoor merchandise direction.'
  },
  pixel: {
    title: 'Pixel Playground', category: 'INTERACTIVE EXPERIMENT / 2024',
    description: 'An ongoing creative sketchbook exploring what happens when shape, color, and a few lines of code get a little room to play.',
    approach: 'Start with a simple shape. Change the rules. Find something unexpected. This study explores scale, contrast, and joyful composition.',
    deliverables: 'Generative graphic studies, browser experiments, and an evolving visual language.'
  },
  afterhours: {
    title: 'After Hours', category: 'TYPE & MOTION EXPLORATION / 2024',
    description: 'An experimental visual direction inspired by late-night radio, underground posters, and the energy of a city after dark.',
    approach: 'High-contrast typography and a phosphor-inspired palette explore how a static composition can suggest rhythm and movement.',
    deliverables: 'Typographic studies, poster concepts, and digital art direction.'
  }
};
const dialog = document.querySelector('#project-dialog');
projectCards.forEach(card => card.addEventListener('click', () => {
  const project = projects[card.dataset.project];
  document.querySelector('#dialog-art').replaceChildren(card.querySelector('.project-art').cloneNode(true));
  for (const key of ['title', 'category', 'description', 'approach', 'deliverables']) {
    document.querySelector(`#dialog-${key}`).textContent = project[key];
  }
  dialog.showModal();
  dialog.scrollTop = 0;
  document.body.classList.add('modal-open');
}));
function closeDialog() { dialog.close(); }
document.querySelector('.dialog-close').addEventListener('click', closeDialog);
document.querySelector('#dialog-contact').addEventListener('click', closeDialog);
dialog.addEventListener('click', event => {
  if (event.target !== dialog) return;
  const rect = dialog.getBoundingClientRect();
  if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) closeDialog();
});
dialog.addEventListener('close', () => document.body.classList.remove('modal-open'));

const copyButton = document.querySelector('#copy-email');
copyButton.addEventListener('click', async () => {
  const status = document.querySelector('#copy-status');
  try {
    await navigator.clipboard.writeText('hello@alexrivera.design');
    status.textContent = 'EMAIL COPIED. GOOD THINGS START WITH A HELLO.';
  } catch {
    status.textContent = 'COPY THIS ADDRESS: hello@alexrivera.design';
  }
});

const observer = new IntersectionObserver(entries => {
  entries.forEach(entry => {
    if (!entry.isIntersecting) return;
    navigation.querySelectorAll('a').forEach(link => link.classList.toggle('active', link.hash === `#${entry.target.id}`));
  });
}, { rootMargin: '-15% 0px -45% 0px', threshold: 0 });
['work', 'about', 'contact'].forEach(id => observer.observe(document.getElementById(id)));
document.querySelector('#year').textContent = new Date().getFullYear();

// A quiet, original synth arpeggio. Audio starts only after an explicit click.
let audioContext;
let soundTimer;
let noteIndex = 0;
const notes = [220, 261.63, 329.63, 392, 329.63, 261.63, 196, 261.63];
const soundButton = document.querySelector('#sound-toggle');
function stopSound() {
  clearInterval(soundTimer);
  soundTimer = undefined;
  if (audioContext) audioContext.suspend();
  soundButton.setAttribute('aria-pressed', 'false');
  soundButton.innerHTML = '<span class="sound-icon"></span> SOUND OFF';
}
soundButton.addEventListener('click', async () => {
  if (soundTimer) { stopSound(); return; }
  try {
    const AudioEngine = window.AudioContext || window.webkitAudioContext;
    if (!AudioEngine) throw new Error('Audio unavailable');
    audioContext ??= new AudioEngine();
    await audioContext.resume();
    const playNote = () => {
      const oscillator = audioContext.createOscillator();
      const gain = audioContext.createGain();
      const now = audioContext.currentTime;
      oscillator.type = 'triangle';
      oscillator.frequency.value = notes[noteIndex++ % notes.length];
      gain.gain.setValueAtTime(0, now);
      gain.gain.linearRampToValueAtTime(0.035, now + 0.025);
      gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.6);
      oscillator.connect(gain).connect(audioContext.destination);
      oscillator.start(now);
      oscillator.stop(now + 0.65);
      oscillator.onended = () => { oscillator.disconnect(); gain.disconnect(); };
    };
    playNote();
    soundTimer = setInterval(playNote, 380);
    soundButton.setAttribute('aria-pressed', 'true');
    soundButton.innerHTML = '<span class="sound-icon"></span> SOUND ON';
  } catch {
    stopSound();
    soundButton.textContent = 'SOUND UNAVAILABLE';
  }
});
document.addEventListener('visibilitychange', () => { if (document.hidden) stopSound(); });
