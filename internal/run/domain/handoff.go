package domain

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	phase, ok := headers[handoffPhaseKey]
	if !ok {
		return fmt.Errorf("handoff.md has no %s line", handoffPhaseKey)
	}
	if phase != string(run.Phase) {
		return fmt.Errorf("handoff.md has phase %s, want %s", clip(phase), run.Phase)
	}
	stepText, ok := headers[handoffStepKey]
	if !ok {
		return fmt.Errorf("handoff.md has no %s line", handoffStepKey)
	}
	if step, err := strconv.Atoi(stepText); err != nil || step != run.Step {
		return fmt.Errorf("handoff.md has step %s, want %d", clip(stepText), run.Step)
	}
	for _, name := range handoffSections {
		count, found := items[name]
		if !found {
			return fmt.Errorf("handoff.md has no %s section", name)
		}
		if count == 0 {
			return fmt.Errorf("handoff.md section %s has no %q line", name, strings.TrimSpace(handoffItemTag))
		}
	}
	return nil
}

// clip bounds a value read from the file so a refusal stays short.
func clip(value string) string {
	if runes := []rune(value); len(runes) > handoffClip {
		return string(runes[:handoffClip]) + "..."
	}
	return value
}
