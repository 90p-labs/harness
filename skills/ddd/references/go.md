# DDD layout in Go

The native tool is go-arch-lint. Through the wrapper plugin of the project, the harness checks the catalogue ids `ddd/src-roots`, `ddd/domain-purity`, `ddd/layer-direction` and `ddd/cross-context`. go-arch-lint checks the imports, not the file names, so `ddd/file-suffix` gets a waiver in the proof of the lint setup. The other rules of the ddd skill are the convention.

## Tree

One package tree for each context under `internal/`, and one package for each layer:

```
cmd/
└── shop/main.go
internal/
├── billing/
│   ├── domain/
│   ├── application/
│   └── infrastructure/
└── order/
    ├── domain/
    │   ├── order.go
    │   ├── order_id.go
    │   ├── events.go
    │   └── repository.go
    ├── application/
    │   ├── place_order/
    │   │   └── handler.go
    │   └── get_order/
    │       └── handler.go
    └── infrastructure/
        ├── sql_repository.go
        └── mapper.go
```

- The composition root is in `cmd/`, outside `internal/`. A file directly in `internal/order/` is a `ddd/src-roots` finding.
- A context has no module file, so `ddd/module-per-context` and `ddd/one-kernel` map to the composition root in `cmd/`, which wires each context.
- A use case is one package in `application/`, such as `place_order`, with its message and its handler. This package maps `ddd/cqrs-folders` and `ddd/handler-pair`. No `.go` file sits directly in `application/`.
- go-arch-lint fails with `not found directories` when a component folder is missing. Give each context of the config its folders, or edit the components to your contexts.
- A Go package name is one lower case word with no underscore: the folder `place_order` holds `package placeorder`. The package path tells the layer and the use case, and a type name tells the rest: `PlaceOrder` and `Handler` in `application/place_order`, `OrderRepository` in `domain`, `SQLOrderRepository` in `infrastructure`.
- A domain event type is a fact in the past tense, such as `OrderPlaced`.

## go-arch-lint

Start from the tested config `examples/go/.go-arch-lint.yml` of the harness plugin folder. Its README gives the wrapper steps. Do not write a config from memory: with `depOnAnyVendor: false`, a component with no vendor entry can use only the standard library.

| Rule | go-arch-lint in the example |
|---|---|
| `ddd/src-roots` | a file under `internal/` that no component covers |
| `ddd/domain-purity` | the domain component has no `deps` entry, so it uses no other component and no third-party library |
| `ddd/layer-direction` | the application component does not list the infrastructure component |
| `ddd/cross-context` | no component lists a component of another context |
| `ddd/file-suffix` | not checked: a waiver |

depguard in golangci-lint can check the layer rules too, but it does not report an unplaced file, so `ddd/src-roots` needs a second check.
