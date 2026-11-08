# harness

The harness is an open source Go tool that drives an AI coding agent through a fixed loop: plan, work, review, ship and watch production. Plugins can veto or extend each step. It works with Claude Code and Codex.

## Status

Pre-alpha. The design is stable. The code is a skeleton: the run state machine, the plugin event bus and the telemetry setup.

## Build and check

Build the binary:

```sh
make build
```

Run the tests, the lint and the layer check:

```sh
make check
```

## Documents

- `DESIGN.md` describes the modules, the loop and the plugin events.
- `CONTEXT.md` defines the terms.

## License

MIT. See `LICENSE`.
