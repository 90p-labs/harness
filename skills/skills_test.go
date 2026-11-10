package skills_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	skillFile           = "skills/harness/SKILL.md"
	explorerFile        = "agents/explorer.md"
	skillDocName        = "SKILL.md"
	skillDocs           = "skills/*/" + skillDocName
	agentDocs           = "agents/*.md"
	designFile          = "DESIGN.md"
	readmeFile          = "README.md"
	lintSetupSkill      = "skills/lint-setup/SKILL.md"
	linterAgent         = "agents/linter.md"
	reviewerAgent       = "agents/reviewer.md"
	frontmatterEnd      = "\n---\n"
	skillsDir           = "skills"
	decisionTreeHeading = "## Decision tree"
	levelTwoHeading     = "## "
	fenceOpen           = "```"
	skillKind           = "skill"
	agentKind           = "agent"
	modelFieldText      = "`model` field"

	workingItemStart      = "4. `working`"
	reviewingItemStart    = "5. `reviewing`"
	verifyCommitsText     = "harness verify --commits"
	runReviewText         = "harness run review"
	selfCheckFileText     = "`self-check.md`"
	reviewerChecklistText = "lens checklists of `agents/reviewer.md`"
	proofCommandText      = "proof command"
	proofPipeEscapeText   = "A `|` in a proof command is written `\\|` in the proof table."
	selfCheckHeadText     = "Head: <full sha of HEAD>"
	selfCheckProofHeader  = "| # | Proof | Exit |"
	selfCheckWorkerText   = "the first model of the `worker` role"
	selfCheckEvidenceText = "<criterion or file:line>"
	resultPass            = "`pass`"
	resultFixed           = "`fixed`"
	resultOpen            = "`open`"
	configurationDoc      = "docs/configuration.md"
	leadRowPrefix         = "| `lead` |"
	lightLeadText         = "A light-track run can use `sonnet` as the lead."
	lightNoSwitchText     = "On a light track, the lead does not ask a `sonnet` session to switch."
	diffStatFirstText     = "Read a diff with `git diff --stat` first, then only the files that you check."
	tailPipefailText      = "set -o pipefail; go test ./... 2>&1 | tail -n 40"
	verifyForegroundText  = "Run `harness verify --commits <default branch>` in the foreground, with a Bash timeout of 600000 ms (600 s)."
	verifyBackgroundText  = "When it does not fit in 600 s, run it in the background."
)

var (
	selfCheckIDs       = []string{"spec-proof", "spec-scope", "spec-text", "standards-rules", "standards-judgement"}
	roleAgents         = []string{"explorer", "advisor", "worker", "reviewer", "verifier"}
	crossReference     = regexp.MustCompile("`([a-z][a-z-]*)` (" + skillKind + "|" + agentKind + ")\\b")
	roleAgentLine      = regexp.MustCompile("`(" + strings.Join(roleAgents, "|") + ")` agents?\\b|model of the (?:`(?:" + strings.Join(roleAgents, "|") + ")` )?role\\b")
	requiredReferences = map[string][]string{
		linterAgent:    {"ddd"},
		lintSetupSkill: {"ddd"},
		explorerFile:   {"ddd"},
		reviewerAgent:  {"ddd"},
		skillFile:      {"pr-open", "lint-setup", "ddd"},
	}
)

func pluginDocs(t *testing.T) map[string]string {
	t.Helper()
	docs := map[string]string{}
	err := filepath.WalkDir(repository.Path(t, skillsDir), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".md" {
			docs[repository.Relative(t, path)] = repository.Read(t, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	agents, err := repository.Glob(t, agentDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range agents {
		docs[repository.Relative(t, path)] = repository.Read(t, path)
	}
	return docs
}

func TestSkillDecisionTree(t *testing.T) {
	skills, err := repository.Glob(t, skillDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range skills {
		_, body, _ := strings.Cut(strings.TrimPrefix(repository.Read(t, path), "---\n"), frontmatterEnd)
		var headings []int
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, levelTwoHeading) {
				headings = append(headings, i)
			}
		}
		if len(headings) == 0 || lines[headings[0]] != decisionTreeHeading {
			t.Errorf("%s: the first level-2 heading is not %q", path, decisionTreeHeading)
			continue
		}
		next := ""
		for _, line := range lines[headings[0]+1:] {
			if next = strings.TrimSpace(line); next != "" {
				break
			}
		}
		if !strings.HasPrefix(next, fenceOpen) {
			t.Errorf("%s: %q has no fence below it", path, decisionTreeHeading)
		}
	}
}

func TestSkillCrossReferencesExist(t *testing.T) {
	for path, doc := range pluginDocs(t) {
		for _, m := range crossReference.FindAllStringSubmatch(doc, -1) {
			target := filepath.Join(skillsDir, m[1], skillDocName)
			if m[2] == agentKind {
				target = filepath.Join(filepath.Dir(agentDocs), m[1]+".md")
			}
			if _, err := os.Stat(repository.Path(t, target)); err != nil {
				t.Errorf("%s names the %s %s, which has no file %s", path, m[2], m[1], target)
			}
		}
	}
}

func TestSkillCrossReferencesRequired(t *testing.T) {
	for path, names := range requiredReferences {
		doc := repository.Read(t, path)
		for _, name := range names {
			if !strings.Contains(doc, "`"+name+"` "+skillKind) {
				t.Errorf("%s does not name the `%s` skill", path, name)
			}
		}
	}
}

func TestSkillsNoEmDash(t *testing.T) {
	docs := pluginDocs(t)
	for _, path := range []string{readmeFile, designFile} {
		docs[repository.Relative(t, path)] = repository.Read(t, path)
	}
	for path, doc := range docs {
		if strings.Contains(doc, repository.EmDash) {
			t.Errorf("%s has an em dash", path)
		}
	}
}

func TestSkillSelfCheckStep(t *testing.T) {
	doc := repository.Read(t, skillFile)
	start := strings.Index(doc, workingItemStart)
	end := strings.Index(doc, reviewingItemStart)
	if start < 0 || end < start {
		t.Fatalf("%s has no phase working item before the phase reviewing item", skillFile)
	}
	item := doc[start:end]
	verify := strings.Index(item, verifyCommitsText)
	review := strings.Index(item, runReviewText)
	selfCheck := strings.Index(item, selfCheckFileText)
	if verify < 0 || selfCheck < verify || review < 0 || review < selfCheck {
		t.Errorf("the working item must name %q, then %s, then %q", verifyCommitsText, selfCheckFileText, runReviewText)
	}
	pins := []string{
		reviewerChecklistText, proofCommandText, proofPipeEscapeText,
		selfCheckHeadText, selfCheckProofHeader, selfCheckWorkerText, selfCheckEvidenceText,
		resultPass, resultFixed, resultOpen,
	}
	for _, want := range append(pins, selfCheckIDs...) {
		if !strings.Contains(item, want) {
			t.Errorf("the working item does not name %q", want)
		}
	}
}

func TestSkillLeadCostLines(t *testing.T) {
	doc := repository.Read(t, skillFile)
	start := strings.Index(doc, workingItemStart)
	end := strings.Index(doc, reviewingItemStart)
	if start < 0 || end < start {
		t.Fatalf("%s has no phase working item before the phase reviewing item", skillFile)
	}
	item := doc[start:end]
	pins := []string{
		lightLeadText, lightNoSwitchText, diffStatFirstText, tailPipefailText,
		verifyForegroundText, verifyBackgroundText,
	}
	for _, want := range pins {
		if !strings.Contains(item, want) {
			t.Errorf("the working item does not name %q", want)
		}
	}
	var leadRow string
	for _, line := range strings.Split(repository.Read(t, configurationDoc), "\n") {
		if strings.HasPrefix(line, leadRowPrefix) {
			leadRow = line
		}
	}
	if !strings.Contains(leadRow, lightLeadText) {
		t.Errorf("the lead row of %s does not name %q", configurationDoc, lightLeadText)
	}
}

func TestSkillAgentStartsNameModelField(t *testing.T) {
	skills, err := repository.Glob(t, skillDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range skills {
		started := map[string]bool{}
		for i, line := range strings.Split(repository.Read(t, path), "\n") {
			match := roleAgentLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			if !strings.Contains(line, modelFieldText) {
				t.Errorf("%s:%d names a role agent or a role model and does not name the %s", path, i+1, modelFieldText)
			} else if path == skillFile {
				started[match[1]] = true
			}
		}
		if path != skillFile {
			continue
		}
		for _, role := range roleAgents {
			if !started[role] {
				t.Errorf("%s starts no %s agent with the %s", skillFile, role, modelFieldText)
			}
		}
	}
}
