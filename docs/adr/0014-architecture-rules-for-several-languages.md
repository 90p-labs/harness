# 0014 Architecture rules for several languages

## Status

Accepted, 2026-11-08.

## Context

ADR 0013 lets a project prove the catalogue rules in its own language. A repository had one manifest, and a catalogue rule id belonged to one active pack. A TypeScript and PHP repository needs the built-in `ddd` pack and a deptrac pack with the same ids, so the packs clashed. Once `lint/rules.yaml` existed, `harness init` named the one-manifest limit, and a second language never got its rules. The built-in pack also gave a `ddd/src-roots` finding for a PHP root file such as `src/Kernel.php`.

## Decision

- A custom pack can hold a catalogue id that another active pack holds, when the engines differ. A same-engine clash, or a clash on an id outside the catalogue, stays an error.
- The scopes of two packs that share an id must not hold one file. The check lists the git files of the repository, only when two packs share an id. For the engine `harness`, only TypeScript and JavaScript files count, because the built-in engine reads only these. A file in both scopes is an error that names both packs, both engines, the rule and the file.
- The repository keeps one manifest. A rule on two engines gives one canary or one waiver for each engine under `engines`. A rule on one engine keeps the single form, so the existing manifests stay valid.
- `harness lint canaries` proves each (rule, engine) pair. A line from an `engines` entry names the engine. A single-form line stays the same as before.
- Each engine that holds a rule needs an entry, also `harness`. A waiver such as "the binary fixtures prove the built-in rule" fits a rule with no TypeScript canary. A single-form entry for a rule on two engines is an error that names both engines, because the harness cannot know which engine it is for.
- `harness init` prints the `lint-setup` step for each language that no plugin rule covers, also when `lint/rules.yaml` exists.
- The built-in `ddd/src-roots` check skips a root file in `src/` that is not TypeScript or JavaScript, as it skips a folder with no script.
- The ddd pack scopes, such as `src/**` and `src/**/domain/**`, get no extension. A folder finding such as `src/Order` has no extension, so a scope with an extension would drop it.

## Consequences

- A TypeScript and PHP repository runs the built-in `ddd` pack and a deptrac pack together, and the canaries prove each language.
- With a second language, the built-in pack needs a manifest entry for each catalogue rule. Alone, it still needs no manifest.
- A pack scope with no extension, such as `src/**`, fails at load next to the built-in pack as soon as a TypeScript file is in its scope. The skill tells the agent to end each scope with the extension of its language.
- The folder checks of the built-in pack read the whole tree, not the scope. A PHP folder layout that matches the TypeScript context layout could still give a finding.
- Every `harness` command that loads two packs with a shared id pays one `git ls-files` call for each pack that shares an id.
