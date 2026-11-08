# Pack ddd-es

This pack adds event sourcing rules. It needs the pack `ddd`.

- Each event has a version.
- Each raised event has an apply method in its aggregate.
- A projection does not import a repository port, an aggregate or a command.

Use an outbox or a failover store between the events and the bus. This way, an event is not lost when the bus is down. No rule checks it.
