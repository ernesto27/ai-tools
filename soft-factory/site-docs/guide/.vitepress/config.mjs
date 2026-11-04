import { defineConfig } from 'vitepress'

const spanishTheme = {
  nav: [
    { text: 'Desktop', link: 'https://ernesto27.github.io/ai-tools/desktop/' },
    { text: 'Guía de uso', link: '/es/first-run' },
    { text: 'Referencia', link: '/es/reference' },
    { text: 'Agent Sandbox', link: 'https://ernesto27.github.io/ai-tools/es/' }
  ],
  sidebar: [
    { text: 'Introducción', link: '/es/' },
    { text: 'Primeros pasos', items: [
      { text: 'Instalación', link: '/es/installation' },
      { text: 'Tu primera ejecución', link: '/es/first-run' }
    ] },
    { text: 'Uso cotidiano', items: [
      { text: 'Configuración', link: '/es/configuration' },
      { text: 'Flujo de trabajo y continuación', link: '/es/workflow' },
      { text: 'Documentos contexto', link: '/es/documents' },
      { text: 'Tareas de Jira', link: '/es/jira' },
      { text: 'Configuración de Google Drive', link: '/es/google-drive' },
      { text: 'Informes y registros de ejecución', link: '/es/reports' }
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
    pattern: 'https://github.com/ernesto27/ai-tools/edit/master/soft-factory/site-docs/guide/:path',
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
  title: 'Soft Factory',
  description: 'Implement tasks, review changes, assess risk, and understand the result with agent-sandbox.',
  locales: {
    root: { label: 'English', lang: 'en-US' },
    es: {
      label: 'Español',
      lang: 'es',
      description: 'Implementa tareas, revisa cambios, evalúa riesgos y comprende el resultado con agent-sandbox.',
      themeConfig: spanishTheme
    }
  },
  base: '/ai-tools/soft-factory/',
  lastUpdated: true,
  sitemap: { hostname: 'https://ernesto27.github.io/ai-tools/soft-factory/' },
  themeConfig: {
    logo: { src: '/mark.svg', alt: '' },
    nav: [
      { text: 'Desktop', link: 'https://ernesto27.github.io/ai-tools/desktop/' },
      { text: 'User guide', link: '/first-run' },
      { text: 'Reference', link: '/reference' },
      { text: 'Agent Sandbox', link: 'https://ernesto27.github.io/ai-tools/' }
    ],
    sidebar: [
      { text: 'Overview', link: '/' },
      { text: 'Getting started', items: [
        { text: 'Installation', link: '/installation' },
        { text: 'Your first run', link: '/first-run' }
      ] },
      { text: 'Everyday use', items: [
        { text: 'Configuration', link: '/configuration' },
        { text: 'Workflow and continuing work', link: '/workflow' },
        { text: 'Supporting documents', link: '/documents' },
        { text: 'Jira tasks', link: '/jira' },
        { text: 'Google Drive setup', link: '/google-drive' },
        { text: 'Reports and execution logs', link: '/reports' }
      ] },
      { text: 'Reference', items: [
        { text: 'Commands and flags', link: '/reference' },
        { text: 'Troubleshooting', link: '/troubleshooting' }
      ] }
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
      pattern: 'https://github.com/ernesto27/ai-tools/edit/master/soft-factory/site-docs/guide/:path',
      text: 'Edit this page on GitHub'
    }
  }
})
