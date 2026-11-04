<script setup>
import { computed, ref } from 'vue'
import { withBase } from 'vitepress'

const selected = ref(0)
const copied = ref(false)
const copyFailed = ref(false)
const stages = [
  { title: 'Run', command: 'agent-sandbox run -b fix-login -a codex \\\n  -q "fix the login redirect loop"', heading: 'Give the agent a separate place to work.', detail: 'Creates a branch and worktree from your current commit, then runs the agent inside Docker. Changes stay uncommitted for you to review.', link: '/first-run', label: 'Follow your first run' },
  { title: 'Review', command: 'agent-sandbox worktree-list\nagent-sandbox worktree-editor -b fix-login', heading: 'Open the worktree and inspect the changes.', detail: 'Find the recorded worktree and open it in VS Code. You can also open the listed path in your preferred editor.', link: '/worktrees', label: 'Find and manage worktrees' },
  { title: 'Continue', command: 'agent-sandbox resume -b fix-login -a codex \\\n  -q "add a regression test for the redirect"', heading: 'Keep working in the same worktree.', detail: 'Starts another agent invocation with the existing files and commits. Your original working copy remains available.', link: '/worktrees', label: 'Continue an existing task' }
]
const stage = computed(() => stages[selected.value])
function selectStage(index) { selected.value = index; copied.value = false; copyFailed.value = false }
async function copyCommand() {
  try { await navigator.clipboard.writeText(stage.value.command); copied.value = true; copyFailed.value = false }
  catch { copyFailed.value = true }
}
const guides = [
  { title: 'Start a task', description: 'Install the CLI, connect your agent, and run a small change.', links: [['Installation', '/installation'], ['Authentication', '/authentication'], ['Your first run', '/first-run']] },
  { title: 'Make it your workflow', description: 'Set defaults, continue a task, and publish the result.', links: [['Prompts and terminal UI', '/running'], ['Configuration', '/configuration'], ['Worktrees', '/worktrees'], ['Push and pull requests', '/publishing']] },
  { title: 'Find an answer', description: 'Look up a flag, adjust the environment, or resolve a failed run.', links: [['Commands and flags', '/reference'], ['Images and networking', '/environment'], ['Troubleshooting', '/troubleshooting']] }
]
</script>

<template>
  <div class="sandbox-home">
    <section class="sandbox-hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <h1 id="hero-title">Your agent.<br>Its own workspace.</h1>
        <p class="hero-description">Run Codex, Claude Code, opencode, or pi on your local machine, inside Docker and on a separate Git worktree. Keep working while your agent takes on a task.</p>
        <div class="hero-actions"><a class="primary-link" :href="withBase('/installation')">Install agent-sandbox</a><a class="secondary-link" :href="withBase('/first-run')">Walk through a first run</a></div>
        <p class="hero-platform">Linux x86_64 and macOS (Intel and Apple silicon). Requires Docker and Git.</p>
      </div>
      <div class="hero-note"><span class="branch-glyph" aria-hidden="true">⑂</span><p>A branch and working directory for each task.</p><p>You review the result.<br>You decide what ships.</p></div>
    </section>

    <section class="workspace-section" aria-label="Explore the sandbox workflow">
      <div class="workflow-picker" role="group" aria-label="Workflow stage"><button v-for="(item, index) in stages" :key="item.title" type="button" :aria-pressed="selected === index" @click="selectStage(index)"><span class="stage-number" aria-hidden="true">{{ index + 1 }}</span>{{ item.title }}</button><span class="picker-hint">See where the work happens</span></div>
      <div class="workspace-map">
        <div class="host-workspace"><span class="location-label">Your local machine</span><h2>Your working copy</h2><p>Your files and current work.<br>Keep using your usual tools.</p><span class="path-label">~/projects/your-repo</span></div>
        <div class="worktree-connection" aria-hidden="true"><span></span><b>Git worktree</b><span></span></div>
        <div class="agent-workspace" :class="{ reviewing: selected === 1 }"><div class="agent-location"><span class="location-label">Agent’s worktree</span><span class="branch-name">fix-login</span></div><div class="container-boundary"><span class="docker-label">{{ selected === 1 ? 'Review locally' : 'Runs inside Docker' }}</span><h2>{{ selected === 1 ? 'Your editor' : 'Your coding agent' }}</h2><p v-if="selected === 1">Inspect the diff and verify the result.</p></div></div>
      </div>
      <p class="map-note">Separate files and branch; worktrees share the repository’s Git metadata.</p>
      <div class="command-workbench"><div class="stage-explanation" aria-live="polite"><h3>{{ stage.heading }}</h3><p>{{ stage.detail }}</p><a :href="withBase(stage.link)">{{ stage.label }}</a></div><div class="command-example"><div class="command-heading"><span>{{ stage.title }} command</span><button type="button" @click="copyCommand">{{ copied ? 'Copied' : 'Copy command' }}</button></div><pre><code>{{ stage.command }}</code></pre><p class="copy-status" role="status">{{ copied ? 'Command copied to clipboard.' : copyFailed ? 'Select the command above and copy it manually.' : '' }}</p></div></div>
    </section>

    <section class="guide-section" aria-labelledby="guide-title"><div class="guide-heading"><h2 id="guide-title">From first task to everyday use.</h2><p>Start with a walkthrough, or go straight to the detail you need.</p></div><div class="guide-chapters"><div v-for="guide in guides" :key="guide.title" class="guide-chapter"><h3>{{ guide.title }}</h3><p>{{ guide.description }}</p><ul><li v-for="[label, path] in guide.links" :key="path"><a :href="withBase(path)">{{ label }}</a></li></ul></div></div></section>
    <footer class="home-footer"><span>agent-sandbox / User documentation</span><a :href="withBase('/installation#update')">Update your CLI</a></footer>
  </div>
</template>
