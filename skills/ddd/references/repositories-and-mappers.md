# Repositories and Mappers

**A repository is a port in the domain. Its adapter in infrastructure gets every dependency through its constructor.**

## Repositories

- The port of a repository is in the domain (`ddd/repository-port`). It speaks in domain objects and ids.
- The adapter is in infrastructure. Its name tells the store, such as "SQL order repository" or "in-memory order repository".
- One repository serves one aggregate root. See `aggregates.md`.
- The port has a method for each question the domain asks. It does not expose a generic query builder.
- A "not found" case is a domain error or an explicit absent value. Never return an object with missing fields.
- An in-memory adapter implements the same port. Tests use it. See `tests.md`.

## Constructor injection

- A class gets its dependencies through its constructor. The kernel opens each connection once.
- No code looks up a connection, an adapter or a port at call time. That pattern is a service locator.
- The reason: the reader cannot see, replace or test a hidden dependency.
- A DI container is allowed as a tool. The rule is the same: only the module wires the context. Each context has one module (`ddd/module-per-context`).
- See `composition-root.md` for the module and the kernel.

## Mappers

- A mapper converts a stored row to a domain object and back, through the rebuild factory of `aggregates.md`. It lives in infrastructure (`ddd/mapper-location`).
- The domain knows no row, no column and no store format.
- The mapper builds value objects through their factories. A stored value outside the known set raises a domain error.
- The mapper owns the select list: the list of fields that each query reads.
- A test checks that the select list equals the keys of the row. A new column then fails a test, not a user.
- No double cast. The mapper accepts a typed row, so the compiler checks the shape.
- The read types tell what the store holds. A column that may be empty has an optional type.
- Never let a raw row leave the adapter.

## Read side

- A read side may read a read model and skip the aggregate. The aggregate exists for writes.
- A reader adapter has its own port and its own mapper. It returns the read model as is.
- The read model is built for the reader, with the fields the screen or the API needs.
- A projection fills the read model from events. See `events.md`.
