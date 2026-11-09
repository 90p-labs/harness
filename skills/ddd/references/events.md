# Events

**An event is an immutable fact in the past tense. The aggregate records it and the handler publishes it after the save.**

## Domain events

- Name an event in the past tense, such as "order placed" (`ddd/event-past-tense`). It states a fact that happened.
- An event is immutable (`ddd/event-readonly`). Nobody changes a fact.
- The aggregate records an event for each business fact. It does not publish.
- The handler publishes the recorded events after the save (`ddd/publish-after-save`). A failed save then publishes nothing.
- An outbox is a recommendation: store the events in the same transaction as the aggregate, and a relay publishes them later. This avoids a lost event.
- When the publish fails after a successful save, the outbox delivers the event later. Without an outbox, the event is lost.
- Each event carries an id, the aggregate id, the time it occurred, and the ids of the message that caused it (correlation and causation).
- An event handler lives in the application layer. It reacts to an event of its own context or of another context.
- See `contexts.md` for events that cross contexts.

## Event sourcing

- The state of the aggregate is the sum of its events. The aggregate records an event, then applies it.
- Each event that the aggregate records has an apply method on that aggregate (`ddd-es/apply-handler`).
- An apply method only changes state. It never validates, never fails and never reads the outside.
- An event carries a schema version field (`ddd-es/event-version`). The reason: the shape changes over time, and readers must know which shape they get.
- The behavior method checks the rules first. Then it records the event.
- An event with no apply method fails fast. A silent skip lets a bug pass with no error.
- A rebuild loads the history of the aggregate and applies each event in order.
- Write the tests in the form given events, when a command, then events. See `tests.md`.

## Event store

- The event store is append-only. Nobody updates or deletes an event.
- The event store keeps one stream for each aggregate id, ordered by stream version, the position of the event in the stream.
- The store saves each event with a stable type name. A registry maps the name to the current shape, so an upcast can read an old schema version.
- A save sends the new events with the expected version, the stream version that the handler loaded. The store rejects the append when the stored stream version differs. This gives optimistic concurrency.
- A rejected append is a conflict. The handler retries with a fresh load, or reports the conflict.
- A snapshot saves the state every N events. It is optional. A load reads the last snapshot, then the later events.
- A snapshot is a cache. Delete it and the history still rebuilds the aggregate.
- Upcast old schema versions on read. A function turns an old schema version into the current shape.
- Never rewrite the store to change an old event.

## Projections

- A projection reads events and builds a read model. It is read-only (`ddd-es/projection-read-only`).
- A projection never calls a command handler and never changes an aggregate.
- Each projection keeps its stored position, the last event it handled.
- A rebuild clears the read model and replays all events from the start.
- After a stop, the projection catches up from its stored position.

## Delivery

- Delivery is at least once. An event handler can see the same event twice.
- Every event handler is idempotent. A second run of the same event changes nothing more.
- Use the event id or the stored position to detect a repeat.
- Read models are eventually consistent. A query right after a command may show the old state.
- Design the screen for this delay. Never block a command to wait for a projection.
