package harness_test

import (
	"regexp"
	"strings"
	"testing"

	plandomain "github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	skillFile          = "skills/harness/SKILL.md"
	grillFile          = "skills/harness/references/grill.md"
	explorerFile       = "agents/explorer.md"
	advisorFile        = "agents/advisor.md"
	workerFile         = "agents/worker.md"
	advisorRole        = "`advisor`"
	grillingSource     = "https://github.com/mattpocock/skills/blob/main/skills/productivity/grilling/SKILL.md"
	mitLicense         = "MIT"
	explorerRole       = "`explorer`"
	runPlanCommand     = "harness run plan"
	initLintSetupLine  = "next: run the lint-setup skill"
	prOpenSkill        = "skills/pr-open/SKILL.md"
	verifierFile       = "agents/verifier.md"
	reviewingRef       = "skills/harness/references/reviewing.md"
	reviewingRefText   = "references/reviewing.md"
	findingsRowExample = "| F1 | major | real | open |"
	fixupFlag          = "--fixup"
	templateFile       = "skills/harness/references/plan-template.md"
	ruleSourceGlob     = "internal/plan/domain/*.go"
	threatModelADR     = "ADR 0019"
	reviewRisksHead    = "## Review risks"
	planningItem       = "2. `planning`"
	approvalItem       = "3. `awaiting_approval`"
	advisorOnLine      = "advisor: on, model <m>"
	modelsLine         = "`models:` line"
	adrIndexText       = "`docs/adr/index.md` first"
	configCommand      = "harness config"
	advisorOffLine     = "advisor: off"
	decisionsHead      = "## Decisions"
	handoffFile        = "handoff.md"
	statusRouteStart   = "- `status` or empty"
	skillRewriteStep   = "After each edit of `plan.md`, run `harness plan view`"
	changePlanStep     = "edit `plan.md`, run `harness plan view`"
	grillRewriteStep   = "Run `harness plan check` and `harness plan view` after each change of `plan.md`"
	panelPreviewPhrase = "host preview"
)

// The texts of the section Seam of the plan template.
const (
	seamHead      = "## Seam"
	seamHighest   = "highest seam"
	seamOne       = "one seam"
	seamLicense   = "MIT license"
	seamTddURL    = "<https://github.com/mattpocock/skills/blob/main/skills/engineering/tdd/SKILL.md>"
	seamToSpecURL = "<https://github.com/mattpocock/skills/blob/main/skills/engineering/to-spec/SKILL.md>"
)

// The handoff phrases of SKILL.md.
const (
	noEndTurnPhrase    = "Do not end the turn"
	userQuestionPhrase = "a question to the user"
	contextLimitPhrase = "context is near its limit"
	continuePhrase     = "then continue"
	staleEndPhrase     = "then end the turn"
)

var (
	ruleID       = regexp.MustCompile(`RuleID = "([a-z-]+)"`)
	grillViews   = []string{"Product", "QA", "Tech specs"}
	planViewOpen = regexp.MustCompile("harness plan view[^`\\n]*--open")
	reviewLabels = []string{"**Common list**", "**Verify**", "**Fix**", "**Re-review**", "**Final panel**"}
)

func TestSkillOpensPanelURL(t *testing.T) {
	skill := repository.Read(t, skillFile)
	var line string
	for _, l := range strings.Split(skill, "\n") {
		if strings.Contains(l, panelPreviewPhrase) {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("%s has no line that names %q", skillFile, panelPreviewPhrase)
	}
	for _, want := range []string{"`panel:`", "open"} {
		if !strings.Contains(line, want) {
			t.Errorf("the %q line of %s does not name %q: %s", panelPreviewPhrase, skillFile, want, line)
		}
	}
}

func TestHarnessSkillGrill(t *testing.T) {
	skill, grill, explorer := repository.Read(t, skillFile), repository.Read(t, grillFile), repository.Read(t, explorerFile)

	explore := strings.Index(skill, explorerRole)
	submit := strings.Index(skill, runPlanCommand)
	if explore < 0 || submit < 0 || explore > submit {
		t.Errorf("%s: the explorer (at %d) must come before %s (at %d)", skillFile, explore, runPlanCommand, submit)
	}
	advise := strings.Index(skill, advisorRole)
	if advise < explore || advise > submit {
		t.Errorf("%s: the advisor (at %d) must sit between the explorer (at %d) and %s (at %d)", skillFile, advise, explore, runPlanCommand, submit)
	}
	for _, want := range []string{grillFile[len("skills/harness/"):], shareddomain.ApprovalHeader, shareddomain.ApproveLabel, shareddomain.ChangePlanLabel} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
	wants := []string{grillingSource, mitLicense, shareddomain.RecommendedSuffix[1:]}
	for _, tool := range shareddomain.QuestionTools {
		wants = append(wants, tool.Name)
	}
	for _, want := range wants {
		if !strings.Contains(grill, want) {
			t.Errorf("%s does not name %q", grillFile, want)
		}
	}
	last := -1
	for _, view := range grillViews {
		at := strings.Index(grill, "### "+view)
		if at <= last {
			t.Errorf("%s: the view %q is missing or out of order", grillFile, view)
		}
		last = at
	}
	for path, text := range map[string]string{skillFile: skill, grillFile: grill, explorerFile: explorer, advisorFile: repository.Read(t, advisorFile), workerFile: repository.Read(t, workerFile), prOpenSkill: repository.Read(t, prOpenSkill), verifierFile: repository.Read(t, verifierFile)} {
		if strings.Contains(text, repository.EmDash) || strings.ContainsFunc(text, repository.IsEmoji) {
			t.Errorf("%s has an em dash or an emoji", path)
		}
	}
}

func TestHarnessSkillOpensPlanOnce(t *testing.T) {
	for path, steps := range map[string][]string{skillFile: {skillRewriteStep, changePlanStep}, grillFile: {grillRewriteStep}} {
		text := repository.Read(t, path)
		if open := planViewOpen.FindString(text); open != "" {
			t.Errorf("%s still runs %q, and the plan.view plugin already opens the page", path, open)
		}
		for _, step := range steps {
			if !strings.Contains(text, step) {
				t.Errorf("%s does not rewrite the page after a plan edit: %q is missing", path, step)
			}
		}
	}
}

func TestLintSetupStartsAfterInit(t *testing.T) {
	skill := repository.Read(t, skillFile)

	for _, want := range []string{initLintSetupLine, "`lint-setup`"} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
}

func TestHarnessSkillReadsReviewingReference(t *testing.T) {
	skill := repository.Read(t, skillFile)
	reviewing := repository.Read(t, reviewingRef)
	for _, want := range []string{"findings.md", "Final panel", "| Status | Model | File:line |", "`Model` cell"} {
		if !strings.Contains(reviewing, want) {
			t.Errorf("%s does not name %q", reviewingRef, want)
		}
	}
	if !strings.Contains(skill, reviewingRefText) {
		t.Errorf("%s does not name %q", skillFile, reviewingRefText)
	}
	if strings.Contains(skill, findingsRowExample) {
		t.Errorf("%s still holds the findings table example %q", skillFile, findingsRowExample)
	}
}

func TestHarnessSkillReviewRound(t *testing.T) {
	round := repository.Read(t, reviewingRef)
	last := -1
	for _, label := range reviewLabels {
		at := strings.Index(round, label)
		if at <= last {
			t.Errorf("%s: the label %s is missing or out of order", reviewingRef, label)
		}
		last = max(last, at)
	}
	if !strings.Contains(round, fixupFlag) {
		t.Errorf("%s does not name %q", reviewingRef, fixupFlag)
	}
}

func TestPlanTemplate(t *testing.T) {
	template := repository.Read(t, templateFile)
	sources, err := repository.Glob(t, ruleSourceGlob)
	if err != nil {
		t.Fatal(err)
	}
	rules := 0
	for _, path := range sources {
		for _, m := range ruleID.FindAllStringSubmatch(repository.Read(t, path), -1) {
			rules++
			if !strings.Contains(template, "`"+m[1]+"`") {
				t.Errorf("%s does not name the rule %q in backticks", templateFile, m[1])
			}
		}
	}
	if rules == 0 {
		t.Fatalf("no rule id in %s", ruleSourceGlob)
	}
	for _, want := range []string{threatModelADR, reviewRisksHead} {
		if !strings.Contains(template, want) {
			t.Errorf("%s does not name %q", templateFile, want)
		}
	}
}

func TestPlanTemplateSeam(t *testing.T) {
	template := repository.Read(t, templateFile)
	_, rest, found := strings.Cut(template, seamHead+"\n")
	if !found {
		t.Fatalf("%s has no section %q", templateFile, seamHead)
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	wants := []string{
		plandomain.SeamLabel,
		"`" + string(plandomain.RuleSeamMissing) + "`",
		seamHighest,
		seamOne,
		seamLicense,
		seamTddURL,
		seamToSpecURL,
	}
	for _, want := range wants {
		if !strings.Contains(section, want) {
			t.Errorf("the section %q of %s does not hold %q", seamHead, templateFile, want)
		}
	}
}

func TestHarnessSkillHandoff(t *testing.T) {
	skill := repository.Read(t, skillFile)
	_, planning, found := strings.Cut(skill, planningItem)
	if !found {
		t.Fatalf("%s has no item %q", skillFile, planningItem)
	}
	planning, _, _ = strings.Cut(planning, approvalItem)
	for _, want := range []string{advisorOnLine, advisorOffLine, decisionsHead} {
		if !strings.Contains(planning, want) {
			t.Errorf("%s: the item %q does not name %q", skillFile, planningItem, want)
		}
	}
	if !strings.Contains(planning, modelsLine) {
		t.Errorf("%s: the item %q does not name %q", skillFile, planningItem, modelsLine)
	}
	if !strings.Contains(skill, adrIndexText) {
		t.Errorf("%s does not name %q", skillFile, adrIndexText)
	}
	if strings.Contains(planning, configCommand) {
		t.Errorf("%s: the item %q still runs %q", skillFile, planningItem, configCommand)
	}
	for _, want := range []string{handoffFile, noEndTurnPhrase, userQuestionPhrase, contextLimitPhrase, continuePhrase} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
	if strings.Contains(skill, staleEndPhrase) {
		t.Errorf("%s still says %q", skillFile, staleEndPhrase)
	}
	_, route, _ := strings.Cut(skill, statusRouteStart)
	route, _, _ = strings.Cut(route, "\n")
	if !strings.Contains(route, handoffFile) {
		t.Errorf("%s: the status route does not name %q", skillFile, handoffFile)
	}
}

func TestHarnessShippingPolicy(t *testing.T) {
	skill := repository.Read(t, skillFile)
	_, step, found := strings.Cut(skill, "6. `shipping`")
	if !found {
		t.Fatalf("%s has no shipping item", skillFile)
	}
	step, _, _ = strings.Cut(step, "\n")
	for _, want := range []string{"harness pr policy", "merge: user", "merge: agent", "checks pass"} {
		if !strings.Contains(step, want) {
			t.Errorf("%s: the shipping step does not name %q", skillFile, want)
		}
	}
	if report, done := strings.Index(step, "harness run report"), strings.Index(step, "harness run done --pr <url>"); report < 0 || done < report {
		t.Errorf("%s: the shipping step must run harness run report, then harness run done --pr <url>", skillFile)
	}
	for _, want := range []string{
		"With `merge: user`, the user merges.",
		"With `draft: optional` and `merge: agent`, the agent merges after the checks pass.",
		"In every other case, the user merges.",
	} {
		if !strings.Contains(step, want) {
			t.Errorf("%s: the shipping step does not say %q", skillFile, want)
		}
	}
	for _, old := range []string{"The user merges it.", "as a draft, with", "With `merge: agent`, the agent merges"} {
		if strings.Contains(step, old) {
			t.Errorf("%s: the shipping step still says %q", skillFile, old)
		}
	}
}

const (
	learningFile      = "skills/harness/references/learning.md"
	learningRefText   = "references/learning.md"
	learnCommand      = "harness run learn"
	shippingItem      = "6. `shipping`"
	reviewerFile      = "agents/reviewer.md"
	adrDir            = "`docs/adr/`"
	contextFile       = "`CONTEXT.md`"
	standardsRulesRow = "- `standards-rules`:"
	lensColumnHead    = "| File:line | Lens |"
	lensCellText      = "`Lens` cell"
	standardsLensHead = "## Lens Standards"
	conventionsHeader = "`Conventions`"
	writeOption       = "`Write`"
	skipOption        = "`Skip`"
	noDuplicatePhrase = "no duplicate"
	verifyCommitsText = "harness verify --commits"
)

var (
	learningHomes  = []string{"lint rule", "docs/adr/index.md", "CONTEXT.md` entry"}
	learningFields = []string{"State the rule", "why it matters", "bad example", "good example", "files that follow it"}
	learningSearch = "search `CONTEXT.md`, `docs/adr/` and the lint pack"
)

func TestHarnessSkillLearnStep(t *testing.T) {
	skill, learning := repository.Read(t, skillFile), repository.Read(t, learningFile)

	_, step, found := strings.Cut(skill, shippingItem)
	if !found {
		t.Fatalf("%s has no shipping item", skillFile)
	}
	step, _, _ = strings.Cut(step, "\n")
	if learn, open := strings.Index(step, learnCommand), strings.Index(step, "`pr-open`"); learn < 0 || open < 0 || learn > open {
		t.Errorf("%s: the shipping step must name %s (at %d) before pr-open (at %d)", skillFile, learnCommand, learn, open)
	}
	if !strings.Contains(step, learningRefText) {
		t.Errorf("%s: the shipping step does not name %q", skillFile, learningRefText)
	}

	last := -1
	for _, home := range learningHomes {
		at := strings.Index(learning, home)
		if at <= last {
			t.Errorf("%s: the home %q is missing or out of order", learningFile, home)
		}
		last = max(last, at)
	}
	wants := []string{noDuplicatePhrase, learningSearch, conventionsHeader, writeOption, skipOption, verifyCommitsText, "`docs:`", "harness run start"}
	wants = append(wants, learningFields...)
	for _, tool := range shareddomain.QuestionTools {
		wants = append(wants, tool.Name)
	}
	for _, want := range wants {
		if !strings.Contains(learning, want) {
			t.Errorf("%s does not name %q", learningFile, want)
		}
	}
	if write, skip := strings.Index(learning, writeOption), strings.Index(learning, skipOption); write < 0 || skip < write {
		t.Errorf("%s: the option Write must come before Skip", learningFile)
	}
	if strings.Contains(learning, repository.EmDash) || strings.ContainsFunc(learning, repository.IsEmoji) {
		t.Errorf("%s has an em dash or an emoji", learningFile)
	}
}

func TestConventionReadPath(t *testing.T) {
	skill, reviewer, explorer, reviewing := repository.Read(t, skillFile), repository.Read(t, reviewerFile), repository.Read(t, explorerFile), repository.Read(t, reviewingRef)

	_, lens, found := strings.Cut(reviewer, standardsLensHead)
	if !found {
		t.Fatalf("%s has no section %q", reviewerFile, standardsLensHead)
	}
	lens, _, _ = strings.Cut(lens, "\n## ")
	_, row, found := strings.Cut(skill, standardsRulesRow)
	if !found {
		t.Fatalf("%s has no row %q", skillFile, standardsRulesRow)
	}
	row, _, _ = strings.Cut(row, "\n")
	for _, want := range []string{adrDir, contextFile} {
		if !strings.Contains(lens, want) {
			t.Errorf("%s: the lens Standards does not name %s", reviewerFile, want)
		}
		if !strings.Contains(row, want) {
			t.Errorf("%s: the standards-rules row does not name %s", skillFile, want)
		}
	}
	for _, want := range []string{"docs/adr/", "CONTEXT.md"} {
		if !strings.Contains(explorer, want) {
			t.Errorf("%s does not name %q", explorerFile, want)
		}
	}
	for _, want := range []string{lensColumnHead, lensCellText} {
		if !strings.Contains(reviewing, want) {
			t.Errorf("%s does not name %q", reviewingRef, want)
		}
	}
}

func TestHarnessSkillAmend(t *testing.T) {
	skill := repository.Read(t, skillFile)
	for _, want := range []string{
		"edit `plan.md` first, then run `harness run amend`",
		"ask the `Plan` question again, then run `harness run approve`",
		"With `run.approval: false`, the run goes to `working`",
		"A kept-structure amend needs a recorded approval and a kept structure",
		"A docs-only amend needs a plan that took the docs-only waiver, with no recorded approval",
		"Both skip the approval only in `working` with no live review round",
		"Any other amend needs the `Plan` answer",
		"`harness run plan` never follows an amend",
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
}
