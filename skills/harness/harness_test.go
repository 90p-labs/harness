package harness_test

import (
	"strings"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	skillFile         = "skills/harness/SKILL.md"
	grillFile         = "skills/harness/references/grill.md"
	explorerFile      = "agents/explorer.md"
	advisorFile       = "agents/advisor.md"
	workerFile        = "agents/worker.md"
	advisorRole       = "`advisor`"
	grillingSource    = "https://github.com/mattpocock/skills/blob/main/skills/productivity/grilling/SKILL.md"
	mitLicense        = "MIT"
	explorerRole      = "`explorer`"
	runPlanCommand    = "harness run plan"
	initLintSetupLine = "next: run the lint-setup skill"
	prOpenSkill       = "skills/pr-open/SKILL.md"
	verifierFile      = "agents/verifier.md"
	reviewingItem     = "5. `reviewing`"
	shippingItem      = "6. `shipping`"
	fixupFlag         = "--fixup"
)

var (
	grillViews   = []string{"Product", "QA", "Tech specs"}
	reviewLabels = []string{"**Common list**", "**Verify**", "**Fix**", "**Re-review**", "**Final panel**"}
)

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

func TestLintSetupStartsAfterInit(t *testing.T) {
	skill := repository.Read(t, skillFile)

	for _, want := range []string{initLintSetupLine, "`lint-setup`"} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
}

func TestHarnessSkillReviewRound(t *testing.T) {
	skill := repository.Read(t, skillFile)
	_, round, found := strings.Cut(skill, reviewingItem)
	if !found {
		t.Fatalf("%s has no item %q", skillFile, reviewingItem)
	}
	round, _, found = strings.Cut(round, shippingItem)
	if !found {
		t.Fatalf("%s has no item %q", skillFile, shippingItem)
	}
	last := -1
	for _, label := range reviewLabels {
		at := strings.Index(round, label)
		if at <= last {
			t.Errorf("%s: the label %s is missing or out of order in the item %q", skillFile, label, reviewingItem)
		}
		last = max(last, at)
	}
	if !strings.Contains(round, fixupFlag) {
		t.Errorf("%s does not name %q in the item %q", skillFile, fixupFlag, reviewingItem)
	}
}
