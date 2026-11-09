---
name: lint-setup
description: "Sets up the architecture rules of the harness catalogue on a project in any language: the native tool config, a wrapper lint plugin, and a temporary proof folder with one canary for each rule, run by harness lint canaries --proof and then deleted. Use when harness init prints the next step for the lint-setup skill, or when the user types /harness:lint-setup (Codex: $harness:lint-setup) with a language."
argument-hint: "[language]"
---

# Architecture rules in any language

The harness keeps a catalogue of architecture rules. A native tool of the project language checks them, and a wrapper plugin maps its output to harness findings. At setup, a canary for each rule proves that the tool catches it. The canaries live in the temporary folder `lint/proof/`, and the skill deletes it after the proof. A project commits no canary.

Language: `$ARGUMENTS`. When it is empty, the `linter` agent finds the languages.

## Decision tree

```
Start: run harness version ("Binary")
├── the layout of the contexts and the layers, or the folder names of a language
│   └── load the `ddd` skill, and give its path to the linter agent with the step 1 inputs
├── TypeScript only → "Known tools": add the ddd pack to lint.packs, and stop
├── a language in "Known tools", or a language with no known tool → "Steps"
├── a canary fails → "Steps", step 6
├── a rule needs a waiver → "Steps", step 5
└── the wrapper plugin or a file format → "Files" and "Wrapper contract"
```

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
| PHP | deptrac | the layer and context rules, from its layers and its ruleset. `--report-uncovered` gives the unplaced classes and dependencies |
| Python | import-linter | the layer and context rules. Exhaustive `layers` contracts give `ddd/src-roots`, an `independence` contract gives `ddd/cross-context`, and a custom contract type with an allow list gives `ddd/domain-purity`, because a `forbidden` contract cannot hold an allow list |
| Go | go-arch-lint or depguard | the layer and context rules, from the allowed imports of each component. go-arch-lint reports an unplaced file |
| TypeScript | the built-in `ddd` pack | each rule. Add `ddd` to `lint.packs` and stop: the binary tests prove this pack, so the canary is optional. With no `rules.yaml` in the proof folder, `harness lint canaries --proof lint/proof` prints one line and passes. Next to another language, the proof `rules.yaml` gives the engine `harness` a TypeScript canary or a waiver for each rule |

For a language that is not in the table, the agent searches for a tool and proposes it. The user chooses.

A tool that checks only imports cannot catch `ddd/file-suffix`. Then the rule is a waiver question, never a silent gap.

## Steps

1. Start the `linter` agent to survey the project. Give it the language, the repository root, the full path of `harness` and the path of this `SKILL.md`. It returns the languages, the folders or namespaces of the contexts and the layers, the proposed tool and its open questions.
2. Ask the open questions with the question tool of the host: `AskUserQuestion` on Claude Code (4 questions in one call, 2 to 4 options), `request_user_input` on Codex (3 questions, 2 to 3 options). Put the recommended option first, with the suffix ` (Recommended)`. A host with no question tool gets a numbered list in a plain message. Typical questions: the tool, the context folders, the allowed libraries of the domain.
3. Start the `linter` agent again to write the setup, with the answers. It writes:
   - the native tool config. The user owns this file and can tune it after the setup.
   - the wrapper plugin `.harness/plugins/<tool>.sh` on `lint.check`, and its `plugins` entry in `.harness.yaml`.
   - the custom pack `lint/architecture/pack.yaml` with the catalogue ids and the wrapper as engine, and its entry in `lint.packs`.
   - the proof folder `lint/proof/`: the `rules.yaml` and one canary folder for each rule. The folder is temporary and untracked.

   For a second language, the agent keeps the first setup. It adds the new pack next to the first, in its own folder such as `lint/architecture-go`. The first proof is gone, so it writes the proof again for every active rule: a canary or a waiver for each engine of the rule. A rule that two packs hold gets an entry for each engine under `engines`.
4. The agent runs `harness lint init`, `harness lint canaries --proof lint/proof` and `harness lint`. `harness lint` must give no finding in `lint/proof/`.
5. A rule that the tool cannot catch comes back as a waiver question. Ask it. On a yes, the agent writes `waived: <reason>` for the rule in the proof `rules.yaml`, with a reason of 3 words or more, and keeps the rule out of the custom pack. On a no, the agent tunes the tool config and runs the check again. A proof that waives each rule fails: at least one rule needs a canary.
6. On a `FAIL` line, stop. Keep `lint/proof/` so the user can read it, and report the `FAIL` lines. Do not delete the folder.
7. On success, delete `lint/proof/`. Run `git status --porcelain` and check that it names no file under `lint/proof/`. Report each `PASS` line and each `WAIVED` line with its reason to the user, so a waiver is not silent.

The proof is a setup step. A later edit of the tool config that breaks a rule stays green until someone runs the skill again.

## Files

The plugin entry. The exec plugin runs in the envelope `cwd`, which is the canary folder under `harness lint canaries --proof`. So the command finds the script from the repository root:

```yaml
plugins:
  - name: deptrac
    command: [sh, -c, 'exec sh "$(git rev-parse --show-toplevel)/.harness/plugins/deptrac.sh"']
    events: [lint.check]
    timeout: 60s
lint:
  packs: [./lint/architecture]
```

The custom pack holds one rule for each catalogue id with a canary, with the wrapper as engine. A waived rule id has no rule in the custom pack:

```yaml
name: architecture
rules:
  - id: ddd/layer-direction
    engine: deptrac
    severity: error
    scope: "src/**/*.php"
    message: The application layer uses the domain layer, never the infrastructure layer.
```

The proof `lint/proof/rules.yaml` gives each rule of the catalogue pack a canary or a waiver. A canary path is relative to the proof folder:

```yaml
pack: ddd
rules:
  - id: ddd/layer-direction
    canary: ddd-layer-direction
  - id: ddd/file-suffix
    waived: deptrac checks the dependencies, not the file names.
```

When two active packs hold a rule on two engines, for example the built-in `ddd` pack (engine `harness`) and a deptrac pack, the rule gives one canary or one waiver for each engine under `engines`. A rule on one engine keeps the single form. `harness lint canaries --proof` names the engine on each line of an `engines` entry:

```yaml
pack: ddd
rules:
  - id: ddd/domain-purity
    engines:
      harness:
        canary: ts-domain-purity
      deptrac:
        canary: ddd-domain-purity
  - id: ddd/file-suffix
    engines:
      harness:
        waived: the binary fixtures prove the built-in rule.
```

A canary folder mirrors the source layout, for example `lint/proof/ddd-layer-direction/src/Order/Application/PlaceOrder.php`. It breaks its one rule on purpose and breaks no other rule. The tool config and the pack scopes must not include `lint/proof/`. No `.gitignore` rule may match `lint/proof/`: the file list drops an ignored file, so its canary would fail.

The command refuses a proof folder outside the work tree and a canary path that leaves the proof folder.

## Wrapper contract

- Read the envelope from stdin. Analyse the files of `payload.files` under the envelope `cwd`. Under `harness lint canaries --proof`, `cwd` is the canary folder: point the tool at `cwd`, not at the repository `src/`.
- Give each finding a `rule` from `payload.rules` and a `file` relative to `cwd`. Keep only findings whose file is in `payload.files`. A canary passes only on a finding for a file inside the canary folder.
- Map each file that the tool cannot place to `ddd/src-roots`, and each dependency that it cannot place to the rule of the layer that holds the dependant file. Never drop an unplaced file or dependency.
- End each pack rule scope with the file extension of the language, such as `src/**/*.php`. `harness init` uses the extension to find the languages that the pack covers.
- Print one verdict: `{"verdict":"allow","findings":[...]}`. A non-zero exit is a plugin failure.
- Set a `timeout` above the 5 second default. On a timeout, the harness stops the process group of the plugin. A container keeps running after its client stops, so a wrapper that starts one names it and kills it on SIGTERM:

```sh
name="shop-lint-$$"
trap 'docker kill "$name" >/dev/null 2>&1; exit 143' TERM
docker compose run --rm -T --name "$name" php vendor/bin/deptrac analyse --formatter=json &
wait $!
```

## Limits

- Two packs can hold one catalogue id only on two different engines. A pack on the same engine as another pack with that id is an error: then extend the first pack, or stop and tell the user.
- The scopes of two packs that share an id must not hold one file. For the engine `harness`, only TypeScript and JavaScript files count. A file in both scopes is an error that names both packs and the file: end each scope with the extension of the language.
- A canary proves the rule on the canary folder only. A source folder that a pack scope misses still passes the canary.
