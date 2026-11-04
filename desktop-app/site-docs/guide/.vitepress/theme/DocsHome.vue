<script setup>
import { withBase } from 'vitepress'
</script>

<template>
  <div class="sandbox-home">
    <section class="sandbox-hero" aria-labelledby="hero-title">
      <div class="hero-copy">
        <h1 id="hero-title">Your workflow.<br>On your desktop.</h1>
        <p class="hero-description">Bring your repositories, coding workflows, and stage output into one desktop app. Configure Soft Factory, start a task, and follow the work as it happens.</p>
        <div class="hero-actions">
          <a class="primary-link" :href="withBase('/installation')">Install the desktop app</a>
          <a class="secondary-link" :href="withBase('/first-run')">Start your first run</a>
        </div>
        <p class="hero-platform">Linux x64 and macOS Apple Silicon packages. Requires Software Factory and Agent Sandbox installed separately.</p>
      </div>
      <div class="hero-note">
        <span class="branch-glyph" aria-hidden="true">⑂</span>
        <p>A repository.<br>A branch.<br>A clear view of each run.</p>
      </div>
    </section>
    <figure class="desktop-preview">
      <a :href="withBase('/screenshots')"><img :src="withBase('/screenshots/new-run-dark.png')" width="1280" height="860" alt="Soft Factory Desktop New run screen with task input and repository navigation" fetchpriority="high" /></a>
      <figcaption>The actual desktop app, captured with a temporary example repository. <a :href="withBase('/screenshots')">Explore the desktop screens.</a></figcaption>
    </figure>
    <section class="desktop-chapters" aria-label="Explore the guide">
      <a :href="withBase('/first-run')"><h2>Start a task</h2><p>Add a local Git repository, choose your configuration, and launch a workflow.</p></a>
      <a :href="withBase('/runs')"><h2>Follow the work</h2><p>Read stage output, inspect the walkthrough, and continue or resume a run.</p></a>
      <a :href="withBase('/configuration')"><h2>Make it yours</h2><p>Choose agents and models, adjust workflow stages, and set your desktop preferences.</p></a>
    </section>
  </div>
</template>
