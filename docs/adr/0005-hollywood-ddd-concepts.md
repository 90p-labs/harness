# 0005 Hollywood DDD concepts

## Status

Accepted, 2026-11-07.

## Context

The harness needs a DDD direction for the packs that it ships. The project `jorge07/hollywood` gives the concepts. The CodelyTV DDD example gives a second view. Both are direction, not strict sources.

## Decision

- Take the concepts of hollywood, with no need of the library. Event sourcing is an optional pack.
- Use NestJS file suffixes. `src/` holds only `app/`, `components/` and the business root, default `contexts/`.
- Bounded contexts can nest, for example `src/agency/hhrr/salaries/` and `src/agency/hhrr/events/`. Each one has its own layers.
- Adopt from CodelyTV: event recording, events as the context contract, `shared/` folders, mappers and object mothers.
- Do not adopt from CodelyTV: commands in `domain/`, `equals` by class name, the Criteria pattern as a rule, and a test tree that mirrors `src/`.
- The event outbox is a recommendation in the `ddd-es` docs. It is not a lint rule.

## Consequences

- The `ddd` pack checks the layout and the suffixes with rules.
- Teams that use event sourcing opt in with the `ddd-es` pack.
- The pack docs state the known defects of the sources, and the packs avoid them.
