package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
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
	nextStepUser       = "leave it to the user"
	nextStepDraft      = "open the pull request with --draft"
	nextStepDiagram    = "draw it with `harness diagram`"
	nextStepBody       = "pass the text inline, or write the file in a separate call"
	nextStepShape      = "write the body with the lines ## Why, ## What and ## How, in this order"
	reasonBodyFileRead = "cannot read the body file: "
	nextStepWords      = "split the line into lines of 40 words or fewer"
)

var (
	mergePath      = regexp.MustCompile(`pulls/\d+/merge`)
	bodySections   = []string{"## Why", "## What", "## How"}
	bodyFlags      = []string{"--body", "-b"}
	bodyFileFlags  = []string{"--body-file", "-F"}
	prBodyCommands = []string{subCreate, subEdit}
)

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
	lines := strings.Split(body, "\n")
	if reason, denied := checkSections(lines); denied {
		return reason, true
	}
	var diagram []string
	inFence, inMermaid := false, false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, fenceMarker) && !inFence:
			inFence = true
			inMermaid = strings.TrimPrefix(trimmed, fenceMarker) == fenceMermaid
			diagram = nil
		case strings.HasPrefix(trimmed, fenceMarker):
			inFence = false
			if inMermaid {
				if problems := shareddomain.CheckMarks(strings.Join(diagram, "\n")); len(problems) > 0 {
					return deny("the mermaid diagram is not marked: "+problems[0], nextStepDiagram)
				}
			}
			inMermaid = false
		case inFence:
			diagram = append(diagram, line)
		case len(strings.Fields(line)) > maxBodyWords:
			return deny(fmt.Sprintf("a body line has more than %d words", maxBodyWords), nextStepWords)
		}
	}
	return "", false
}

func checkSections(lines []string) (string, bool) {
	at := make([]int, len(bodySections))
	for i, section := range bodySections {
		at[i] = slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == section })
		if at[i] < 0 {
			return deny(fmt.Sprintf("the body has no %s line", section), nextStepShape)
		}
	}
	for i := 1; i < len(at); i++ {
		if at[i] < at[i-1] {
			return deny(fmt.Sprintf("%s is before %s", bodySections[i], bodySections[i-1]), nextStepShape)
		}
	}
	return "", false
}
