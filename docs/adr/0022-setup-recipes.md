# 0022 Setup recipes

## Status

Accepted, 2026-11-11. It adds the `harness recipe` command. It does not change the gates, the lint packs or the plugin events.

## Context

- A new repository needs the same files each time: `.harness.yaml`, a `compose.yaml`, a Dockerfile and the lint files. The examples of `examples/` hold those files, but a user had to copy them by hand.
- A copy from an example can overwrite a file that the user already owns, such as a `compose.yaml` with other services.
- A new app is not in the examples. Each one starts from a pinned scaffold command, such as `create-next-app`, and the user runs it.
- The name `prepare` is already the `lint.prepare` plugin event. A second meaning of the word would confuse the docs and the code.

Prior art:

- Symfony Flex: a recipe is declarative data, a manifest and a set of files. Flex copies the files that do not exist and keeps the others.
- shadcn: the user owns the written files. A later run never updates them.
- projen: it generates the files and updates them on each run. The user must not edit them.
- Rails templates and Yeoman: a recipe is code that runs at setup. It is hard to review, and it can run any command.

## Decision

1. A recipe is a folder of `examples/` with a `recipe.yaml`. The four folders, `go`, `nextjs`, `python` and `symfony`, are the first recipes. The binary embeds `examples/` through `examples/recipes.go`.
2. A `recipe.yaml` holds the name, a summary, the files, the scaffold steps and the next steps. It is declarative data, as in Symfony Flex. A recipe written as code is out of scope.
3. `harness recipe list` prints the recipes. `harness recipe show <name>` prints the summary, the files, the scaffold steps, the next steps and the optional `guide.md`.
4. `harness recipe apply <name> [--dry-run]` writes at the git top level. Outside git it writes in the current folder.
5. Apply writes only the missing files. It prints each path with `wrote`, `kept` or `differs`. A dry run prints `would write`, `would keep` or `differs`. An existing file is never changed. A symlink counts as kept, and apply never writes through a link.
6. Apply never runs a scaffold step. `show` prints the pinned commands, and the user or a later skill runs them in order in one shell.
7. Apply never edits `.gitignore`. A recipe with a plugin prints a next step for `.harness/*` and `!.harness/plugins/`.
8. Apply plans every path before it writes. A folder at a file path stops the whole apply with exit 1 and no write.
9. `README.md`, `recipe.yaml`, `guide.md` and `*_test.go` never ship in a recipe. A test fails when an example file is neither listed nor excluded.
10. R1 has no templating. A next step names the placeholder to edit, such as `name:` in `compose.yaml`.
11. An unknown recipe is a usage error. The command exits with the usage code and lists the known names.

The run plan splits the work in three runs. R1 adds the recipe command and the four example recipes. R2 adds the adoption guides, a concepts index and the `prepare` skill. R3 adds the framework-free TypeScript recipe `node-ddd-api`. The `show` command already prints `guide.md`, so R2 adds content and no code.

## Consequences

- A new repository gets the example files with one command, and no file of the user is overwritten.
- A user who wants a changed file merges it by hand. The `differs` line shows which files need that.
- Each recipe is data that a reviewer can read. A recipe cannot run a command on apply.
- The examples are the source of the recipes. A file added to an example is not shipped until its `recipe.yaml` lists it.
- The next steps, not apply, run the gates and the scaffold. A user who skips them has files without a working project.
