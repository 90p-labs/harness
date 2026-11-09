# Delivery edge

**The edge is small: it reads the input, builds a message, sends it through the module and maps the result. Presentation holds no backend code.**

## The edge

- The edge is the controllers, the CLI commands, the consumers and the jobs. All of them are in infrastructure.
- Each controller, CLI command, consumer and job does five steps. It reads the session, checks the raw input shape, builds the message, sends it through the module or the bus, and maps the result. The module runs the handler.
- The order of authentication and validation is in `use-cases.md`.
- The edge checks only the shape of the raw input, with a schema. The handler builds the value objects.
- A controller holds no business logic. A business decision lives in the domain or in a handler.
- The edge or the presentation makes a view model from the returned domain object with a pure function. The domain holds no view model. A query handler returns domain objects, and the edge makes the view.
- The edge reads a typed field of the kernel. See `composition-root.md`.

## Error mapping

- This file owns the error mapping. The edge keeps one error map that converts each domain error to a transport status or to an exit code.
- A handler lets a domain error propagate. It never catches the error to build a status.
- An error that the error map does not know maps to the generic failure status, and the edge logs it.
- The same error map serves every surface of the context, so a developer changes a rule in one place.
- The shape of the error body is in `api-contracts.md`.

## Consumers and jobs

- A consumer acknowledges a message after the handler succeeds. An early acknowledgment loses the message when the handler fails.
- A consumer may see the same message twice, so the handler is idempotent. See `events.md`.
- A job builds a message and sends it through the module, as a controller does. It holds no logic of its own.

## Presentation

- Presentation has no backend code: no kernel, no environment, no secret.
- Presentation does not compute a business value. A component shows a value that it receives.
- One pure function decides each concept, and every surface calls it: the API, the page, the CLI and the report. Two copies of a rule end up different.
- Presentation never declares again a constant of the domain, such as a status or a limit. It imports the constant or receives it.
- A date and a time zone reach the code as a resolved value. The edge resolves the zone of the user once, and the code below it does not guess.

## Review

1. Open each controller. Check that it has no branch on a business rule.
2. Search the presentation code for an import of the kernel.
3. Search for two functions that decide the same concept.
4. Check that each consumer acknowledges after the handler returns.
