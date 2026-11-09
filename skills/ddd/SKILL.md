---
name: ddd
description: "The DDD layout that the harness packs ddd and ddd-es check: contexts, layers, dependency direction, CQRS folders, naming and the kernel, each rule with its id. Use when you create, move or review a file in a bounded context, or set up the lint rules of a DDD project."
argument-hint: "[language]"
---

# DDD layout

The harness checks a domain driven design project with the rules of the packs ddd and ddd-es. This page gives the layout that those rules expect, with the rule id beside each rule. It names no language: the folder names, the file names and the tool of each language are in `references/`.

Language: `$ARGUMENTS`.

## Decision tree

```
What do you need?
├── the folder names, the file names or the tool of one language: read only that file
│   ├── TypeScript → references/typescript.md
│   ├── PHP → references/php.md
│   ├── Python → references/python.md
│   └── Go → references/go.md
├── the name of a context, an aggregate or an event → "Language"
├── a new context, or a call between two contexts → "Contexts", then "Dependencies"
├── the layer of a new file → "Layers", then "Naming"
├── whether an import is allowed → "Dependencies"
├── a command, a query or an event handler → "Application"
├── an aggregate, a value object, an event or a repository → "Domain model"
├── an event sourced aggregate or a projection → "Event sourcing"
└── which rules the harness checks in this language → "Coverage"
```

## Language

Read `CONTEXT.md`, the glossary or `docs/adr/` of the project first, when one exists. Name each context, aggregate, command and event with the terms of that file. An event name is a fact in the past tense, such as order placed (`ddd/event-past-tense`).

## Contexts

- The business root holds the bounded contexts, the `shared` folders and one kernel.
- A folder with layer folders, or with a module at its top level, is a context. Another folder is a group of contexts.
- A context splits its code in the layers `domain`, `application` and `infrastructure`. A context with only one of them is a finding (`ddd/context-layers`).
- Each context has exactly one module at its root. The module wires the context and is its public entry point (`ddd/module-per-context`).
- The business root has exactly one kernel, which wires the modules (`ddd/one-kernel`).
- A group holds contexts, groups and a `shared` folder, and no code of its own (`ddd/parent-no-code`).
- Each source file and each dependency belongs to a known context or layer. A file that the tool cannot place is a finding, never a silent pass (`ddd/src-roots`).

## Layers

| Layer | Holds |
|---|---|
| `domain` | aggregates, entities, value objects, domain events, errors and the repository ports |
| `application` | commands, queries, their handlers and the event handlers |
| `infrastructure` | the repository adapters, the mappers, the controllers, the schemas and the other adapters |

- A `shared` folder holds only the three layers (`ddd/shared-layers`).
- The root `shared` domain holds only the base types: aggregate root, entity, value object, domain event and error (`ddd/shared-base-only`).

## Dependencies

```
infrastructure ──> application ──> domain ──> shared domain, allowed libraries
```

- The domain uses only itself, the shared domain and the libraries that the project allows. It never uses the application layer, the infrastructure layer, a framework, an ORM or the event bus (`ddd/domain-purity`).
- The application layer uses the domain, never the infrastructure layer (`ddd/layer-direction`).
- A context uses another context only through its public entry point: the module of a context uses the module of another, and an event handler uses the events of another (`ddd/cross-context`).

## Application

- The application layer holds only three kinds of folder: commands, queries and event handlers (`ddd/cqrs-folders`).
- Each command or query folder holds exactly one message and one handler (`ddd/handler-pair`).
- A command handler returns nothing (`ddd/command-handler-returns`).
- A query handler writes nothing: no save, delete, persist, publish or dispatch (`ddd/query-handler-no-write`).
- A command handler publishes the events after the repository saves the aggregate (`ddd/publish-after-save`).

## Domain model

- An aggregate exposes no public writable state (`ddd/aggregate-encapsulation`).
- A value object has an `equals` method (`ddd/value-object-equals`) and a factory, never a public constructor (`ddd/value-object-factory`).
- An event is immutable (`ddd/event-readonly`).
- The repository port is in `domain`, and its adapter is in `infrastructure` (`ddd/repository-port`).
- A mapper is in `infrastructure` (`ddd/mapper-location`).

## Naming

A file in a layer tells its role in its name (`ddd/file-suffix`). The roles are command, query, handler, event, aggregate, entity, value object, repository, mapper, error, module, controller, adapter, schema, projection, spec and mother. A repository adapter also names its store, such as an order repository on a SQL store.

## Event sourcing

The pack ddd-es adds three rules on top of ddd:

- Each event that an aggregate raises has an apply method on that aggregate (`ddd-es/apply-handler`).
- Each event has a version field (`ddd-es/event-version`).
- A projection only reads: it uses no repository port, no aggregate and no command (`ddd-es/projection-read-only`).

## Coverage

- TypeScript: the harness engines check every rule of the packs ddd and ddd-es.
- Another language: the native tool of the project checks the catalogue ids `ddd/src-roots`, `ddd/domain-purity`, `ddd/layer-direction`, `ddd/cross-context` and `ddd/file-suffix`. `harness lint catalogue` prints them. A tool that checks only imports waives `ddd/file-suffix`, so a reviewer checks the file names by reading the code.
- The other rules of this page are the convention in every language. A reviewer checks them by reading the code.
- A reference file states each rule that its language maps in another way, such as a composition root in place of a module for each context.
