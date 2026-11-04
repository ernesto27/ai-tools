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
  { title: t('Run'), command: 'software-factory', heading: t('Take a task through the complete workflow.'), detail: t('Reads your configured task and supporting documents, implements in a sandbox worktree, then runs code review, security review, risk classification, and a final walkthrough.'), link: '/first-run', label: t('Follow your first run') },
  { title: t('Inspect'), command: 'agent-sandbox worktree-list', heading: t('Read the reports and inspect the changes.'), detail: t('Find your sandbox worktree to inspect the diff. Read the risk classification and final walkthrough in the invocation logs printed by Soft Factory, including any remaining findings or verification gaps.'), link: '/reports', label: t('Understand reports and logs') },
  { title: t('Continue'), command: 'software-factory continue', heading: t('Keep working in the existing sandbox branch.'), detail: t('Runs implementation and the enabled review stages again using resume.branch. Keep the original task in run.filePrompt, or supply --jira again for a Jira task.'), link: '/workflow', label: t('Continue an existing task') }
])
const stage = computed(() => stages.value[selected.value])
function selectStage(index) { selected.value = index; copied.value = false; copyFailed.value = false }
async function copyCommand() {
  try { await navigator.clipboard.writeText(stage.value.command); copied.value = true; copyFailed.value = false }
  catch { copyFailed.value = true }
}
const chapters = computed(() => [
  { number: '01', title: t('Set up your factory'), description: t('Install the CLI and take a local task through the workflow.'), links: [
    { text: t('Installation'), path: '/installation' },
    { text: t('Your first run'), path: '/first-run' }
  ] },
  { number: '02', title: t('Build your workflow'), description: t('Choose stages, supply context, and continue existing work.'), links: [
    { text: t('Configuration'), path: '/configuration' },
    { text: t('Workflow'), path: '/workflow' },
    { text: t('Supporting documents'), path: '/documents' },
    { text: t('Jira tasks'), path: '/jira' },
    { text: t('Google Drive setup'), path: '/google-drive' }
  ] },
  { number: '03', title: t('Understand the result'), description: t('Read reports, find a command, or resolve a failed run.'), links: [
    { text: t('Reports and logs'), path: '/reports' },
    { text: t('Commands and flags'), path: '/reference' },
    { text: t('Troubleshooting'), path: '/troubleshooting' }
  ] }
])
</script>

<template>
  <div class="sandbox-home">
    <section class="sandbox-hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <h1 id="hero-title">{{ t("One task.") }}<br>{{ t("A complete workflow.") }}</h1>
        <p class="hero-description">{{ t("Give your agent a task. Soft Factory coordinates implementation, code review, security review, risk classification, and a walkthrough of the final changes through Agent Sandbox.") }}</p>
        <div class="hero-actions"><a class="primary-link" :href="localePath('/installation')">{{ t("Install Soft Factory") }}</a><a class="secondary-link" :href="localePath('/first-run')">{{ t("Walk through a first run") }}</a></div>
        <p class="hero-platform">{{ t("Linux x86_64 and macOS (Intel and Apple silicon). Requires Agent Sandbox, Docker, and Git.") }}</p>
      </div>
      <div class="hero-note"><span class="branch-glyph" aria-hidden="true">⑂</span><p>{{ t("Implementation, reviews, risk classification, and a final walkthrough.") }}</p><p>{{ t("You review the result.") }}<br>{{ t("You decide what ships.") }}</p></div>
    </section>

    <section class="workspace-section" :aria-label="t('Explore the Soft Factory workflow')">
      <div class="workflow-picker" role="group" :aria-label="t('Workflow stage')"><button v-for="(item, index) in stages" :key="item.title" type="button" :aria-pressed="selected === index" @click="selectStage(index)"><span class="stage-number" aria-hidden="true">{{ index + 1 }}</span>{{ item.title }}</button><span class="picker-hint">{{ t("See where the work happens") }}</span></div>
      <div class="workspace-map">
        <div class="host-workspace"><span class="location-label">{{ t("Your local machine") }}</span><h2>{{ t("Your working copy") }}</h2><p>{{ t("Your task and supporting documents.") }}<br>{{ t("Configure the workflow here.") }}</p><span class="path-label">software-factory.json</span></div>
        <div class="worktree-connection" aria-hidden="true"><span></span><b>Git worktree</b><span></span></div>
        <div class="agent-workspace" :class="{ reviewing: selected === 1 }"><div class="agent-location"><span class="location-label">{{ t("Agent’s worktree") }}</span><span class="branch-name">factory-task</span></div><div class="container-boundary"><span class="docker-label">{{ selected === 1 ? t('Review locally') : t('Runs inside Docker') }}</span><h2>{{ selected === 1 ? t('Your editor') : t('Implementation and reviews') }}</h2><p v-if="selected === 1">{{ t("Inspect the diff and verify the result.") }}</p></div></div>
      </div>
      <div class="command-workbench"><div class="stage-explanation" aria-live="polite"><h3>{{ stage.heading }}</h3><p>{{ stage.detail }}</p><a :href="localePath(stage.link)">{{ stage.label }}</a></div><div class="command-example"><div class="command-heading"><span>{{ stage.title }} {{ t('command') }}</span><button type="button" @click="copyCommand">{{ copied ? t('Copied') : t('Copy command') }}</button></div><pre><code>{{ stage.command }}</code></pre><p class="copy-status" role="status">{{ copied ? t('Command copied to clipboard.') : copyFailed ? t('Select the command above and copy it manually.') : '' }}</p></div></div>
    </section>

    <section class="guide-section" aria-labelledby="guide-title"><div class="guide-heading"><h2 id="guide-title">{{ t("Start here. Come back as you need.") }}</h2><p>{{ t("Start with a walkthrough, or go straight to the detail you need.") }}</p></div><div class="guide-chapters"><div v-for="chapter in chapters" :key="chapter.number" class="guide-chapter"><h3>{{ chapter.title }}</h3><p>{{ chapter.description }}</p><ul><li v-for="link in chapter.links" :key="link.path"><a :href="localePath(link.path)">{{ link.text }}</a></li></ul></div></div></section>
    <footer class="home-footer"><span>{{ t("Soft Factory / User documentation") }}</span><a :href="localePath('/installation#update')">{{ t("Update your CLI") }}</a></footer>
  </div>
</template>
