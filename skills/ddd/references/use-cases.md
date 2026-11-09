# Use Cases

**A use case is one message and one handler in one folder, and it does one action.**

## Messages

- A message is a command or a query. It holds primitives only, so it can pass through a bus or a queue.
- A command changes state. A query reads state. Never mix both in one message.
- A use case does one action. Never add a mode flag to select between two actions. Write two use cases.
- The caller makes the id before the command. The command carries the id, so the caller knows it without a return value.
- The folder of a use case holds the message and the handler together (`ddd/handler-pair`).
- The application layer holds only the folders commands, queries and event handlers (`ddd/cqrs-folders`).

## Validation

- The edge checks the shape of the raw input: types, required fields and formats. It rejects bad input before the handler runs.
- The handler builds the value objects through their factories. A factory rejects a bad value with a domain error.
- A domain error propagates up to the edge. The edge maps it to a status. See `delivery-edge.md` for the mapping.
- Never return a boolean in place of the reason of a failure. A handler raises a domain error that names the reason.
- Authenticate before you validate. An unknown caller must learn nothing about the rules of the domain.
- A use case owns the permissions it needs. The caller never passes them. The handler checks the permission itself.

## Command handlers

- A command handler returns nothing (`ddd/command-handler-returns`). The reason: a returned value couples the caller to the write.
- The handler loads the aggregate, calls its behavior, saves it and publishes the recorded events.
- The handler publishes events after the save, never before (`ddd/publish-after-save`). See `events.md`.
- The handler holds no business rule. A handler that computes a business value moves the rule out of the domain. See `aggregates.md`.

## Query handlers

- A query handler writes nothing (`ddd/query-handler-no-write`). No save, no publish, no side effect.
- A query handler returns domain objects or read models. It never maps them to a DTO.
- The edge makes the view or the wire shape from the returned object.
- A query handler may read a read model and skip the aggregate. See `repositories-and-mappers.md`.

## Dependencies

- A handler depends on domain ports only. It never imports an adapter.
- A handler receives its ports through its constructor. See `repositories-and-mappers.md` for the rule.
- The module wires the handler. The edge sends the message through the module or the bus, and never builds a handler or reaches an adapter.

## Bus

- A bus is optional. A direct call to the handler works as well.
- A bus allows middleware. Typical middleware: a transaction around a command, logging, and a retry.
- Middleware wraps the handler. It never holds a business rule.
- With or without a bus, each message has exactly one handler.
- An event handler is a different thing. It reacts to an event and lives in the application layer. See `events.md`.
