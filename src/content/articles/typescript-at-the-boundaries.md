---
title: "TypeScript is most useful at the boundaries"
description: "Go beyond annotations. Use types and runtime validation where untrusted data enters your application, and let the rest of your code breathe."
publishedAt: 2026-10-16
author:
  name: Alex Rivera
  role: Senior frontend engineer
category: Development
tags: [typescript, validation, api-design]
artwork: typescript
---

A type annotation can make a promise the runtime never agreed to keep. That is the problem with treating the result of a network request as a trusted object: the compiler sees your declaration, while your application sees whatever the server returned.

The most valuable place to spend type-system effort is where data enters your control. Once a value crosses that boundary safely, the rest of the program becomes easier to write.

## Accept unknown honestly

Suppose a dashboard fetches a project summary. Avoid asserting that the JSON response is already a project. Start with `unknown` and earn a more precise type through validation.

```ts
type Project = { id: string; name: string };

function parseProject(value: unknown): Project {
  if (typeof value !== "object" || value === null) {
    throw new Error("Expected a project object");
  }
  if (
    !("id" in value) ||
    typeof value.id !== "string" ||
    !("name" in value) ||
    typeof value.name !== "string"
  ) {
    throw new Error("Project requires a string id and name");
  }
  return { id: value.id, name: value.name };
}
```

For a larger schema, a runtime validation library can reduce repetition and infer the TypeScript type from the same definition. The important property is that the definition is executed, rather than erased during compilation.

## Model the states that matter

Boundaries are not only network connections. A user interface also moves between states. Separate booleans for loading, failure, and success allow combinations that do not make sense.

```ts
type RemoteProject =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; project: Project };
```

A discriminated union makes the relationship explicit. In the ready branch, the project exists. In the error branch, there is a message to display. Consumers no longer need to guess which fields belong together.

## Keep errors useful

A validation failure should tell you which boundary failed without copying private payloads into logs. Capture the endpoint, a request identifier, and a safe summary of the missing field. Give the person using the product a clear next step.

Do not silently turn every malformed response into an empty list. That hides a broken contract behind a plausible interface. An explicit error state is often more trustworthy than a successful screen with missing information.

## Let inference do the routine work

After validation, pass domain values through small functions. Let TypeScript infer local variables and straightforward return values. Add explicit return types at public module boundaries, where they document a contract and prevent accidental changes.

A successful type design does not require a clever generic on every line. It places a few strong checks where mistakes would otherwise spread, then makes ordinary application code feel ordinary again.
