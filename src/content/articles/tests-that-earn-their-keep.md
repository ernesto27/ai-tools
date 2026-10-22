---
title: "Write tests that earn their keep"
description: "A useful test suite gives you confidence to change things. Focus on behavior, meaningful boundaries, and failures that tell you what matters."
publishedAt: 2026-10-12
author:
  name: Alex Rivera
  role: Senior frontend engineer
category: Development
tags: [testing, maintainability, developer-experience]
artwork: testing
---

A team can have thousands of tests and still hesitate before a release. That usually means the suite is measuring the wrong things. It proves that the current implementation resembles yesterday's implementation, without proving that a person can complete an important task.

Useful tests protect an observable promise. They should fail when that promise breaks and stay quiet when you change how it is fulfilled.

## Name the promise first

Before writing a test, finish this sentence: a customer should be able to do this, even when that happens. For an invoice calculator, the promise might be that discounts apply before tax and totals are rounded once at the end.

```ts
it("applies the discount before calculating tax", () => {
  const total = calculateInvoice({
    subtotalInCents: 10_000,
    discountPercent: 10,
    taxPercent: 20,
  });
  expect(total).toBe(10_800);
});
```

That test remains useful if you replace three internal helpers with one function. A test that asserts the order of calls to those helpers would instead turn the refactor into unnecessary maintenance.

## Choose boundaries deliberately

Fast unit tests work well for deterministic domain rules. Integration tests are valuable where two parts must agree on a contract: a query and its database schema, a form and its validation, or a route and its response format.

Use a small number of browser tests for the paths where the assembled product matters. Opening an article, searching by tag, and following the result is more meaningful than checking whether an internal search handler ran.

Avoid mocking every dependency by default. Each mock is another model of the system that can drift from reality. Use one where it isolates a real source of nondeterminism or makes an otherwise impractical failure reproducible.

## Test the failure a person can encounter

A useful suite includes the empty state, the rejected request, and the invalid input. Consider what the user can do next. Does a failed payment preserve the order? Does an empty search offer a way back?

Pick failures using the product's history and consequences. A complicated test for an impossible state is less valuable than a simple test for a mistake people make every week.

## Make a failure easy to diagnose

Keep setup close to the assertion, name values for the scenario, and avoid large fixtures full of irrelevant properties. A failure should explain what promise was broken without requiring a tour of the test utilities.

When a test repeatedly fails for reasons unrelated to its promise, fix the isolation or remove the test. Quarantining it indefinitely teaches everyone to ignore the suite.

The goal is not a particular test count. It is a short, reliable feedback loop that lets your team improve the product with less hesitation.
