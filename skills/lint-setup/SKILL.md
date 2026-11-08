---
name: lint-setup
description: "Sets up the architecture rules of the harness catalogue on a project in any language: the native tool config, a wrapper lint plugin, one canary for each rule and the manifest lint/rules.yaml, proven by harness lint canaries. Use when harness init prints the next step for the lint-setup skill, or when the user types /harness:lint-setup (Codex: $harness:lint-setup) with a language."
argument-hint: "[language]"
---

# Architecture rules in any language

The harness keeps a catalogue of architecture rules. A native tool of the project language checks them, and a wrapper plugin maps its output to harness findings. A canary for each rule proves on each run that the tool still catches it.

Language: `$ARGUMENTS`. When it is empty, the `linter` agent finds the languages.

## Binary

- In each command below, `harness` means the full path `"${CLAUDE_PLUGIN_ROOT}/bin/harness"`. Write the full path in each command: a shell variable does not stay from one Bash call to the next.
- On Codex, `${CLAUDE_PLUGIN_ROOT}` shows unexpanded. Use `bin/harness` two folders above this `SKILL.md`.
- `harness lint catalogue --json` prints the rules. The catalogue is the source: do not copy the rules from this page into a file.

## The rules

| Rule id | In plain language |
|---|---|
| `ddd/src-roots` | Every source file and every dependency belongs to a known context or layer. The tool reports a file or a dependency that it cannot place. It never drops one. |
| `ddd/domain-purity` | The domain uses only itself, the shared domain and the allowed libraries. It never uses the application layer, the infrastructure layer or a framework. |
| `ddd/layer-direction` | The application layer uses the domain layer, never the infrastructure layer. |
| `ddd/cross-context` | A bounded context uses another context only through its public entry point or its published events. |
| `ddd/file-suffix` | The name of a file in a layer tells its role: a command, a handler, an aggregate or a repository. |

## Known tools

| Language | Tool | Rules that it can catch |
|---|---|---|
| PHP | deptrac | the layer and context rules, from its layers and its ruleset |
| Python | import-linter | the layer and context rules, from its `layers` and `forbidden` contracts |
| Go | go-arch-lint or depguard | the layer and context rules, from the allowed imports of each component |
| TypeScript | the built-in `ddd` pack | each rule. Add `ddd` to `lint.packs` and stop: the binary tests prove this pack, so it needs no canary |

For a language that is not in the table, the agent searches for a tool and proposes it. The user chooses.

A tool that checks only imports cannot catch `ddd/file-suffix`. Then the rule is a waiver question, never a silent gap.

## Steps

1. Start the `linter` agent to survey the project. Give it the language, the repository root, the full path of `harness` and the path of this `SKILL.md`. It returns the languages, the folders or namespaces of the contexts and the layers, the proposed tool and its open questions.
2. Ask the open questions with the question tool of the host: `AskUserQuestion` on Claude Code (4 questions in one call, 2 to 4 options), `request_user_input` on Codex (3 questions, 2 to 3 options). Put the recommended option first, with the suffix ` (Recommended)`. A host with no question tool gets a numbered list in a plain message. Typical questions: the tool, the context folders, the allowed libraries of the domain.
3. Start the `linter` agent again to write the setup, with the answers. It writes:
   - the native tool config. The user owns this file and can tune it after the setup.
   - the wrapper plugin `.harness/plugins/<tool>.sh` on `lint.check`, and its `plugins` entry in `.harness.yaml`.
   - the custom pack `lint/architecture/pack.yaml` with the catalogue ids and the wrapper as engine, and its entry in `lint.packs`.
   - one canary folder for each rule under `lint/canaries/`, and the manifest `lint/rules.yaml`.
4. The agent runs `harness lint init`, `harness lint canaries` and `harness lint`. `harness lint` must give no finding in `lint/canaries/`.
5. A rule that the tool cannot catch comes back as a waiver question. Ask it. On a yes, the agent writes `waived: <reason>` for the rule. On a no, the agent tunes the tool config and runs the check again.
6. Run `harness gates init` to add the `lint-canaries` gate. When `.harness.yaml` has a `gates` key, add `{id: lint-canaries, run: harness lint canaries}` to it by hand.
7. Show the last output of `harness lint canaries` to the user.

## Files

The plugin entry. The exec plugin runs in the envelope `cwd`, which is the canary folder under `harness lint canaries`. So the command finds the script from the repository root:

```yaml
plugins:
  - name: deptrac
    command: [sh, -c, 'exec sh "$(git rev-parse --show-toplevel)/.harness/plugins/deptrac.sh"']
    events: [lint.check]
    timeout: 60s
lint:
  packs: [./lint/architecture]
```

The custom pack holds one rule for each catalogue id with a canary, with the wrapper as engine. A waived rule needs no pack rule:

```yaml
name: architecture
rules:
  - id: ddd/layer-direction
    engine: deptrac
    severity: error
    scope: "src/**/*.php"
    message: The application layer uses the domain layer, never the infrastructure layer.
```

The manifest gives each rule of the catalogue pack a canary or a waiver:

```yaml
pack: ddd
rules:
  - id: ddd/layer-direction
    canary: lint/canaries/ddd-layer-direction
  - id: ddd/file-suffix
    waived: deptrac checks the dependencies, not the file names.
```

A canary folder mirrors the source layout, for example `lint/canaries/ddd-layer-direction/src/Order/Application/PlaceOrder.php`. It breaks its one rule on purpose and breaks no other rule. The tool config and the pack scopes must not include `lint/canaries/`.

## Wrapper contract

- Read the envelope from stdin. Analyse the files of `payload.files` under the envelope `cwd`. Under `harness lint canaries`, `cwd` is the canary folder: point the tool at `cwd`, not at the repository `src/`.
- Give each finding a `rule` from `payload.rules` and a `file` relative to `cwd`. Keep only findings whose file is in `payload.files`.
- Print one verdict: `{"verdict":"allow","findings":[...]}`. A non-zero exit is a plugin failure.
- Set a `timeout` above the 5 second default.

## Limits

- A repository has one manifest for one catalogue pack. When `lint/rules.yaml` exists for another language, stop and tell the user.
- A custom pack cannot use the catalogue ids while the built-in `ddd` pack is in `lint.packs`. When both are needed, stop and tell the user.
