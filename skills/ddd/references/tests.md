# Tests

**A test runs one unit of work through one public entry point and checks the result that a caller sees.**

## Shape of a test

- A test checks behavior, never structure. It does not read a private field or count a call.
- One test covers one unit of work: from one public entry point to one result that a caller sees.
- Replace only the outside dependencies with in-memory adapters, such as the store, the clock, the id generator and the network. The test runs entities and value objects for real.
- A mock that does the asserted work proves nothing. Use an in-memory adapter that holds real state.
- The expected value is a literal in the test. The code under test never computes it.
- A match is exact. A loose match, such as "contains" or "any", lets a wrong value pass.
- A negative case must fail for the reason that its name states. Assert the domain error by its type.
- A regression test fails on the code before the fix. Run it on the old code once to prove it.

## Given, when, then

- This file owns the form for event sourcing: given past events, when a command, then the new events or the domain error.
- Given: the test loads the past events of the aggregate in the in-memory event store adapter.
- When: the test sends one command to the handler.
- Then: the test reads the events that the store holds after the command, and compares them with the expected events.
- A rejected command has no new events. The test asserts the domain error and checks that the store is unchanged.
- The same form works for a state based aggregate: given a stored aggregate, when a command, then the stored state or the error.

## Tests by layer

- Domain: unit tests with no in-memory adapter. They call the factories and the methods of the aggregates.
- Application: handler tests over in-memory adapters. Each path of a handler has a test.
- Infrastructure: adapter integration tests against a real store. They prove the queries, the transactions and the schema.
- Edge: one test end to end for each controller, CLI command, consumer and job. Each one crosses the edge, the handler and the store once.
- A mapper test checks that the select list equals the row keys. A key in one and not in the other is a defect. See `repositories-and-mappers.md`.
- A projection test feeds events to the projection and reads the read model. The test checks the read model, not the internals of the projection.

## Object mothers

- A mother is a builder of a valid domain object. Its file has the suffix mother (`ddd/file-suffix`).
- A test starts from the valid object and changes one thing. The change is the point of the test.
- A mother builds through the same factories as the production code. It never skips an invariant.

## Architecture tests

- The lint rules of the packs are the architecture tests: `ddd/domain-purity`, `ddd/layer-direction` and `ddd/cross-context`.
- Prove each rule with a canary: a file that breaks the rule on purpose. The lint run must fail on it.
- A rule with no failing canary may check nothing.
- The test tree need not mirror the source tree. Group tests by behavior.

## Coverage

- Coverage finds gaps. It is not a score.
- A line that no test runs is a question: is the behavior needed? Test it or delete it.
- Never write a test only to raise a number.
