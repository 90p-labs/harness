package docs_test

import (
	"slices"
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
	lessonHomeBase   = []string{"follow-up `harness run start` prompt", "`docs:` commit"}
	amendmentHomes   = append(slices.Clone(lessonHomeBase), lastMessage)
	lessonHomes      = append(slices.Clone(lessonHomeBase), reportSkill)
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
	text, _, foundTrace := strings.Cut(after, traceHeading)
	if !foundTrace {
		t.Fatalf("%s has no heading %q", learnADR, traceHeading)
	}
	return text
}

func TestADRRecordsTheLessonAmendment(t *testing.T) {
	text := amendmentText(t)
	wants := []string{learnCommand, evidenceRuleText, countLineRuleText, mechanicalRule, memoryRule, lessonDropped, creditName, "5863806", pointerHome, noDuplicateText}
	wants = append(wants, lessonSources...)
	wants = append(wants, lessonEvidenceID...)
	wants = append(wants, lessonCategories...)
	wants = append(wants, amendmentHomes...)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s: the amendment does not name %q", learnADR, want)
		}
	}
	status := repository.Section(t, repository.Read(t, learnADR), statusHead)
	if !strings.Contains(status, "amendment") {
		t.Errorf("%s: the status line does not name the amendment", learnADR)
	}
}

const (
	traceHeading    = "## Amendment 2026-11-14, lesson trace"
	statusHead      = "## Status"
	followUpsFile   = "`docs/follow-ups.md`"
	reportSkill     = "`report` skill"
	citedBy         = "Cited by"
	followUpsHead   = "## Follow-ups"
	pluginLessons   = "## Plugin lessons"
	noDuplicateSec  = "## No duplicate"
	followUpSearch  = "grep -n '^### ' docs/follow-ups.md"
	runFolder       = "run folder"
	lastMessage     = "last message"
	reportDraft     = "`report-<n>.md`"
	reportSkillDoc  = "skills/report/SKILL.md"
	followUpTerm    = "- **Follow-up**"
	waitCommand     = "harness run wait --pr <url>"
	harnessSkill    = "skills/harness/SKILL.md"
	keepStepText    = "Keep the draft"
	skipKeepText    = "A kept draft of a plugin lesson stays"
	conventionSkip  = "`Write` and `Skip` and an empty question do not drop the draft"
	noStopRule      = "does not stop the turn at step 5"
	aloneNoQuestion = "With a plugin lesson alone, the lead asks no `Conventions` question"
	writeHead       = "## Write"
	unattended      = "Unattended"
	draftCount      = "`<n>` counts the drafts of the run from 1"
	draftDelete     = "deletes both copies of the draft"
	lessonQuestion  = "Is it a lesson of the learn step about the plugin?"
	lessonTerm      = "- **Lesson**"
	layoutSentence  = "A new file starts with the title `# Follow-ups`, one line that names `" + learningRef + "`, the heading `## Open`, and then its entries."
	missingSentence = "A missing file means that no cause is open"
	commentDraft    = "the draft is a comment for that issue"
	noStopStep3     = "does not stop the turn at step 3"
	matchComment    = "a comment for that issue"
	repeatComment   = "repeat of a plugin lesson becomes a comment on its issue"
	repeatException = "a repeat of an open cause adds a `Cited by` line to its entry"
	listNoPlugin    = "The list holds no plugin lesson."
	commentYes      = "continues at the yes for the comment"
	sameAtStep3     = "The same holds for a match at step 3 of the skill."
	noSeamPrompt    = "a `no-seam` prompt get one too"
	commentDelete   = "posts the comment with the `gh issue comment` command of step 3. Then it deletes each copy of the draft"
	learnPointer    = `During the learn step, read "From the learn step".`
	duplicateHead   = "## 3. Look for a duplicate"
	treeFence       = "```"
	treeBranch      = "├── "
	treeLastBranch  = "└── "
	treeBranchCount = 2
	harnessQuestion = "Is the problem in the harness?"
	homesHead       = "## Homes"
	fromLearnHead   = "## From the learn step"
	decisionHead    = "## Decision tree"
	reminderText    = "after `harness run wait --pr <url>` and before `harness run done`"
	reminderUser    = "With a user, the lead asks the confirmation of each kept report draft and comment draft"
	unattendedPlug  = "Unattended means that the user told the lead to ask nothing"
	unattendedSkill = "Unattended, the lead asks nothing"
	countRule       = "reads the entry of the cause to count"
	titleKeepRule   = "name it in `handoff.md`, with its title"
	titleSkillRule  = "writes the title of the draft next to its path in `handoff.md`"
	titleUseRule    = "a later `/90p:report` with the path uses that title"
	noSeamADR       = "`no-seam` prompt of ADR 0025 gets an entry too"
)

func TestADRRecordsTheLessonTrace(t *testing.T) {
	adr := repository.Read(t, learnADR)
	_, text, found := strings.Cut(adr, traceHeading)
	if !found {
		t.Fatalf("%s has no heading %q", learnADR, traceHeading)
	}
	for _, want := range []string{followUpsFile, citedBy, reportSkill, "decision 7", runFolder, lastMessage, waitCommand, "stop rule", unattended, repeatComment, noSeamADR} {
		if !strings.Contains(text, want) {
			t.Errorf("%s: the lesson trace amendment does not name %q", learnADR, want)
		}
	}
	status := repository.Section(t, adr, statusHead)
	for _, want := range []string{followUpsFile, reportSkill} {
		if !strings.Contains(status, want) {
			t.Errorf("%s: the status does not name %q", learnADR, want)
		}
	}
}

func TestLearningReferenceKeepsTheFollowUps(t *testing.T) {
	doc := repository.Read(t, learningRef)
	lessons, follow, plugin, question := strings.Index(doc, lessonsHead), strings.Index(doc, followUpsHead), strings.Index(doc, pluginLessons), strings.Index(doc, questionHead)
	if lessons < 0 || follow < lessons || plugin < follow || question < plugin {
		t.Fatalf("%s: the sections %q, %q, %q and %q are not in this order", learningRef, lessonsHead, followUpsHead, pluginLessons, questionHead)
	}
	section := repository.Section(t, doc, followUpsHead)
	for _, want := range []string{followUpSearch, citedBy, categoryPrefix, citedByPrefix, promptPrefix, conventionCategory, promptCommand, followUpsFile, "`Conventions`", layoutSentence, missingSentence, noSeamPrompt, countRule} {
		if !strings.Contains(section, want) {
			t.Errorf("%s: the section Follow-ups does not name %q", learningRef, want)
		}
	}
	duplicate := repository.Section(t, doc, noDuplicateSec)
	for _, want := range []string{"search `CONTEXT.md`, `docs/adr/` and the lint pack", followUpsFile} {
		if !strings.Contains(duplicate, want) {
			t.Errorf("%s: the section No duplicate does not name %q", learningRef, want)
		}
	}
	if !strings.Contains(repository.Section(t, doc, homesHead), followUpsFile) {
		t.Errorf("%s: the section Homes does not name %s", learningRef, followUpsFile)
	}
}

func TestPluginLessonGoesToTheReportSkill(t *testing.T) {
	doc := repository.Read(t, learningRef)
	if strings.Contains(doc, "One line in the last message of the lead, for a lesson about the harness plugin") {
		t.Errorf("%s still sends a plugin lesson to one line of the last message", learningRef)
	}
	section := repository.Section(t, doc, pluginLessons)
	for _, want := range []string{reportSkill, "`/90p:report`", "`$90p:report`", "files nothing", runFolder, reportDraft, "`handoff.md`", lastMessage, waitCommand, conventionSkip, noStopRule, sameAtStep3, aloneNoQuestion, commentDraft, unattendedPlug, titleKeepRule} {
		if !strings.Contains(section, want) {
			t.Errorf("%s: the section Plugin lessons does not name %q", learningRef, want)
		}
	}
	if strings.Contains(repository.Section(t, doc, writeHead), keepStepText) {
		t.Errorf("%s: the section Write still holds %q", learningRef, keepStepText)
	}
	question := repository.Section(t, doc, questionHead)
	for _, want := range []string{skipKeepText, listNoPlugin} {
		if !strings.Contains(question, want) {
			t.Errorf("%s: the section Question does not name %q", learningRef, want)
		}
	}
	skill := repository.Section(t, repository.Read(t, reportSkillDoc), fromLearnHead)
	for _, want := range []string{runFolder, reportDraft, lastMessage, "`/90p:report`", waitCommand, draftCount, draftDelete, noStopRule, noStopStep3, matchComment, commentYes, commentDelete, unattendedSkill, titleSkillRule, titleUseRule} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s: the section From the learn step does not name %q", reportSkillDoc, want)
		}
	}
	duplicate := repository.Section(t, repository.Read(t, reportSkillDoc), duplicateHead)
	if !strings.Contains(duplicate, learnPointer) {
		t.Errorf("%s: step 3 does not hold %q", reportSkillDoc, learnPointer)
	}
	tree := repository.Section(t, repository.Read(t, reportSkillDoc), decisionHead)
	_, afterFence, _ := strings.Cut(tree, treeFence)
	first, _, _ := strings.Cut(afterFence, treeFence)
	branches := strings.Count(first, treeBranch) + strings.Count(first, treeLastBranch)
	if !strings.Contains(first, harnessQuestion) || branches != treeBranchCount || strings.Contains(first, "learn step") {
		t.Errorf("%s: the first tree must ask %q, hold %d branches and not name the learn step", reportSkillDoc, harnessQuestion, treeBranchCount)
	}
	if !strings.Contains(tree, lessonQuestion) {
		t.Errorf("%s: the decision tree must ask %q below them", reportSkillDoc, lessonQuestion)
	}
	for _, want := range []string{reminderText, reminderUser} {
		if !strings.Contains(repository.Read(t, harnessSkill), want) {
			t.Errorf("%s does not name %q in the shipping phase", harnessSkill, want)
		}
	}
	if !strings.Contains(repository.Read(t, reportSkillDoc), "learn step") {
		t.Errorf("%s does not name the learn step", reportSkillDoc)
	}
}

func TestContextNamesTheFollowUp(t *testing.T) {
	context := repository.Read(t, learnContextDoc)
	at, lesson, plugin := strings.Index(context, followUpTerm), strings.Index(context, lessonTerm), strings.Index(context, pluginHead)
	if at < 0 || lesson < 0 {
		t.Fatalf("%s has no entry %q or %q", learnContextDoc, followUpTerm, lessonTerm)
	}
	if at < lesson || at > plugin {
		t.Errorf("%s: the entry %q must sit after Lesson and above %q", learnContextDoc, followUpTerm, pluginHead)
	}
	entry, _, _ := strings.Cut(context[at:], "\n")
	for _, want := range []string{"`" + promptCommand + "`", followUpsFile, "`" + citedBy + "`"} {
		if !strings.Contains(entry, want) {
			t.Errorf("%s: the entry Follow-up does not name %q", learnContextDoc, want)
		}
	}
}

func TestLearningReferenceNamesTheLessons(t *testing.T) {
	doc := repository.Read(t, learningRef)
	at, fields, question := strings.Index(doc, lessonsHead), strings.Index(doc, fieldsHead), strings.Index(doc, questionHead)
	if fields < 0 || question < 0 {
		t.Fatalf("%s has no heading %q or %q", learningRef, fieldsHead, questionHead)
	}
	if at < 0 || at < fields || at > question {
		t.Fatalf("%s: %q must come after %q and before %q", learningRef, lessonsHead, fieldsHead, questionHead)
	}
	section := repository.Section(t, doc, lessonsHead)
	wants := []string{evidenceRuleText, countLineRuleText, mechanicalRule, memoryRule, pointerHome, noRepeatRule, repeatException}
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
	for _, term := range []string{"- **Run evidence**", lessonTerm} {
		if !strings.Contains(context, term) {
			t.Errorf("%s has no entry %q", learnContextDoc, term)
		}
	}
	plugin := strings.Index(context, pluginHead)
	if plugin < 0 {
		t.Fatalf("%s has no heading %q", learnContextDoc, pluginHead)
	}
	for _, term := range []string{"- **Run evidence**", lessonTerm} {
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
