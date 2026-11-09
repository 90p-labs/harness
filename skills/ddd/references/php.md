# DDD layout in PHP

The native tool is deptrac. Through the wrapper plugin of the project, the harness checks the catalogue ids `ddd/src-roots`, `ddd/domain-purity`, `ddd/layer-direction` and `ddd/cross-context`. deptrac checks the dependencies, not the file names, so `ddd/file-suffix` gets a waiver in the proof of the lint setup. The other rules of the ddd skill are the convention.

## Tree

One namespace for each context, and one namespace for each layer in it:

```
src/
├── Kernel.php
├── Shared/
│   └── Domain/AggregateRoot.php
└── Order/
    ├── Domain/
    │   ├── Order.php
    │   ├── OrderId.php
    │   ├── OrderPlaced.php
    │   └── OrderRepository.php
    ├── Application/
    │   ├── Command/PlaceOrder/
    │   │   ├── PlaceOrderCommand.php
    │   │   └── PlaceOrderHandler.php
    │   ├── Query/GetOrder/
    │   └── EventHandler/
    └── Infrastructure/
        ├── DoctrineOrderRepository.php
        ├── OrderMapper.php
        └── OrderController.php
```

- The class name ends in its role: `PlaceOrderCommand`, `PlaceOrderHandler`, `OrderPlaced` for an event, `OrderRepository` for the port and `DoctrineOrderRepository` for its adapter.
- The aggregate, the entity and the value object take the plain domain name, such as `Order` and `OrderId`.
- A context has no module class. The service config `config/services.yaml` wires each context, so `ddd/module-per-context` and `ddd/one-kernel` map to it. The `Kernel` uses only the framework.

## deptrac

Start from the tested config `examples/symfony/deptrac.yaml` of the harness plugin folder. Its README gives the wrapper steps. Do not write a config from memory: a missing library layer turns each framework dependency into a `ddd/src-roots` finding.

| Rule | deptrac in the example |
|---|---|
| `ddd/src-roots` | `deptrac analyse --report-uncovered` and `deptrac debug:unassigned` give each class and dependency that no layer holds |
| `ddd/domain-purity` | each `Domain` layer depends only on `SharedDomain` and `DomainLibraries` |
| `ddd/layer-direction` | each `Application` layer does not list its `Infrastructure` layer |
| `ddd/cross-context` | no layer of a context lists a layer of another context |
| `ddd/file-suffix` | not checked: a waiver |

The library layers are `DomainLibraries` (the allow list of the domain), `Framework` (Symfony and Doctrine, for the kernel and the infrastructure) and `Vendor` (the other packages, for the application and the infrastructure).

## Patterns

| Topic | In PHP |
|---|---|
| `use-cases.md` | `PlaceOrderCommand` and `PlaceOrderHandler` share one folder. A validator component checks the raw input in the controller. The message holds primitives, and the handler builds the value objects through their factories. |
| `composition-root.md` | `config/services.yaml` wires each context, and the `Kernel` wires the whole. A framework container is an option: a class still gets its dependencies through its constructor. |
| `events.md` | Event sourcing is an option: an aggregate records events and an event store keeps them. A message bus of the framework can carry the events. An integration event interface lives in `App\Shared\Domain`, which every layer may use. |
| `tests.md` | A test runner runs the `OrderTest` classes. An object mother is an `OrderMother` class. The architecture test is deptrac. The `lint-setup` skill proves its rules once with canaries. |
