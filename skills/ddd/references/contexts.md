# Contexts

**Cut the contexts by language, not by table. Contexts talk only through a module or through events.**

## The cut

- A context is one ubiquitous language. When a word changes its meaning, the boundary of a context is there.
- Never cut by table or by record type. A table is a storage detail, not a business term.
- A context holds one or more aggregates. Each aggregate has its own repository.
- Merge two contexts that share one language. Two names for one language cause translation work and no gain.
- A merge moves code, not data. The stores stay where they are until a separate step moves them.

## Layout

- Contexts nest in groups. A group holds contexts, groups and a `shared` folder, and no code of its own (`ddd/parent-no-code`).
- A context never has only one of the layers domain, application and infrastructure (`ddd/context-layers`).
- A `shared` folder holds only the three layers (`ddd/shared-layers`).
- The root shared domain holds only the base types and the integration event interfaces (`ddd/shared-base-only`). The base types are aggregate root, entity, value object, domain event and error.
- Business code does not go to `shared` to avoid a dependency. Move it to a context, or make an event.

## Calls between contexts

- A context calls another context only through its module, or through the events of that context (`ddd/cross-context`).
- A context never imports a repository, an aggregate or a handler of another context.
- The module is the public entry point for a direct call. See `composition-root.md`.
- The module or the event handler that receives a foreign type maps it to the terms of its own context. The domain never uses a foreign type.
- A read that needs data of another context uses a local read model. An event handler fills it from the integration events of that context. See `events.md`.

## Events across contexts

- This file owns the events between contexts.
- An integration event is an interface in the root shared domain. It has the role suffix event and a schema version, and it is a contract.
- The publishing context fires an event of its own domain that implements the interface.
- The event handler of another context imports only the interface. This is the convention, not the lint rule.
- `ddd/cross-context` lets an event handler import any event of another context. A reviewer checks for a foreign domain event by reading the code.
- Every other event stays internal by convention.
- The event handler of the other context maps the integration event to its own terms itself. Then it sends a message to its own handler. See `events.md`.
- A change of an integration event follows the rule of a breaking change in `api-contracts.md`.

## Re-cut

- Make one move in each reviewable step. A reviewer must read the move and see no other change.
- Tests move with their code, in the same step.
- Before you move an engine, record a golden master. See `porting-legacy.md`.
- After each move, run the architecture checks. The rules of this file fail when a move breaks a boundary.
