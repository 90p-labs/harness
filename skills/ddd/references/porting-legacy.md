# Porting legacy code

**Write tests that record the legacy behavior first. Then port the semantics verbatim.**

## Order of work

- Write characterization tests before the first change. They record what the legacy code does today, not what it should do.
- Use the legacy examples as the cases: its fixtures, its recorded requests and its documented samples.
- A characterization test passes on the legacy code first. A test that never passed on the old code has no value.
- Port the semantics verbatim. Keep the odd cases, the order of the steps and the rounding.
- Change the behavior only in a separate step, after the port is green. One step changes the structure, the other changes the behavior.

## Contracts

- Keep the exact user-facing strings. A message, a label or a code in the legacy output is a contract.
- Keep the wire formats: field names, date formats, number formats and the order of the fields.
- Keep the frozen contracts stable while both systems run: the schema of the store and the API. See `api-contracts.md`.
- Reuse a legacy library that is already installed. Do not write a hand copy of its output, because the copy goes out of date.

## Shape of the new code

- Split a legacy manager with a mode flag into one use case for each action. See `use-cases.md`.
- Each action gets its own message, its own handler and its own tests.
- Move the checks of the legacy code into the factories of the value objects and the invariants of the aggregate.
- Name the new code with the terms of the domain, not with the names of the legacy tables.

## Move in small steps

- Move one feature at a time behind the legacy edge. This is the strangler approach.
- The legacy edge calls the new handler for the moved feature, and the old code for the rest.
- Run both paths on the same inputs and compare the results, until the results agree.
- Remove the old code of a feature only after its characterization tests pass on the new code.
- Before you move an engine with a deterministic output, record a golden master of its output. The move is safe only while the output stays the same.
- Never move two features in one step. A failure must point to one feature.
