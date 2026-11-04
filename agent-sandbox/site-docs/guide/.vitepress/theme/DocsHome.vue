<script setup>
import { computed, ref } from 'vue'
import { useData, withBase } from 'vitepress'
import spanish from './home-es.js'

const { lang } = useData()
const isSpanish = computed(() => lang.value.startsWith('es'))
const t = (text) => isSpanish.value ? spanish[text] ?? text : text
const localePath = (path) => withBase(`${isSpanish.value ? '/es' : ''}${path}`)

const selected = ref(0)
const copied = ref(false)
const copyFailed = ref(false)
const stages = computed(() => [
  { title: t('Run'), command: 'agent-sandbox run -b fix-login -a codex \\\n  -q "fix the login redirect loop"', heading: t('Give the agent a separate place to work.'), detail: t('Creates a branch and worktree from your current commit, then runs the agent inside Docker. Changes stay uncommitted for you to review.'), link: '/first-run', label: t('Follow your first run') },
  { title: t('Review'), command: 'agent-sandbox worktree-list\nagent-sandbox worktree-editor -b fix-login', heading: t('Open the worktree and inspect the changes.'), detail: t('Find the recorded worktree and open it in VS Code. You can also open the listed path in your preferred editor.'), link: '/worktrees', label: t('Find and manage worktrees') },
  { title: t('Continue'), command: 'agent-sandbox resume -b fix-login -a codex \\\n  -q "add a regression test for the redirect"', heading: t('Keep working in the same worktree.'), detail: t('Starts another agent invocation with the existing files and commits. Your original working copy remains available.'), link: '/worktrees', label: t('Continue an existing task') }
])
const stage = computed(() => stages.value[selected.value])
function selectStage(index) { selected.value = index; copied.value = false; copyFailed.value = false }
async function copyCommand() {
  try { await navigator.clipboard.writeText(stage.value.command); copied.value = true; copyFailed.value = false }
  catch { copyFailed.value = true }
}
const guides = computed(() => [
  { title: t('Start a task'), description: t('Install the CLI, connect your agent, and run a small change.'), links: [[t('Installation'), '/installation'], [t('Authentication'), '/authentication'], [t('Your first run'), '/first-run']] },
  { title: t('Make it your workflow'), description: t('Set defaults, continue a task, and publish the result.'), links: [[t('Prompts and terminal UI'), '/running'], [t('Configuration'), '/configuration'], [t('Worktrees'), '/worktrees'], [t('Push and pull requests'), '/publishing']] },
  { title: t('Find an answer'), description: t('Look up a flag, adjust the environment, or resolve a failed run.'), links: [[t('Commands and flags'), '/reference'], [t('Images and networking'), '/environment'], [t('Troubleshooting'), '/troubleshooting']] }
])
</script>

<template>
  <div class="sandbox-home">
    <section class="sandbox-hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <h1 id="hero-title">{{ t("Your agent.") }}<br>{{ t("Its own workspace.") }}</h1>
        <p class="hero-description">{{ t("Run Codex, Claude Code, opencode, or pi on your local machine, inside Docker and on a separate Git worktree. Keep working while your agent takes on a task.") }}</p>
        <div class="hero-actions"><a class="primary-link" :href="localePath('/installation')">{{ t("Install agent-sandbox") }}</a><a class="secondary-link" :href="localePath('/first-run')">{{ t("Walk through a first run") }}</a></div>
        <p class="hero-platform">{{ t("Linux x86_64 and macOS (Intel and Apple silicon). Requires Docker and Git.") }}</p>
      </div>
      <div class="hero-note"><span class="branch-glyph" aria-hidden="true">⑂</span><p>{{ t("A branch and working directory for each task.") }}</p><p>{{ t("You review the result.") }}<br>{{ t("You decide what ships.") }}</p></div>
    </section>

    <section class="workspace-section" :aria-label="t('Explore the sandbox workflow')">
      <div class="workflow-picker" role="group" :aria-label="t('Workflow stage')"><button v-for="(item, index) in stages" :key="item.title" type="button" :aria-pressed="selected === index" @click="selectStage(index)"><span class="stage-number" aria-hidden="true">{{ index + 1 }}</span>{{ item.title }}</button><span class="picker-hint">{{ t("See where the work happens") }}</span></div>
      <div class="workspace-map">
        <div class="host-workspace"><span class="location-label">{{ t("Your local machine") }}</span><h2>{{ t("Your working copy") }}</h2><p>{{ t("Your files and current work.") }}<br>{{ t("Keep using your usual tools.") }}</p><span class="path-label">~/projects/your-repo</span></div>
        <div class="worktree-connection" aria-hidden="true"><span></span><b>Git worktree</b><span></span></div>
        <div class="agent-workspace" :class="{ reviewing: selected === 1 }"><div class="agent-location"><span class="location-label">{{ t("Agent’s worktree") }}</span><span class="branch-name">fix-login</span></div><div class="container-boundary"><span class="docker-label">{{ selected === 1 ? t('Review locally') : t('Runs inside Docker') }}</span><h2>{{ selected === 1 ? t('Your editor') : t('Your coding agent') }}</h2><p v-if="selected === 1">{{ t("Inspect the diff and verify the result.") }}</p></div></div>
      </div>
      <p class="map-note">{{ t("Separate files and branch; worktrees share the repository’s Git metadata.") }}</p>
      <div class="command-workbench"><div class="stage-explanation" aria-live="polite"><h3>{{ stage.heading }}</h3><p>{{ stage.detail }}</p><a :href="localePath(stage.link)">{{ stage.label }}</a></div><div class="command-example"><div class="command-heading"><span>{{ stage.title }} {{ t('command') }}</span><button type="button" @click="copyCommand">{{ copied ? t('Copied') : t('Copy command') }}</button></div><pre><code>{{ stage.command }}</code></pre><p class="copy-status" role="status">{{ copied ? t('Command copied to clipboard.') : copyFailed ? t('Select the command above and copy it manually.') : '' }}</p></div></div>
    </section>

    <section class="guide-section" aria-labelledby="guide-title"><div class="guide-heading"><h2 id="guide-title">{{ t("From first task to everyday use.") }}</h2><p>{{ t("Start with a walkthrough, or go straight to the detail you need.") }}</p></div><div class="guide-chapters"><div v-for="guide in guides" :key="guide.title" class="guide-chapter"><h3>{{ guide.title }}</h3><p>{{ guide.description }}</p><ul><li v-for="[label, path] in guide.links" :key="path"><a :href="localePath(path)">{{ label }}</a></li></ul></div></div></section>
    <footer class="home-footer"><span>{{ t("agent-sandbox / User documentation") }}</span><a :href="localePath('/installation#update')">{{ t("Update your CLI") }}</a></footer>
  </div>
</template>
