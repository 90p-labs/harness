package domain

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	HandoffLimit      = 4096
	handoffClip       = 12
	HandoffSizeFormat = "handoff.md has %d bytes, the limit is %d"

	handoffPhaseKey   = "Phase"
	handoffStepKey    = "Step"
	handoffSectionTag = "## "
	handoffItemTag    = "- "

	handoffNext       = "## Next"
	handoffFindings   = "## Open findings"
	handoffUnreviewed = "## Unreviewed commits"
	handoffDecisions  = "## Decisions"
)

var (
	ErrHandoffMissing = errors.New("handoff.md is missing")

	handoffSections = []string{handoffNext, handoffFindings, handoffUnreviewed, handoffDecisions}
)

type HandoffReader interface {
	ReadHandoff(ctx context.Context, worktree, runID string) ([]byte, error)
}

func CheckHandoff(run Run, raw []byte) error {
	if raw == nil {
		return ErrHandoffMissing
	}
	if len(raw) > HandoffLimit {
		return fmt.Errorf(HandoffSizeFormat, len(raw), HandoffLimit)
	}
	headers := map[string]string{}
	items := map[string]int{}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case strings.HasPrefix(line, handoffSectionTag):
			section = line
			items[section] = 0
		case strings.HasPrefix(line, handoffItemTag):
			items[section]++
		default:
			if key, value, ok := strings.Cut(line, ": "); ok && section == "" {
				headers[key] = strings.TrimSpace(value)
			}
		}
	}
	var problems []string
	if phase, ok := headers[handoffPhaseKey]; !ok {
		problems = append(problems, "no "+handoffPhaseKey+" line")
	} else if phase != string(run.Phase) {
		problems = append(problems, fmt.Sprintf("phase %s, want %s", clip(phase), run.Phase))
	}
	if stepText, ok := headers[handoffStepKey]; !ok {
		problems = append(problems, "no "+handoffStepKey+" line")
	} else if step, err := strconv.Atoi(stepText); err != nil || step != run.Step {
		problems = append(problems, fmt.Sprintf("step %s, want %d", clip(stepText), run.Step))
	}
	var bad []string
	for _, name := range handoffSections {
		if items[name] == 0 {
			bad = append(bad, name)
		}
	}
	if len(bad) > 0 {
		problems = append(problems, fmt.Sprintf("needs %s, each with a %q line", strings.Join(bad, ", "), handoffItemTag))
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New("handoff.md: " + strings.Join(problems, "; "))
}

// HandoffSections returns a copy of the section names a handoff needs, in order.
func HandoffSections() []string {
	return append([]string(nil), handoffSections...)
}

// clip bounds a value read from the file to handoffClip bytes, cut on a rune
// boundary, so a refusal stays short in bytes.
func clip(value string) string {
	if len(value) <= handoffClip {
		return value
	}
	end := handoffClip
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "..."
}
