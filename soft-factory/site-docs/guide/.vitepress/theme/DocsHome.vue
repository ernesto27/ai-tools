<script setup>
import { ref } from 'vue'
import { withBase } from 'vitepress'

const command = 'software-factory'
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
  { number: '01', title: 'Set up your factory', description: 'Install the CLI and take a local task through the workflow.', links: [
    { text: 'Installation', path: '/installation' },
    { text: 'Your first run', path: '/first-run' }
  ] },
  { number: '02', title: 'Build your workflow', description: 'Choose stages, supply context, and continue existing work.', links: [
    { text: 'Configuration', path: '/configuration' },
    { text: 'Workflow', path: '/workflow' },
    { text: 'Supporting documents', path: '/documents' },
    { text: 'Jira tasks', path: '/jira' },
    { text: 'Google Drive setup', path: '/google-drive' }
  ] },
  { number: '03', title: 'Understand the result', description: 'Read reports, find a command, or resolve a failed run.', links: [
    { text: 'Reports and logs', path: '/reports' },
    { text: 'Commands and flags', path: '/reference' },
    { text: 'Troubleshooting', path: '/troubleshooting' }
  ] }
]
</script>

<template>
  <div class="sandbox-home">
    <section class="sandbox-hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <p class="eyebrow"><span class="status-mark" aria-hidden="true"></span> Coding agents / Reviewed changes</p>
        <h1 id="hero-title">One task.<br>A complete workflow.</h1>
        <p class="hero-description">Give your agent a task. Soft Factory coordinates implementation, code review, security review, risk classification, and a walkthrough of the final changes through Agent Sandbox.</p>
        <div class="hero-actions">
          <a class="primary-link" :href="withBase('/installation')">Get started <span aria-hidden="true">↗</span></a>
          <a class="secondary-link" :href="withBase('/first-run')">Walk through a first run <span aria-hidden="true">→</span></a>
        </div>
        <p class="hero-platform">Linux x86_64 <span aria-hidden="true">·</span> Agent Sandbox <span aria-hidden="true">·</span> Docker</p>
      </div>
      <div class="workspace-panel">
        <div class="panel-heading"><span class="terminal-symbol" aria-hidden="true">&gt;_</span><span>From task to reviewed changes</span><span class="panel-label">CLI</span></div>
        <div class="command-example">
          <div class="command-heading"><span>Run from your configured project</span><button type="button" @click="copyCommand">{{ copyLabel }}</button></div>
          <pre><span class="shell-prompt" aria-hidden="true">$ </span><code>{{ command }}</code></pre>
          <span class="copy-status" role="status">{{ copyLabel === 'Copied' ? 'Command copied to clipboard.' : '' }}</span>
        </div>
        <div class="workspace-map" role="group" aria-label="The implementation and review stages">
          <div class="workspace-node"><span class="node-marker" aria-hidden="true"></span><div><span class="node-label">Local task or Jira issue</span><strong>Implement in a sandbox worktree</strong></div><span class="node-tag">01</span></div>
          <div class="worktree-connection"><span aria-hidden="true">↳</span> code review → security review → risk</div>
          <div class="workspace-node sandbox-node"><span class="node-marker" aria-hidden="true"></span><div><span class="node-label">Reports and verification</span><strong>Explain the final changes</strong></div><span class="node-tag">05</span></div>
        </div>
        <div class="panel-footnote"><span aria-hidden="true">↗</span> Review findings, corrections, and remaining verification gaps.</div>
      </div>
    </section>
    <div class="agent-strip" aria-label="Task and context sources"><span class="eyebrow">Bring your context</span><div><span>Local tasks</span><span>Jira</span><span>Google Docs</span><span>Custom skills</span></div></div>
    <section class="workflow-section" aria-labelledby="workflow-title">
      <div class="section-heading"><p class="eyebrow">The workflow</p><h2 id="workflow-title">From requirements to an explained result.</h2></div>
      <ol class="workflow-steps">
        <li><span class="step-number">01 /</span><h3>Describe and implement</h3><p>Supply a task and supporting documents. The selected agent works in its sandbox branch.</p></li>
        <li><span class="step-number">02 /</span><h3>Review and correct</h3><p>Code and security reviews apply corrections and verify the result, with up to three rounds each.</p></li>
        <li><span class="step-number">03 /</span><h3>Assess and understand</h3><p>Read the risk classification and final walkthrough, then inspect the changes before accepting them.</p></li>
      </ol>
    </section>
    <section class="guide-section" aria-labelledby="guide-title">
      <div class="section-heading"><p class="eyebrow">The user guide</p><h2 id="guide-title">Start here. Come back as you need.</h2></div>
      <div class="guide-chapters"><div v-for="chapter in chapters" :key="chapter.number" class="guide-chapter">
        <span class="chapter-number">{{ chapter.number }}</span>
        <div class="chapter-intro"><h3>{{ chapter.title }}</h3><p>{{ chapter.description }}</p></div>
        <div class="chapter-links"><a v-for="link in chapter.links" :key="link.path" :href="withBase(link.path)">{{ link.text }} <span aria-hidden="true">↗</span></a></div>
      </div></div>
    </section>
    <footer class="home-footer"><span>Soft Factory <span class="footer-divider">/</span> User documentation</span><a :href="withBase('/installation#update')">Keep your CLI up to date <span aria-hidden="true">→</span></a></footer>
  </div>
</template>
