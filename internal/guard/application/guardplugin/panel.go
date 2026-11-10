package guardplugin

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

// PanelSource reads the facts of the final panel that Deps does not hold.
// A nil function is a missing fact, so the panel is every listed model.
type PanelSource struct {
	LastRework    func(ctx context.Context, dir, runID string) (domain.ReworkEvent, error)
	ReviewedByAll func(ctx context.Context, dir, runID string, models []string) (head string, err error)
	FixupLines    func(ctx context.Context, dir, from string) (int, error)
	Limit         int
}

// ReviewerLists returns the reviewer models of each host in role order, without duplicates.
func ReviewerLists(models map[shareddomain.Host]map[string][]string) map[shareddomain.Host][]string {
	lists := make(map[shareddomain.Host][]string, len(models))
	for host, roles := range models {
		var list []string
		for _, model := range roles[string(shareddomain.RoleReviewer)] {
			if !slices.Contains(list, model) {
				list = append(list, model)
			}
		}
		if len(list) > 0 {
			lists[host] = list
		}
	}
	return lists
}

// FinalPanel gathers the facts of the run and picks the final review panel.
// The guard and `harness review panel` both call it, so they print the same panel.
// counted are the models with a counted review, which pick the host list. It returns that host and list too.
func FinalPanel(ctx context.Context, deps Deps, runID string, counted []string) (host shareddomain.Host, list, panel []string, reason string) {
	lists := ReviewerLists(deps.Models)
	host, list = domain.FullestList(counted, lists)
	full := func(why string) (shareddomain.Host, []string, []string, string) {
		return host, list, slices.Clone(list), "full panel: " + why
	}
	if len(list) == 0 || runID == "" {
		return full("no reviewer list or no run")
	}
	src := deps.Panel
	if src.LastRework == nil || src.ReviewedByAll == nil || src.FixupLines == nil {
		return full("a fact source is missing")
	}
	findings, err := readFindings(deps, runID)
	if err != nil {
		return full("findings.md has no readable findings table")
	}
	last, err := src.LastRework(ctx, deps.Worktree, runID)
	if err != nil {
		return full(fmt.Sprintf("the rework events are unreadable: %v", err))
	}
	facts := domain.PanelFacts{Listed: list, Findings: findings, Last: last, Limit: src.Limit}
	if last.Name == shareddomain.TransitionRequestRework {
		head, err := src.ReviewedByAll(ctx, deps.Worktree, runID, list)
		if err != nil {
			return full(fmt.Sprintf("the review records are unreadable: %v", err))
		}
		if head == "" {
			return full("no HEAD was reviewed by every listed model")
		}
		if lines, err := src.FixupLines(ctx, deps.Worktree, head); err == nil {
			facts.Fixup = domain.FixupSize{Lines: lines, Known: true}
		}
	}
	panel, reason = domain.PanelModels(facts)
	return host, list, panel, reason
}

func readFindings(deps Deps, runID string) ([]shareddomain.ReviewFinding, error) {
	text, err := deps.ReadFile(filepath.Join(deps.Worktree, shareddomain.RunFindingsFile(runID)))
	if err != nil {
		return nil, err
	}
	return shareddomain.ParseReviewFindings(string(text))
}
