# API contracts

**A contract that another system reads comes from the same schemas that check the input and the output. Nobody edits it by hand.**

## Generation

- An API that another system calls has a contract. OpenAPI is the usual format for a synchronous API.
- The build generates the contract from the code. A stored copy differs from the routes after a change.
- The schemas that validate the input and the output are the source. The contract and the checks cannot disagree.
- Never edit the contract by hand. Change the schema, then generate the contract again.

## Ownership

- Each context owns its API operations and its response schemas. They are in the infrastructure layer of that context.
- A context does not declare an operation for another context.
- The API operations map to messages. The controller of a route sends the message through the module or the bus. See `delivery-edge.md`.

## Error envelope

- The API uses one shared error envelope for every failure.
- An unknown path answers with the error envelope, not with a default page.
- The error map, which converts a domain error to a status, is in `delivery-edge.md`. The envelope carries the result of the error map.

## Tests

- A test pairs each route with an operation. It fails on a route that has no operation.
- The same test fails on an operation that has no route.
- A recorded response of the legacy system validates against the response schema. A mismatch shows a change of the wire shape. See `porting-legacy.md`.
- A test builds the contract from the code and checks that it is valid.

## Change

- A breaking change needs a new version. Removing a field, renaming a field or narrowing a type is a breaking change.
- An addition that a caller may ignore is not a breaking change.
- Keep the old version until each caller moves.

## Async contracts

- The same rules apply to an async contract of integration events. AsyncAPI is the usual format.
- The schema of an integration event generates the contract. The event carries a version. See `events.md`.
- A change of an integration event follows the rule of a breaking change. See `contexts.md` for the events between contexts.
- The consumer at the edge checks the shape of the event with the schema of the shared interface. Then it publishes the event on the event bus, and the bus runs the event handler.
