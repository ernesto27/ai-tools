---
title: "Performance starts with a better question"
description: "Before reaching for a cache, find out where the time goes. A measured approach to making your application feel faster for real people."
publishedAt: 2026-10-08
author:
  name: Sam Okafor
  role: Web performance engineer
category: Performance
tags: [web-performance, debugging, measurement]
artwork: performance
---

“The page is slow” describes an experience, not yet a diagnosis. It could mean the first heading takes too long to appear, the search box ignores the first keystroke, or the save button offers no feedback after a click. Each problem points toward a different fix.

Before choosing an optimization, define the moment that disappoints the person using the page.

## Pick a journey and a budget

Start with a specific journey: opening an article from a shared link on a midrange phone over a mobile connection. Decide which milestones matter. Seeing the title, reading the opening paragraph, and interacting with navigation are useful starting points.

Use a performance trace to connect those moments to network activity, scripting, layout, and rendering. A single lab score can help track a trend, but it cannot explain every part of the experience.

Write down a baseline with the device and network conditions. Without that context, a faster result might simply reflect a faster machine.

## Follow the critical path

Look for work that must finish before the first useful content appears. A static article should not need a client-side API request just to render its title. A decorative script should not compete with the text for early network bandwidth.

For a reading site, common improvements include:

- Deliver the article as HTML in the initial response.
- Serve only the font weights and character sets the design uses.
- Reserve image dimensions to prevent layout shifts.
- Defer scripts that are unrelated to the current task.

These changes reduce work before adding mechanisms to hide it.

## Measure an improvement fairly

Run the same journey several times under the same conditions. Compare the distribution rather than celebrating the best run. Then check a second device size and a slower connection to avoid optimizing only for your laptop.

Where real-user measurement is available, segment it. A global average may hide a poor experience on a particular device or route. Respect privacy and collect only the information needed to understand the problem.

A useful result is a claim like: the article title appears earlier under this mobile profile because a render-blocking dependency was removed. It connects observation, change, and reason.

## Cache after you understand

Caching can be the right answer when repeated work dominates. It also adds invalidation rules, memory use, and the possibility of stale responses. Before adding it, decide what freshness means and who owns the invalidation path.

If a slow database query comes from a missing index, a cache might hide the symptom while leaving the expensive operation ready to return during a cold start.

Optimization works best as a sequence of small experiments. Ask a concrete question, measure the current experience, remove the most relevant obstacle, and repeat. The best performance work often leaves the system simpler than it found it.
