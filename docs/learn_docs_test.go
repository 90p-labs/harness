package docs_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	learnADR       = "docs/adr/0024-learn-conventions-at-ship.md"
	learnIndex     = "docs/adr/index.md"
	learnIndexRow  = "| [0024](0024-learn-conventions-at-ship.md) | 0024 Learn conventions at ship |"
	learnCommand   = "harness run learn"
	learnSpan      = "run.learn"
	learnReadme    = "README.md"
	learnDesign    = "DESIGN.md"
	learnConfigDoc = "docs/configuration.md"
	learnAudit     = "`harness run learn [<run id>]` prints"
)

var learnDecisions = []string{
	"read-only query", "`run.learn`", "`pr-open`", "lens `standards`", "`gate-checkable` rework",
	"source `user`", "`Conventions`", "`harness run start` prompt", "`docs/adr/index.md`", "`CONTEXT.md`",
}

const (
	amendmentHeading  = "## Amendment 2026-11-14"
	amendmentCut      = "## Amendment"
	consequencesHead  = "## Consequences"
	lessonsHead       = "## Lessons"
	fieldsHead        = "## Fields"
	questionHead      = "## Question"
	learningRef       = "skills/harness/references/learning.md"
	learnContextDoc   = "CONTEXT.md"
	evidencePrefix    = "`evidence:`"
	retroCredit       = "https://github.com/mattpocock/skills/blob/5863806/skills/engineering/retro/SKILL.md"
	evidenceRuleText  = "lesson with no evidence"
	countLineRuleText = "count line alone supports no lesson"
	mechanicalRule    = "mechanical mistake gets a deterministic check, never a sentence of guidance"
	memoryRule        = "memory of the lead is not evidence"
	creditName        = "Matt Pocock"
	pluginHead        = "## Plugin terms"
	pointerHome       = "line of its ADR in `docs/adr/index.md`"
	noDuplicateText   = "no-duplicate search"
	noRepeatRule      = "does not repeat a convention"
)

var (
	lessonCategories = []string{"Navigation", "Automated checks", "Tool economy", "No-ops", "Information access"}
	lessonEvidenceID = []string{"`rework-<n>`", "`block-<n>`", "`amends`", "`stop-blocks`"}
	lessonHomes      = []string{"follow-up `harness run start` prompt", "`docs:` commit", "last message"}
	lessonSources    = []string{"an `evidence:` line", "a `candidate:` line", "a row of `findings.md`", "a line of `handoff.md`"}
	lessonDropped    = "dropped"
)

func amendmentText(t *testing.T) string {
	t.Helper()
	adr := repository.Read(t, learnADR)
	before, after, found := strings.Cut(adr, amendmentHeading)
	if !found {
		t.Fatalf("%s has no heading %q", learnADR, amendmentHeading)
	}
	if !strings.Contains(before, consequencesHead) {
		t.Errorf("%s: %q must come after %q", learnADR, amendmentHeading, consequencesHead)
	}
	return after
}

func TestADRRecordsTheLessonAmendment(t *testing.T) {
	text := amendmentText(t)
	wants := []string{learnCommand, evidenceRuleText, countLineRuleText, mechanicalRule, memoryRule, lessonDropped, creditName, "5863806", pointerHome, noDuplicateText}
	wants = append(wants, lessonSources...)
	wants = append(wants, lessonEvidenceID...)
	wants = append(wants, lessonCategories...)
	wants = append(wants, lessonHomes...)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s: the amendment does not name %q", learnADR, want)
		}
	}
	status := repository.Section(t, repository.Read(t, learnADR), "## Status")
	if !strings.Contains(status, "amendment") {
		t.Errorf("%s: the status line does not name the amendment", learnADR)
	}
}

func TestLearningReferenceNamesTheLessons(t *testing.T) {
	doc := repository.Read(t, learningRef)
	at, fields, question := strings.Index(doc, lessonsHead), strings.Index(doc, fieldsHead), strings.Index(doc, questionHead)
	if at < 0 || at < fields || at > question {
		t.Fatalf("%s: %q must come after %q and before %q", learningRef, lessonsHead, fieldsHead, questionHead)
	}
	section := repository.Section(t, doc, lessonsHead)
	wants := []string{evidenceRuleText, countLineRuleText, mechanicalRule, memoryRule, pointerHome, noRepeatRule}
	wants = append(wants, lessonSources...)
	wants = append(wants, lessonCategories...)
	wants = append(wants, lessonHomes...)
	for _, want := range wants {
		if !strings.Contains(section, want) {
			t.Errorf("%s: the section Lessons does not name %q", learningRef, want)
		}
	}
	if !strings.Contains(doc, retroCredit) {
		t.Errorf("%s does not hold the credit link %q", learningRef, retroCredit)
	}
	amendment := amendmentText(t)
	for _, category := range lessonCategories {
		if !strings.Contains(amendment, category) || !strings.Contains(section, category) {
			t.Errorf("the ADR amendment and %s do not both name the category %q", learningRef, category)
		}
	}
}

func TestDocsNameTheRunEvidence(t *testing.T) {
	audit := repository.Section(t, repository.Read(t, learnConfigDoc), "### Audit a run")
	_, learnPara, _ := strings.Cut(audit, learnAudit)
	learnPara, _, _ = strings.Cut(learnPara, "\n\n")
	if !strings.Contains(learnPara, evidencePrefix) {
		t.Errorf("%s: the audit paragraph of %s does not name %s", learnConfigDoc, learnCommand, evidencePrefix)
	}
	var rows int
	for _, line := range strings.Split(repository.Read(t, learnReadme), "\n") {
		if strings.Contains(line, learnCommand) && strings.HasPrefix(line, "|") {
			rows++
			if !strings.Contains(line, "run evidence") {
				t.Errorf("%s: the command row does not name the run evidence", learnReadme)
			}
		}
	}
	if rows == 0 {
		t.Errorf("%s has no command row for %s", learnReadme, learnCommand)
	}
	context := repository.Read(t, learnContextDoc)
	for _, term := range []string{"- **Run evidence**", "- **Lesson**"} {
		if !strings.Contains(context, term) {
			t.Errorf("%s has no entry %q", learnContextDoc, term)
		}
	}
	plugin := strings.Index(context, pluginHead)
	if plugin < 0 {
		t.Fatalf("%s has no heading %q", learnContextDoc, pluginHead)
	}
	for _, term := range []string{"- **Run evidence**", "- **Lesson**"} {
		if strings.Index(context, term) > plugin {
			t.Errorf("%s: the entry %q must sit under the run terms", learnContextDoc, term)
		}
	}
}

func TestRunsPageShowsTheShippingHint(t *testing.T) {
	const stale = "next: open the pull request and run harness run wait"
	const current = "next: run harness run learn, open the pull request and run harness run wait --pr URL, then after the merge harness run done --pr URL"
	page := repository.Read(t, "docs/runs.md")
	if strings.Contains(page, stale) {
		t.Errorf("docs/runs.md shows the shipping hint with no learn step: %q", stale)
	}
	if !strings.Contains(page, current) {
		t.Errorf("docs/runs.md does not show the shipping hint %q", current)
	}
}

func TestDocsNameTheLearnStep(t *testing.T) {
	adr, _, _ := strings.Cut(repository.Read(t, learnADR), amendmentCut)
	for _, want := range learnDecisions {
		if !strings.Contains(adr, want) {
			t.Errorf("%s does not record %q", learnADR, want)
		}
	}
	if !strings.Contains(repository.Read(t, learnConfigDoc), learnAudit) {
		t.Errorf("%s does not describe %q", learnConfigDoc, learnAudit)
	}

	index := repository.Read(t, learnIndex)
	if !strings.Contains(index, learnIndexRow) {
		t.Errorf("%s has no row %q", learnIndex, learnIndexRow)
	}

	for _, doc := range []string{learnReadme, learnDesign, learnConfigDoc} {
		if !strings.Contains(repository.Read(t, doc), learnCommand) {
			t.Errorf("%s does not name %q", doc, learnCommand)
		}
	}

	for _, doc := range []string{learnDesign, learnConfigDoc} {
		if !strings.Contains(repository.Read(t, doc), learnSpan) {
			t.Errorf("%s does not name the span %q", doc, learnSpan)
		}
	}
}
