import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Soft Factory Desktop',
  description: 'Configure repositories, launch coding workflows, and follow their output from your desktop.',
  lang: 'en-US',
  base: '/ai-tools/desktop/',
  lastUpdated: true,
  sitemap: { hostname: 'https://ernesto27.github.io/ai-tools/desktop/' },
  themeConfig: {
    logo: { src: '/mark.svg', alt: '' },
    nav: [
      { text: 'User guide', link: '/first-run' },
      { text: 'Screenshots', link: '/screenshots' },
      { text: 'Agent Sandbox', link: 'https://ernesto27.github.io/ai-tools/' },
      { text: 'Soft Factory CLI', link: 'https://ernesto27.github.io/ai-tools/soft-factory/' }
    ],
    sidebar: [
      { text: 'Overview', link: '/' },
      { text: 'Getting started', items: [
        { text: 'Installation', link: '/installation' },
        { text: 'Your first run', link: '/first-run' }
      ] },
      { text: 'Everyday use', items: [
        { text: 'Configuration', link: '/configuration' },
        { text: 'Runs and logs', link: '/runs' },
        { text: 'Screenshots', link: '/screenshots' },
        { text: 'Troubleshooting', link: '/troubleshooting' }
      ] }
    ],
    search: { provider: 'local' },
    outline: [2, 3],
    socialLinks: [{ icon: 'github', link: 'https://github.com/ernesto27/ai-tools' }],
    editLink: {
      pattern: 'https://github.com/ernesto27/ai-tools/edit/master/desktop-app/site-docs/guide/:path',
      text: 'Edit this page on GitHub'
    }
  }
})
