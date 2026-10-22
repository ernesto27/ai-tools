---
title: "Designing systems that stay simple"
description: "Complexity is easy to add and hard to remove. A practical guide to building systems that leave room for change, without getting in your way."
publishedAt: 2026-10-20
author:
  name: Maya Chen
  role: Staff software engineer
category: Architecture
tags: [system-design, simplicity, maintainability]
artwork: systems
featured: true
---

The first version of a system is usually easy to understand. One service accepts a request, a function applies a rule, and a database remembers the result. Six months later, the same request travels through three queues and a service whose name nobody can explain.

Complexity rarely arrives as one obviously bad decision. It accumulates through reasonable local choices. Keeping a system simple means paying attention to what those choices add up to.

## Start with the change you expect

Imagine a small order-processing service. The team knows that payment providers might change, but its product catalog is stable. A useful boundary separates the payment provider from the order workflow. A generic plugin architecture for every product field solves a problem the team does not yet have.

Write down the two or three changes you actually expect. For each proposed abstraction, ask which of those changes it makes easier. If the answer is vague, keep the code direct until a real variation appears.

This is not a rule against planning. It is a way to make plans specific enough to evaluate.

## Make boundaries carry meaning

A boundary should hide a decision, rather than simply move code into a different folder. For payments, that decision might be how a provider represents an authorization. The rest of the application should speak in the language of the order.

```ts
interface PaymentGateway {
  authorize(input: {
    orderId: string;
    amountInCents: number;
    currency: string;
  }): Promise<Authorization>;
}
```

Keep the interface small. Adding every provider capability creates a second provider SDK that your team now has to maintain. Expose what your workflow needs and let the adapter own the translation.

The important test is behavioral: can you replace the provider without rewriting the business rule? Counting interfaces tells you much less.

## Keep the happy path visible

An order workflow should read like an order workflow. Validate the request, reserve inventory, authorize payment, and record the result. Handle failures close to the step that can produce them.

If the reader must jump through six generic helpers to discover that sequence, the abstractions are hiding the most important information. Consider inlining a helper whose only purpose is to rename a single expression.

> Simple code makes the important decisions easy to find.

The same principle applies to operations. A single deployable process with clear internal boundaries can be easier to recover than several independently deployed services with unclear ownership. Split deployment when an operational requirement justifies its cost.

## Budget for understanding

During design review, ask a teammate to trace one request. Note every place where they need extra context. Those pauses identify complexity more reliably than a diagram with tidy boxes.

Review the system after the next feature ships. Which abstractions helped? Which demanded configuration without reducing work? Delete the latter while the reasoning is still fresh.

Simplicity is maintained through repeated small corrections. The goal is a system your team can change confidently, including on a quiet Friday afternoon when the original author is away.
