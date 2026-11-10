# Replay: ddd-depth

- Run: 20261109-164815-make-the-harness-ddd-skill-a-full-ddd-co
- Base: 124e867
- Prompt: the original prompt of the run. Rework reasons: only those recorded before the run start (27 lines).
- Explorer: haiku, brief agents/explorer.md blob 5071d488d57c82ba37c2e03154bcddeb5ff21925, one sample.
- Advisor: fable (agent type 90p:advisor, told to follow the new brief file), brief agents/advisor.md blob 2cdd7b856e726873a85edbba675089b5231fb028, one sample, on the first plan draft of the run.
- Redaction: the scratchpad prefix is written as <scratch>/.
- Known limit: this sample misses the cross-context lint conflict. A sample of blob 7a4eaf0f found it.

## Explorer report

EXPLORER REPORT: ddd skill depth (repo HEAD 124e867, clean). No file edited.

FACTS
1. skills/ddd: SKILL.md 100 lines; references go.md 52, php.md 48, python.md 50, typescript.md 92.
2. skills_test.go has no word or line limit for skill files. Only agent reports say "under N words" (skills_test.go:26). The prompt's claim is wrong.
3. ddd_skill_test.go: TestDDDSkillRuleIDs checks every ddd/ and ddd-es/ id under skills/ddd. TestDDDSkillAgnostic makes SKILL.md name every rule id and forbid biome, deptrac, import-linter, go-arch-lint, depguard and language extensions. TestDDDSkillReferences routes only the four language files. These pass now.
4. The checker hard-codes src/ (internal/lint/domain/layout.go:33) and kernel.ts (layout.go:40). Only .ts and .tsx get suffix checks (layout.go:96).
5. Past reworks (run 20261109-005950, 6 lines): invalid python contract, SKILL misstates context-layers, copied configs drift, agnostic test is case-sensitive. Point to tested examples, do not copy configs.

CHECKER CONFLICTS (first)
C1. Kodify use-case folders and files (application/<use-case>/handler.ts, request.ts) fail ddd/file-suffix (layout.go:217, empty suffix) and ddd/cqrs-folders.
C2. Bare names (user.ts), authorized-use-case.ts, use-case-handler.ts, *.openapi.ts, listeners, subscribers and validators have no known suffix (layout.go:81-86).
C3. No known suffix accepts a non-repository domain port (logger, clock, signer).
C4. x.test.ts in a layer folder fails (suffix "test"). x.spec.ts passes. Kodify's tests/unit mirror tree conflicts with ADR 0005.
C5. ddd/command-handler-returns (grit) checks only method execute. Kodify and hollywood use handle, so the rule never fires on them.
C6. ddd-es/apply-handler (eventsourcing.go:22) needs apply<Event>(. Hollywood's preferred registerHandler form fails it (llm-skills/creating-aggregates.md). Event-sourced *.entity.ts children are not checked.
C7. ddd-es/event-version fails hollywood's creating-aggregates example, whose events have no version field.
C8. ddd-es/projection-read-only (eventsourcing.go:50) matches suffix repository exactly. An import of a *-repository adapter passes.
C9. SKILL.md line 44 says an unplaced file is a finding. SrcRoots (layout.go:260) checks only the folder under src/. A file in src/contexts/order/read-model/ passes, and layerOf (layout.go:204) gives it no layer.
C10. ContextLayers (layout.go:282) flags exactly one layer. A context with zero layers passes.
C11. SKILL.md places schemas in infrastructure. Kodify places request.schema.ts in application. No rule decides.

FILES TOUCHED
- skills/ddd/SKILL.md (router), skills/ddd/references/*.md (4 language files plus the new leaves), ddd_skill_test.go.
- skills_test.go only if a limit is added.
- Context: none. Layer: none. The harness is not a DDD project, and these docs are outside src/.

REVIEW RISKS (template classes)
R1 Doc drift (29): each leaf routed from SKILL.md. Test: extend TestDDDSkillReferences to the leaves.
R2 Prose (20): each leaf sentence at most 25 words. Test: word check on the leaves.
R3 Proof cannot fail (15): TestDDDSkillAgnostic reads SKILL.md only. Test: a .ts string in a leaf must fail it, shown by a probe.
R4 Guidance vs checker (10): each of C1 to C11 is a decision below.

THREAT MODEL: not needed. No guard, security, approval, permission or secret path.

OPEN DECISIONS
D1 (C1) Request shape. (a) commands/<name>/<name>.command.ts and handler, validation in the message constructor. (b) Kodify request.ts. Recommend (a): only it passes the checker.
D2 (C2, C3) Ports and helpers. (a) Add port to the known suffixes (checker change). (b) Forbid ports. Recommend (a): shared clock and logger need a home.
D3 (C4) Tests. (a) Colocated *.spec.*. (b) Mirror tree, rejected by ADR 0005. Recommend (a).
D4 (C5) Command return. (a) Teach void and file a checker fix for handle. (b) Change the grit rule now. Recommend (a): keep this change small.
D5 (C6) Events. (a) Teach apply<Event>. (b) Extend the checker to accept registerHandler. Recommend (a) now.
D6 (C9) Unplaced files. (a) Rewrite SKILL.md line 44 to the real scope. (b) Extend SrcRoots. Recommend (a), and file the gap.
D7 Composition. (a) Kodify plain module and kernel. (b) Hollywood container with string keys. Recommend (a): no library, typed wiring, ADR 0005.
D8 Delivery edge. (a) Controllers in infrastructure, which the checker covers. (b) A ui/ folder, which no rule covers. Recommend (a).
D9 (C11) Schemas. (a) Infrastructure, as SKILL.md says. (b) Application, as kodify says. Recommend (a): it keeps the SKILL.md table.

## Advisor report

Advisor report: plan `<scratch>/replay/ddd-depth-plan.md`, repository `<scratch>/repo-ddd` (base 124e867).

Findings:

- blocker, plan.md:38-171: `harness plan check` refuses the plan, so `harness run plan` cannot pass. observed. Proof: `go run ./cmd/harness plan check --file <plan>` gives exit 1, `error: plan has problems`: 18 `tree-format`, 11 `trace-unlinked`, 15 `sentence-length`. Causes: the trees have no tee or last markers; criteria 1, 2, 5, 6, 8 and 10 are in no `Criteria:` line; lines 104 to 142 have sentences of 26 to 35 words.
- major, plan.md:147: the code tree root is `skills/ddd/`. The parser drops the root from each path (`internal/plan/domain/parse.go:495-496`), so `SKILL.md` matches no `Owned files` entry after the indent fix. inferred. Use the repository root as the tree root.
- major, plan.md:91: task 5 `Test first` starts with `TestDDDSkillTopics` in backticks and owns no test file. `testOf` (`internal/plan/domain/verify.go:99`) reads it as the file, so `harness verify` gives `test-missing`. inferred.
- major, plan.md:36-47: the plan has no `## Review risks` section. The template requires it, and no risk has a test or a decision. observed.
- major, plan.md:40-41: criteria 3 and 4 can pass on the base. A loop on zero topic files passes, and `SKILL.md` has 948 words. inferred. Proof: `go test -run 'TestDDDSkillTopics|TestDDDSkillTopicsAgnostic|TestDDDSkillSize' .` gives `ok [no tests to run]`. Require a fixed list of the eleven names in each test.
- major, plan.md:46: the criterion 9 proof greps one rule id. It passes when the query handler half is absent. observed.
- major, plan.md:44: the criterion 7 proof is weak. `grep -qi position` matches "composition", and `record` and `apply` match any prose. inferred.
- major, plan.md:8: the goal says "no language and no tool", and the prompt forbids text that names kodify. Criterion 3 checks only 5 lint tools and extensions. No proof refuses a language name, a framework name or kodify. observed (`lint_setup_test.go:28`).
- major, plan.md:98: the reason is wrong. `TestDDDSkillAgnostic` needs only the 5 catalogue ids. Each rule then lives in the router and in a topic, and no test reads both. observed (`ddd_skill_test.go:72`).
- major, plan.md:107 and 138: the delivery edge topic puts consumers, jobs and CLI commands in `infrastructure`. The roles of `ddd/file-suffix` in `skills/ddd/SKILL.md` (Naming) have no consumer, job or CLI role. inferred.
- minor, plan.md:104: "only the module wires a context" conflicts with `skills/ddd/references/php.md:34` (the service config wires) and `go.md:34` (the composition root wires). observed.
- minor, plan.md:109: kebab case in an agnostic topic conflicts with `go.md:35` (`place_order`) and `python.md:38`. observed.
- minor, plan.md:117: `SKILL.md` has 948 words, the limit is 1100, and the tree gains 11 routes. inferred.
- minor, plan.md:38: "routes" is not defined. The present test uses `strings.Contains` on the full page, not on the tree fence. observed.
- minor, plan.md:116: a topic that writes "the `ddd` pack" or "Node.js" fails the agnostic check. Tell the workers. inferred.
- minor, plan.md:112: the hollywood docs folder has no file with "outbox"; only ADR 0005 gives it. observed (grep count 0).
- minor, plan.md:118: `DESIGN.md:1275` and `README.md:17` still say "the DDD layout" with one reference file for each language. The prompt keeps them out of scope; record the follow-up. observed.
- minor, size: 11 files of up to 700 words, with the tests and the router, give about 700 to 900 changed lines (the source set has 593 lines). This is at the 800 line limit. inferred.

Checked with no finding: `harness lint catalogue` prints 5 ids, as line 115 says. `grep -rli hollywood skills/ddd` is empty on the base, so criterion 8 can fail. ADR 0005 supports lines 108 and 112. The source bans a bus and a container (`composition-root.md:5`), as line 103 says.

Verdict: BLOCKED
