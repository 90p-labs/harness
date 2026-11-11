package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	subMerge  = "merge"
	subReady  = "ready"
	subCreate = "create"
	subEdit   = "edit"

	flagDraft   = "--draft"
	letterDraft = 'd'

	fenceMarker        = "```"
	fenceMermaid       = "mermaid"
	maxBodyWords       = 40
	wordsQuoted        = 6
	problemSeparator   = "; "
	reasonJoin         = ". "
	nextStepUser       = "leave it to the user"
	nextStepDraft      = "open the pull request with --draft"
	nextStepDiagram    = "draw it with `harness diagram`"
	nextStepBody       = "write the file in a separate call and pass a literal absolute path, or pass the text inline"
	nextStepShape      = "write the body with the lines ## Why, ## What and ## How, in this order"
	reasonBodyFileRead = "cannot read the body file: "
	nextStepBottomLine = "start the body with the bottom line: one or two sentences above %s"
	nextStepWords      = "split the line into lines of 40 words or fewer"
	nextStepEmDash     = "rewrite the line with commas, colons or sentences"
)

var (
	mergePath       = regexp.MustCompile(`pulls/\d+/merge`)
	markdownHeading = regexp.MustCompile(`^#{1,6}(\s|$)`)
	bodySections    = []string{"## Why", "## What", "## How"}
	bodyFlags       = []string{"--body", "-b"}
	bodyFileFlags   = []string{"--body-file", "-F"}
	prBodyCommands  = []string{subCreate, subEdit}
)

// Values of the pull request policy that `harness pr policy` prints.
const (
	DraftRequired DraftPolicy = "required"
	DraftOptional DraftPolicy = "optional"
	MergeUser     MergePolicy = "user"
	MergeAgent    MergePolicy = "agent"
)

// DraftPolicy says whether the pull request must open as a draft.
type DraftPolicy string

// MergePolicy says who merges the pull request.
type MergePolicy string

// PRPolicy is what the guard leaves to the agent for a pull request: whether it must open a draft, and who merges.
type PRPolicy struct {
	// Draft is DraftRequired while pr-draft-only is on, else DraftOptional.
	Draft DraftPolicy
	// Merge is MergeUser while pr-no-merge is on, else MergeAgent.
	Merge MergePolicy
}

// KnownRuleIDs returns every id that guard.disabled may name.
func KnownRuleIDs() []string {
	known := []string{RuleStopHoldsTurn, RuleReviewModels, RuleReviewFindings}
	for _, rule := range Rules() {
		known = append(known, rule.ID)
	}
	return known
}

// CheckRuleIDs rejects the first id of guard.disabled that is not a rule.
func CheckRuleIDs(disabled []string) error {
	known := KnownRuleIDs()
	for _, id := range disabled {
		if !slices.Contains(known, id) {
			return fmt.Errorf("guard.disabled names unknown rule %q", id)
		}
	}
	return nil
}

// PRPolicyOf maps the disabled rule ids to the pull request policy. It rejects an id that is not a rule.
func PRPolicyOf(disabled []string) (PRPolicy, error) {
	if err := CheckRuleIDs(disabled); err != nil {
		return PRPolicy{}, err
	}
	policy := PRPolicy{Draft: DraftRequired, Merge: MergeUser}
	if slices.Contains(disabled, RulePRDraftOnly) {
		policy.Draft = DraftOptional
	}
	if slices.Contains(disabled, RulePRNoMerge) {
		policy.Merge = MergeAgent
	}
	return policy, nil
}

func denyPRMerge(call ToolCall, _ RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if isCommand(c, cmdGh, subPR, subMerge) {
			return deny("a pull request merge is for the user", nextStepUser)
		}
		if isCommand(c, cmdGh, subAPI) && slices.ContainsFunc(c.Args, mergePath.MatchString) {
			return deny("a pull request merge through the API is for the user", nextStepUser)
		}
	}
	return "", false
}

func denyPRNotDraft(call ToolCall, _ RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if isCommand(c, cmdGh, subPR, subCreate) && !hasFlag(c, flagDraft, letterDraft) {
			return deny("a pull request opens as a draft", nextStepDraft)
		}
		if isCommand(c, cmdGh, subPR, subReady) {
			return deny("promoting a draft pull request is for the user", nextStepUser)
		}
	}
	return "", false
}

func denyPRBodyShape(call ToolCall, rc RuleContext) (string, bool) {
	for i, c := range invocations(call.Command) {
		if !isCommand(c, cmdGh, subPR) || len(c.Args) < 3 || !slices.Contains(prBodyCommands, c.Args[2]) {
			continue
		}
		if rc.TextErr != nil && len(fileNames(c, bodyFileFlags...)) > 0 {
			return deny(reasonBodyFileRead+rc.TextErr.Error(), nextStepBody)
		}
		if reason, denied := checkBody(prBody(c, textsOf(call, i))); denied {
			return reason, true
		}
	}
	return "", false
}

func prBody(c Command, files []string) string {
	if inline := flagValues(c, bodyFlags...); len(inline) > 0 {
		return strings.Join(inline, "\n")
	}
	if readsStdin(c, bodyFileFlags...) {
		return c.Heredoc
	}
	return strings.Join(files, "\n")
}

func checkBody(body string) (string, bool) {
	if strings.TrimSpace(body) == "" {
		return "", false
	}
	if problems := BodyProblems(body); len(problems) > 0 {
		return strings.Join(problems, problemSeparator), true
	}
	return "", false
}

func BodyProblems(body string) []string {
	lines := strings.Split(body, "\n")
	problems := sectionProblems(lines)
	var diagram []string
	inFence, inMermaid := false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		words := strings.Fields(line)
		if strings.ContainsRune(line, emDashRune) {
			problems = append(problems, problem(fmt.Sprintf("line %d has an em dash", i+1), nextStepEmDash))
		}
		switch {
		case strings.HasPrefix(trimmed, fenceMarker) && !inFence:
			inFence = true
			inMermaid = strings.TrimPrefix(trimmed, fenceMarker) == fenceMermaid
			diagram = nil
		case strings.HasPrefix(trimmed, fenceMarker):
			inFence = false
			if inMermaid {
				for _, mark := range shareddomain.CheckMarks(strings.Join(diagram, "\n")) {
					problems = append(problems, problem("the mermaid diagram is not marked: "+mark, nextStepDiagram))
				}
			}
			inMermaid = false
		case inFence:
			diagram = append(diagram, line)
		case len(words) > maxBodyWords:
			reason := fmt.Sprintf("line %d has %d words, more than %d words: %q", i+1, len(words), maxBodyWords, strings.Join(words[:wordsQuoted], " "))
			problems = append(problems, problem(reason, nextStepWords))
		}
	}
	return problems
}

func problem(reason, nextStep string) string {
	return reason + reasonJoin + nextStep
}

func sectionProblems(lines []string) []string {
	var outside []string
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, fenceMarker) {
			inFence = !inFence
		} else if !inFence {
			outside = append(outside, trimmed)
		}
	}
	var problems []string
	firstSection, firstName, prevAt, prevName := len(outside), bodySections[0], -1, ""
	for _, section := range bodySections {
		at := slices.Index(outside, section)
		if at < 0 {
			problems = append(problems, problem(fmt.Sprintf("the body has no %s line", section), nextStepShape))
			continue
		}
		if at < firstSection {
			firstSection, firstName = at, section
		}
		if at < prevAt {
			problems = append(problems, problem(fmt.Sprintf("%s is before %s", section, prevName), nextStepShape))
		}
		prevAt, prevName = at, section
	}
	if !slices.ContainsFunc(outside[:firstSection], isBottomLine) {
		problems = append(problems, problem("the body has no bottom line above "+firstName, fmt.Sprintf(nextStepBottomLine, firstName)))
	}
	return problems
}

func isBottomLine(line string) bool {
	return line != "" && !markdownHeading.MatchString(line)
}
