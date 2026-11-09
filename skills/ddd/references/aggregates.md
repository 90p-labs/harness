# Aggregates

**An aggregate is a consistency boundary. Only its root changes it, and one save stores all of it or nothing.**

## Boundary

- An aggregate holds the objects that must stay consistent together. It has one root.
- Outside code holds a reference to the root only. It never holds a reference to a child entity.
- One repository serves each root. A child entity has no repository.
- One transaction covers one save. The store never keeps a half-written aggregate.
- One command changes one aggregate. Use events to change a second aggregate, with eventual consistency.
- Keep the aggregate small. A big aggregate causes write conflicts and slow loads.
- Reference another aggregate by its id, never by an object. The id keeps the boundaries clear.

## Invariants

- The factory of the root checks the invariants. The constructor is private.
- A rebuild factory rebuilds an aggregate from storage. It records no event, and only the mapper or the event store calls it.
- An object that exists is valid. No code builds an invalid aggregate and then fixes it.
- Each behavior method checks its own rules before it changes the state.
- A rule that fails raises a domain error. See `use-cases.md` for the path to the edge.
- The aggregate exposes no public writable state (`ddd/aggregate-encapsulation`). The reason: only the root can protect the rules.
- Expose behavior, not getters and setters for every field.

## Changes

- A change returns a new value, or records an event. It never leaves the object partly changed.
- The root records an event for each business fact. The handler publishes it after the save. See `events.md`.
- Copy a collection on the way in. A caller that keeps the original list must not change the state of the aggregate.
- Copy a collection on the way out, or return a read-only view.

## Value objects

- A value object is immutable. A change builds a new value object.
- A value object has an `equals` method (`ddd/value-object-equals`).
- Two value objects are equal when their values are equal. They are not equal by class name or by identity.
- A value object has a factory that checks the value and rejects a bad one (`ddd/value-object-factory`).
- The constructor is private, so each instance went through the factory.
- A value object holds its own behavior, such as a comparison or a sum.
- Use a value object in place of a bare string or number for each domain concept.

## Rich model

- The model is rich, not anemic. The behavior lives in the aggregate and the value objects.
- A handler that computes a business value moves the rule out of the domain. Move the computation into the aggregate.
- A domain operation holds a rule that spans two or more aggregates and fits none of them. Use one only when no single aggregate can hold the rule.
- Test the aggregate without a store. The domain layer has no infrastructure (`ddd/domain-purity`).
