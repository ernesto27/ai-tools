import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'agent-sandbox',
  description: 'Run coding agents in Docker, review their work, and publish when ready.',
  lang: 'en-US',
  base: '/ai-tools/',
  lastUpdated: true,
  sitemap: { hostname: 'https://ernesto27.github.io/ai-tools/' },
  themeConfig: {
    logo: { src: '/mark.svg', alt: '' },
    nav: [
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
    search: { provider: 'local' },
    outline: [2, 3],
    socialLinks: [{ icon: 'github', link: 'https://github.com/ernesto27/ai-tools' }],
    editLink: {
      pattern: 'https://github.com/ernesto27/ai-tools/edit/master/agent-sandbox/site-docs/guide/:path',
      text: 'Edit this page on GitHub'
    }
  }
})
