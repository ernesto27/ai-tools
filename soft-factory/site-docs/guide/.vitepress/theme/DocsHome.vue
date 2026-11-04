<script setup>
import { computed, ref } from 'vue'
import { withBase } from 'vitepress'

const selected = ref(0)
const copied = ref(false)
const copyFailed = ref(false)
const stages = [
  { title: 'Run', command: 'software-factory', heading: 'Take a task through the complete workflow.', detail: 'Reads your configured task and supporting documents, implements in a sandbox worktree, then runs code review, security review, risk classification, and a final walkthrough.', link: '/first-run', label: 'Follow your first run' },
  { title: 'Inspect', command: 'agent-sandbox worktree-list', heading: 'Read the reports and inspect the changes.', detail: 'Find your sandbox worktree to inspect the diff. Read the risk classification and final walkthrough in the invocation logs printed by Soft Factory, including any remaining findings or verification gaps.', link: '/reports', label: 'Understand reports and logs' },
  { title: 'Continue', command: 'software-factory continue', heading: 'Keep working in the existing sandbox branch.', detail: 'Runs implementation and the enabled review stages again using resume.branch. Keep the original task in run.filePrompt, or supply --jira again for a Jira task.', link: '/workflow', label: 'Continue an existing task' }
]
const stage = computed(() => stages[selected.value])
function selectStage(index) { selected.value = index; copied.value = false; copyFailed.value = false }
async function copyCommand() {
  try { await navigator.clipboard.writeText(stage.value.command); copied.value = true; copyFailed.value = false }
  catch { copyFailed.value = true }
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
        <h1 id="hero-title">One task.<br>A complete workflow.</h1>
        <p class="hero-description">Give your agent a task. Soft Factory coordinates implementation, code review, security review, risk classification, and a walkthrough of the final changes through Agent Sandbox.</p>
        <div class="hero-actions"><a class="primary-link" :href="withBase('/installation')">Install Soft Factory</a><a class="secondary-link" :href="withBase('/first-run')">Walk through a first run</a></div>
        <p class="hero-platform">Linux x86_64 and macOS (Intel and Apple silicon). Requires Agent Sandbox, Docker, and Git.</p>
      </div>
      <div class="hero-note"><span class="branch-glyph" aria-hidden="true">⑂</span><p>Implementation, reviews, risk classification, and a final walkthrough.</p><p>You review the result.<br>You decide what ships.</p></div>
    </section>

    <section class="workspace-section" aria-label="Explore the Soft Factory workflow">
      <div class="workflow-picker" role="group" aria-label="Workflow stage"><button v-for="(item, index) in stages" :key="item.title" type="button" :aria-pressed="selected === index" @click="selectStage(index)"><span class="stage-number" aria-hidden="true">{{ index + 1 }}</span>{{ item.title }}</button><span class="picker-hint">See where the work happens</span></div>
      <div class="workspace-map">
        <div class="host-workspace"><span class="location-label">Your local machine</span><h2>Your working copy</h2><p>Your task and supporting documents.<br>Configure the workflow here.</p><span class="path-label">software-factory.json</span></div>
        <div class="worktree-connection" aria-hidden="true"><span></span><b>Git worktree</b><span></span></div>
        <div class="agent-workspace" :class="{ reviewing: selected === 1 }"><div class="agent-location"><span class="location-label">Agent’s worktree</span><span class="branch-name">factory-task</span></div><div class="container-boundary"><span class="docker-label">{{ selected === 1 ? 'Review locally' : 'Runs inside Docker' }}</span><h2>{{ selected === 1 ? 'Your editor' : 'Implementation and reviews' }}</h2><p v-if="selected === 1">Inspect the diff and verify the result.</p></div></div>
      </div>
      <p class="map-note">The default workflow uses run.branch; continue uses resume.branch. Review stages can apply corrections.</p>
      <div class="command-workbench"><div class="stage-explanation" aria-live="polite"><h3>{{ stage.heading }}</h3><p>{{ stage.detail }}</p><a :href="withBase(stage.link)">{{ stage.label }}</a></div><div class="command-example"><div class="command-heading"><span>{{ stage.title }} command</span><button type="button" @click="copyCommand">{{ copied ? 'Copied' : 'Copy command' }}</button></div><pre><code>{{ stage.command }}</code></pre><p class="copy-status" role="status">{{ copied ? 'Command copied to clipboard.' : copyFailed ? 'Select the command above and copy it manually.' : '' }}</p></div></div>
    </section>

    <section class="guide-section" aria-labelledby="guide-title"><div class="guide-heading"><h2 id="guide-title">Start here. Come back as you need.</h2><p>Start with a walkthrough, or go straight to the detail you need.</p></div><div class="guide-chapters"><div v-for="chapter in chapters" :key="chapter.number" class="guide-chapter"><h3>{{ chapter.title }}</h3><p>{{ chapter.description }}</p><ul><li v-for="link in chapter.links" :key="link.path"><a :href="withBase(link.path)">{{ link.text }}</a></li></ul></div></div></section>
    <footer class="home-footer"><span>Soft Factory / User documentation</span><a :href="withBase('/installation#update')">Update your CLI</a></footer>
  </div>
</template>
