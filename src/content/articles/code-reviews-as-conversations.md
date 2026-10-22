---
title: "Better code reviews start with a conversation"
description: "Turn review from a gate into a shared practice. Small changes in how we give feedback can make a meaningful difference to the code and the team."
publishedAt: 2026-10-04
author:
  name: Jordan Lee
  role: Engineering lead
category: Engineering culture
tags: [code-review, collaboration, developer-experience]
artwork: reviews
---

A pull request can be technically correct and still be exhausting to review. The author has been thinking about a problem for days. The reviewer arrives with a diff and a few minutes between meetings. Much of the friction comes from that uneven context.

Better reviews begin by making the problem easy to understand, then treating feedback as a conversation about the work.

## Give the reviewer a starting point

Open the description with the problem and the resulting behavior. Mention the important constraint, the alternative you considered, and how you checked the result. Avoid narrating every file in the diff.

A useful description might say: “Orders with an expired reservation could still reach payment. This change checks the reservation before authorization and returns a recoverable error. The integration test covers an expiry between checkout and confirmation.”

Now the reviewer knows where to focus. The code has a purpose against which it can be evaluated.

## Separate requirements from preferences

Not every comment carries the same weight. Say when something blocks correctness, when it is a suggestion, and when it is simply a question. The distinction helps the author respond without guessing which details are negotiable.

Instead of “Use a map here,” try “Suggestion: a map could make the repeated lookup easier to follow. I do not expect a performance difference at this size.” For a correctness concern, be direct about the failing case.

Do not soften an important bug into a mysterious hint. Clear feedback is kinder than making someone discover what you already know.

## Review the change at the right level

First check whether the design solves the problem and whether the failure paths are handled. Then examine naming and local clarity. Let automated formatting handle the mechanical details.

When the diff is too large to reason about, ask whether it can be split along a meaningful boundary. A preparatory refactor can often land separately from the behavior change. Splitting arbitrary lines into several pull requests only relocates the confusion.

> The purpose of review is to improve the change and share understanding of the system.

This means the reviewer also has something to learn. Ask why a choice was made before assuming the author overlooked the alternative.

## Know when to leave the comment box

If a thread repeats the same disagreement, a short conversation can reveal the missing assumption. Bring the decision back into the pull request afterward so future readers understand the outcome.

Notice recurring feedback across reviews. If every change triggers a debate about error handling, the team may need a documented convention rather than another round of comments.

A good review leaves the code clearer and both people better informed. Speed comes from shared context and trust, not from making the gate narrower.
