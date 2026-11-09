# DDD layout in Python

The native tool is import-linter. Through the wrapper plugin of the project, the harness checks the catalogue ids `ddd/src-roots`, `ddd/domain-purity`, `ddd/layer-direction` and `ddd/cross-context`. import-linter checks the imports, not the module names, so `ddd/file-suffix` gets a waiver in the proof of the lint setup. The other rules of the ddd skill are the convention.

## Tree

One package for each context, and one subpackage for each layer in it:

```
src/
├── main.py
└── shop/
    ├── shared/
    │   └── domain/money.py
    ├── billing/
    └── order/
        ├── __init__.py
        ├── domain/
        │   ├── order_aggregate.py
        │   ├── order_id_value_object.py
        │   ├── order_placed_event.py
        │   └── order_repository.py
        ├── application/
        │   ├── commands/place_order/
        │   │   ├── place_order_command.py
        │   │   └── place_order_handler.py
        │   ├── queries/get_order/
        │   └── event_handlers/
        └── infrastructure/
            ├── order_sql_repository.py
            ├── order_mapper.py
            └── order_controller.py
```

- Each package and each subpackage has an `__init__.py`. The tree shows only one.
- The `__init__.py` of a context is its public entry point, so `ddd/module-per-context` maps to it.
- The composition root `main.py` is outside the business package `shop`, so `ddd/one-kernel` maps to it. A module directly in `shop`, such as `src/shop/cart.py`, is a `ddd/src-roots` finding.
- A module name ends in its role after an underscore, such as `place_order_handler.py`. A dot cannot separate the role, because a dot splits a Python import path.

## import-linter

Start from the tested config `examples/python/.importlinter` of the harness plugin folder, and its custom contract `examples/python/.harness/plugins/domain_purity.py`. Its README gives the wrapper steps. Do not write a config from memory: an `exhaustive` layers contract needs `containers`, and the wrapper maps a broken contract to the rule id of its name.

| Rule | import-linter in the example |
|---|---|
| `ddd/src-roots` | exhaustive `layers` contracts on `shop` and on each context report a package that no layer names |
| `ddd/domain-purity` | the custom contract type `domain_purity` with an allow list, because a `forbidden` contract cannot hold an allow list |
| `ddd/layer-direction` | a `layers` contract on each context: the application layer never imports the infrastructure layer |
| `ddd/cross-context` | an `independence` contract on the contexts, and a `layers` contract that keeps `shared` below the contexts |
| `ddd/file-suffix` | not checked: a waiver |
