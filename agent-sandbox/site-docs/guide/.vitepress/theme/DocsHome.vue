<script setup>
import { ref } from 'vue'
import { withBase } from 'vitepress'

const command = 'agent-sandbox run -b fix-login -a codex \\\n  -q "fix the login redirect loop"'
const copyLabel = ref('Copy command')

async function copyCommand() {
  try {
    await navigator.clipboard.writeText(command)
    copyLabel.value = 'Copied'
  } catch {
    copyLabel.value = 'Select the command to copy'
  }
}

const chapters = [
  { number: '01', title: 'Set up your sandbox', description: 'From installation to your first completed task.', links: [
    { text: 'Installation', path: '/installation' },
    { text: 'Authentication', path: '/authentication' },
    { text: 'Your first run', path: '/first-run' }
  ] },
  { number: '02', title: 'Build your workflow', description: 'Configure a session, continue the work, and publish it.', links: [
    { text: 'Prompts & terminal UI', path: '/running' },
    { text: 'Configuration', path: '/configuration' },
    { text: 'Worktrees', path: '/worktrees' },
    { text: 'Push & pull requests', path: '/publishing' }
  ] },
  { number: '03', title: 'Keep a reference handy', description: 'Find the option you need or get past a failed run.', links: [
    { text: 'Images & networking', path: '/environment' },
    { text: 'Commands & flags', path: '/reference' },
    { text: 'Troubleshooting', path: '/troubleshooting' }
  ] }
]
</script>

<template>
  <div class="sandbox-home">
    <section class="sandbox-hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <p class="eyebrow"><span class="status-mark" aria-hidden="true"></span> Coding agents / Separate workspaces</p>
        <h1 id="hero-title">Your agent.<br>Its own workspace.</h1>
        <p class="hero-description">Run a coding agent on your local machine, inside Docker and on a Git worktree of its own. Give it a task. Review the changes. Publish when you’re ready.</p>
        <div class="hero-actions">
          <a class="primary-link" :href="withBase('/installation')">Get started <span aria-hidden="true">↗︎</span></a>
          <a class="secondary-link" :href="withBase('/first-run')">Walk through a first run <span aria-hidden="true">→</span></a>
        </div>
        <p class="hero-platform">Linux x86_64 <span aria-hidden="true">·</span> Docker <span aria-hidden="true">·</span> Git</p>
      </div>

      <div class="workspace-panel">
        <div class="panel-heading"><span class="terminal-symbol" aria-hidden="true">&gt;_</span><span>A task, in its own worktree</span><span class="panel-label">CLI</span></div>
        <div class="command-example">
          <div class="command-heading"><span>Start a session</span><button type="button" @click="copyCommand">{{ copyLabel }}</button></div>
          <pre><span class="shell-prompt" aria-hidden="true">$ </span><code>{{ command }}</code></pre>
          <span class="copy-status" role="status">{{ copyLabel === 'Copied' ? 'Command copied to clipboard.' : '' }}</span>
        </div>
        <div class="workspace-map" role="group" aria-label="The host repository and a separate sandbox worktree">
          <div class="workspace-node">
            <span class="node-marker" aria-hidden="true"></span>
            <div><span class="node-label">Your working copy</span><strong>Keep working here</strong></div>
            <span class="node-tag">HOST</span>
          </div>
          <div class="worktree-connection"><span aria-hidden="true">↳</span> separate branch + working directory</div>
          <div class="workspace-node sandbox-node">
            <span class="node-marker" aria-hidden="true"></span>
            <div><span class="node-label">Agent’s worktree</span><strong>fix-login <span>/workspace</span></strong></div>
            <span class="node-tag">DOCKER</span>
          </div>
        </div>
        <div class="panel-footnote"><span aria-hidden="true">↗︎</span> Review locally, push a branch, or open a PR.</div>
      </div>
    </section>

    <div class="agent-strip" aria-label="Supported agents">
      <span class="eyebrow">Choose your agent</span>
      <div><span>Codex</span><span>Claude Code</span><span>opencode</span><span>pi</span></div>
    </div>

    <section class="workflow-section" aria-labelledby="workflow-title">
      <div class="section-heading"><p class="eyebrow">The workflow</p><h2 id="workflow-title">A short path from task to review.</h2></div>
      <ol class="workflow-steps">
        <li><span class="step-number">01 /</span><h3>Describe the task</h3><p>Start from your repository. Choose an agent and tell it what you want changed.</p></li>
        <li><span class="step-number">02 /</span><h3>Let the agent work</h3><p>The tool creates a worktree and runs the agent inside a Docker container.</p></li>
        <li><span class="step-number">03 /</span><h3>Review and continue</h3><p>Inspect the result, resume the task, or publish with <code>--push</code> or <code>--pr</code>.</p></li>
      </ol>
    </section>

    <section class="guide-section" aria-labelledby="guide-title">
      <div class="section-heading"><p class="eyebrow">The user guide</p><h2 id="guide-title">Start here. Come back as you need.</h2></div>
      <div class="guide-chapters">
        <div v-for="chapter in chapters" :key="chapter.number" class="guide-chapter">
          <span class="chapter-number">{{ chapter.number }}</span>
          <div class="chapter-intro"><h3>{{ chapter.title }}</h3><p>{{ chapter.description }}</p></div>
          <div class="chapter-links"><a v-for="link in chapter.links" :key="link.path" :href="withBase(link.path)">{{ link.text }} <span aria-hidden="true">↗︎</span></a></div>
        </div>
      </div>
    </section>

    <footer class="home-footer"><span>agent-sandbox <span class="footer-divider">/</span> User documentation</span><a :href="withBase('/installation#update')">Keep your CLI up to date <span aria-hidden="true">→</span></a></footer>
  </div>
</template>
