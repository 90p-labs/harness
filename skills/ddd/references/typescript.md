# DDD layout in TypeScript

The harness engines check every rule of the packs ddd and ddd-es on TypeScript and JavaScript files. Turn the packs on in `.harness.yaml`:

```yaml
lint:
  packs: [ddd, ddd-es]
  ddd:
    root: contexts
    src_files: [main.ts]
    domain_allow: [uuid]
```

| Setting | Default | Meaning |
|---|---|---|
| `lint.ddd.root` | `contexts` | the business root folder in `src/` |
| `lint.ddd.src_files` | `[main.ts]` | the files that can be directly in `src/` |
| `lint.ddd.domain_allow` | empty | the packages that the domain can import, by exact name or by the prefix `name/` |

## Tree

```
src/
├── main.ts
├── app/
├── components/
└── contexts/
    ├── kernel.ts
    ├── shared/
    │   ├── domain/aggregate-root.aggregate.ts
    │   ├── application/
    │   └── infrastructure/
    └── order/
        ├── order.module.ts
        ├── domain/
        │   ├── order.aggregate.ts
        │   ├── order-id.value-object.ts
        │   ├── order-placed.event.ts
        │   └── order.repository.ts
        ├── application/
        │   ├── commands/place-order/
        │   │   ├── place-order.command.ts
        │   │   └── place-order.handler.ts
        │   ├── queries/get-order/
        │   │   ├── get-order.query.ts
        │   │   └── get-order.handler.ts
        │   └── event-handlers/
        └── infrastructure/
            ├── order.typeorm-repository.ts
            ├── order.mapper.ts
            └── order.controller.ts
```

- `src/` holds only `app/`, `components/`, the business root and the files of `lint.ddd.src_files` (`ddd/src-roots`).
- A folder under the business root with a `*.module.ts` at its top level, or with a `domain`, `application` or `infrastructure` child, is a context. Contexts can nest in groups, such as `src/contexts/agency/hhrr/salaries/`.
- A group holds no file of its own. Only the business root holds `kernel.ts` (`ddd/parent-no-code`, `ddd/one-kernel`).
- The CQRS folders are `commands/`, `queries/` and `event-handlers/` (`ddd/cqrs-folders`). A `commands/NAME/` or `queries/NAME/` folder holds one `*.command.ts` or `*.query.ts` and one `*.handler.ts` (`ddd/handler-pair`).
- The module file of a context imports the module of another context. A file in `event-handlers/` imports the `*.event.ts` of another context. No other file imports another context (`ddd/cross-context`).

## File names

The suffix is the second last dot part of the name, such as `aggregate` in `order.aggregate.ts`. The engine checks it on `.ts` and `.tsx` files in a layer folder (`ddd/file-suffix`).

| Suffix | Layer |
|---|---|
| `command`, `query`, `handler` | application |
| `aggregate`, `entity`, `value-object`, `event`, `error`, `repository` | domain |
| `NAME.STORE-repository`, `mapper`, `controller`, `adapter`, `schema` | infrastructure |
| `module` | the context root |
| `projection` | the read side of ddd-es |
| `spec`, `mother` | the tests and the object mothers |

- `*.repository.ts` is in `domain/`, and `*-repository.ts`, such as `order.typeorm-repository.ts`, is in `infrastructure/` (`ddd/repository-port`).
- `*.mapper.ts` is in `infrastructure/` (`ddd/mapper-location`).
- The last word of a `*.event.ts` name ends in `ed` or is an irregular past form, such as `sent`, `paid` or `written` (`ddd/event-past-tense`).

## Code rules

| Rule | The code that fails |
|---|---|
| `ddd/command-handler-returns` | a command handler `execute` that returns a value other than `void` or `Promise<void>` |
| `ddd/query-handler-no-write` | a `save`, `delete`, `persist`, `publish` or `dispatch` call in a query handler |
| `ddd/publish-after-save` | a `publish` call before the repository `save` call in one command handler method |
| `ddd/event-readonly` | a property without `readonly` in a `*.event.ts` class |
| `ddd/aggregate-encapsulation` | a public property without `readonly` in a `*.aggregate.ts` class |
| `ddd/value-object-equals` | a `*.value-object.ts` class without an `equals` method |
| `ddd/value-object-factory` | a `*.value-object.ts` class with a public constructor |
| `ddd-es/apply-handler` | a `raise(new NAME(...))` in an aggregate with no `applyNAME` method |
| `ddd-es/event-version` | a `*.event.ts` class without a `readonly version` field |
| `ddd-es/projection-read-only` | a `*.projection.ts` import of a repository port, an aggregate or a `commands/` path |

`harness lint` runs the checks. `harness lint --json` gives the findings with their rule ids.
