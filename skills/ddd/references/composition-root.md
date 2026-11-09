# Composition root

**One module wires each context, and one kernel wires the modules. A class never finds its own dependencies.**

## Module

- Each context has one module (`ddd/module-per-context`). Never make one module for each aggregate, because the aggregates of a context share one language.
- The module takes the shared dependencies, after the kernel sets them up, through its constructor.
- The module builds the adapters and the handlers of its context.
- The module is the public entry point of the context. It exposes only what the edge or another module calls: the commands and the queries.
- The edge sends a message through the module or the bus, and the module runs the handler.
- The collaborators of the module stay private. The edge never builds a handler and never reaches a repository.
- A module may depend on a shared module, such as the logger, the clock or the connections.
- Another context calls this module only. See `contexts.md` for the rule on calls between contexts (`ddd/cross-context`).

## Kernel

- The business root has one kernel (`ddd/one-kernel`). A second kernel is a second source of truth for the wiring.
- The kernel boots once. It reads the config once and passes the values to the modules.
- The values reach each class through its constructor. No class reads the environment itself. See `repositories-and-mappers.md` for constructor injection.
- A connection opens once and the kernel caches it. A second call gets the same connection.
- When the boot fails, the kernel resets its state. The next boot starts clean and does not reuse a partly built kernel.
- A call from the edge never tears down the kernel. The process owns the connections until it stops.
- One mechanism sets up the schema of the store. Two mechanisms can apply different schemas to the same store.

## Tool choice

- Plain constructors and a DI container are both allowed. Only the module uses the container.
- A bus is a tool choice too. See `use-cases.md`.
- No class calls a locator or looks up a dependency by a string key. The type check and the reader cannot see a dependency behind a string key.
- A test builds a handler with its constructor and with in-memory adapters. It needs no kernel.

## Edge

- The edge reads a typed field of the kernel, or it sends a message. It never casts the value that it gets.
- A cast at the edge shows that the kernel returns the wrong type. Fix the type of the kernel.
- The edge does not build an adapter. It asks the module. See `delivery-edge.md` for the work of the edge.

## Review

1. Count the modules and the contexts. The two numbers are equal.
2. Search for a class that reads the environment. Only the kernel may read it.
3. Search for a string key that a class passes to a lookup. Replace it with a constructor argument.
4. Check that the module exports no repository.
