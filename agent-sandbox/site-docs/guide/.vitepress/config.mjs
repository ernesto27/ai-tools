import { defineConfig } from 'vitepress'

const spanishTheme = {
  nav: [
    { text: 'Desktop', link: 'https://ernesto27.github.io/ai-tools/desktop/' },
    { text: 'Guía de uso', link: '/es/first-run' },
    { text: 'Referencia', link: '/es/reference' },
    { text: 'Soft Factory', link: 'https://ernesto27.github.io/ai-tools/soft-factory/' }
  ],
  sidebar: [
    { text: 'Introducción', link: '/es/' },
    { text: 'Primeros pasos', items: [
      { text: 'Instalación', link: '/es/installation' },
      { text: 'Autenticación', link: '/es/authentication' },
      { text: 'Tu primera ejecución', link: '/es/first-run' }
    ] },
    { text: 'Uso cotidiano', items: [
      { text: 'Prompts e interfaz de terminal', link: '/es/running' },
      { text: 'Configuración', link: '/es/configuration' },
      { text: 'Worktrees y continuación de tareas', link: '/es/worktrees' },
      { text: 'Push y pull requests', link: '/es/publishing' },
      { text: 'Imágenes y redes', link: '/es/environment' }
    ] },
    { text: 'Referencia', items: [
      { text: 'Comandos y opciones', link: '/es/reference' },
      { text: 'Solución de problemas', link: '/es/troubleshooting' }
    ] }
  ],
  outline: { level: [2, 3], label: 'En esta página' },
  docFooter: { prev: 'Página anterior', next: 'Página siguiente' },
  lastUpdated: { text: 'Última actualización', formatOptions: { dateStyle: 'short', timeStyle: 'short' } },
  editLink: {
    pattern: 'https://github.com/ernesto27/ai-tools/edit/master/agent-sandbox/site-docs/guide/:path',
    text: 'Editar esta página en GitHub'
  },
  langMenuLabel: 'Cambiar idioma',
  sidebarMenuLabel: 'Menú',
  returnToTopLabel: 'Volver arriba',
  skipToContentLabel: 'Saltar al contenido',
  darkModeSwitchLabel: 'Apariencia',
  lightModeSwitchTitle: 'Cambiar al tema claro',
  darkModeSwitchTitle: 'Cambiar al tema oscuro'
}

export default defineConfig({
  title: 'agent-sandbox',
  description: 'Run coding agents in Docker, review their work, and publish when ready.',
  locales: {
    root: { label: 'English', lang: 'en-US' },
    es: {
      label: 'Español',
      lang: 'es',
      description: 'Ejecuta agentes de programación en Docker, revisa su trabajo y publica cuando esté listo.',
      themeConfig: spanishTheme
    }
  },
  base: '/ai-tools/',
  lastUpdated: true,
  sitemap: { hostname: 'https://ernesto27.github.io/ai-tools/' },
  themeConfig: {
    logo: { src: '/mark.svg', alt: '' },
    nav: [
      { text: 'Desktop', link: 'https://ernesto27.github.io/ai-tools/desktop/' },
      { text: 'User guide', link: '/first-run' },
      { text: 'Reference', link: '/reference' },
      { text: 'Soft Factory', link: 'https://ernesto27.github.io/ai-tools/soft-factory/' }
    ],
    sidebar: [
      { text: 'Overview', link: '/' },
      {
        text: 'Getting started',
        items: [
          { text: 'Installation', link: '/installation' },
          { text: 'Authentication', link: '/authentication' },
          { text: 'Your first run', link: '/first-run' }
        ]
      },
      {
        text: 'Everyday use',
        items: [
          { text: 'Prompts and terminal UI', link: '/running' },
          { text: 'Configuration', link: '/configuration' },
          { text: 'Worktrees and continuing work', link: '/worktrees' },
          { text: 'Push and pull requests', link: '/publishing' },
          { text: 'Images and networking', link: '/environment' }
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'Commands and flags', link: '/reference' },
          { text: 'Troubleshooting', link: '/troubleshooting' }
        ]
      }
    ],
    search: {
      provider: 'local',
      options: {
        locales: {
          es: {
            translations: {
              button: { buttonText: 'Buscar', buttonAriaLabel: 'Buscar en la documentación' },
              modal: {
                displayDetails: 'Mostrar lista detallada',
                resetButtonTitle: 'Limpiar búsqueda',
                backButtonTitle: 'Cerrar búsqueda',
                noResultsText: 'Sin resultados para',
                footer: {
                  selectText: 'seleccionar', selectKeyAriaLabel: 'Intro',
                  navigateText: 'navegar', navigateUpKeyAriaLabel: 'Flecha arriba',
                  navigateDownKeyAriaLabel: 'Flecha abajo',
                  closeText: 'cerrar', closeKeyAriaLabel: 'Escape'
                }
              }
            }
          }
        }
      }
    },
    outline: [2, 3],
    socialLinks: [{ icon: 'github', link: 'https://github.com/ernesto27/ai-tools' }],
    editLink: {
      pattern: 'https://github.com/ernesto27/ai-tools/edit/master/agent-sandbox/site-docs/guide/:path',
      text: 'Edit this page on GitHub'
    }
  }
})
