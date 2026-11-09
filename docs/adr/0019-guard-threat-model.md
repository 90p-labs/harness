# 0019 Guard threat model

## Status

Accepted, 2026-11-10.

## Context

The guard rule `shell-write-source` denies a shell command that writes a source file. It now expands the variables and follows the `cd` of the same command, so a write to a scratch file passes.
In each of five review rounds, the reviewers found a new shell construct that a hand-written parser misread. The constructs were process substitution, `case` arms, `coproc`, zsh options and names built from values.
A full shell parser is a large dependency, and the guard still cannot see a name that the shell builds at run time.

## Decision

- The guard stops an honest agent that writes a source file through the shell by mistake. It is not a sandbox against a hostile agent.
- The new resolution applies only to a flat line: simple commands joined by `;`, `&&`, `||`, `|`, `&` or a newline.
- Any other construct makes the line not flat. The line then keeps the verdict of the rule before this change, and a deny asks for a literal absolute path.
- A construct that only a hostile agent writes falls back to that verdict. A review finding on such a construct is not a blocker.
- A flat line that moves from deny to allow and writes a source file is a blocker.
- The guard adds no shell parser dependency.

## Consequences

- Loops, groups, command substitution and option commands keep the old verdict, so some scratch writes still deny. The deny names the step: a literal absolute path.
- A name that the shell builds at run time, such as `$(...)`, an environment value or `OPTARG`, is a known limit.
- A hook must deny or allow. A panic in the guard is a defect.
