import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Soft Factory',
  description: 'Implement tasks, review changes, assess risk, and understand the result with agent-sandbox.',
  lang: 'en-US',
  base: '/ai-tools/soft-factory/',
  lastUpdated: true,
  sitemap: { hostname: 'https://ernesto27.github.io/ai-tools/soft-factory/' },
  themeConfig: {
    logo: { src: '/mark.svg', alt: '' },
    nav: [
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
    search: { provider: 'local' },
    outline: [2, 3],
    socialLinks: [{ icon: 'github', link: 'https://github.com/ernesto27/ai-tools' }],
    editLink: {
      pattern: 'https://github.com/ernesto27/ai-tools/edit/master/soft-factory/site-docs/guide/:path',
      text: 'Edit this page on GitHub'
    }
  }
})
