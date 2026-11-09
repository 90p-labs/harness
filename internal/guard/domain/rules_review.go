package domain

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	RuleReviewModels = "review-models"

	minReviewModels = 2

	nextStepReviewModels  = "run the reviewer on a second model, then approve the review again"
	nextStepMissingModels = "run a reviewer on each missing model, then approve the review again"
	NextStepReviewerList  = "list 2 or more models under roles.reviewer, or add " + RuleReviewModels + " to guard.disabled"
)

func IsReviewerAgent(agentType string) bool {
	return agentRole(agentType) == shareddomain.RoleReviewer
}

func DenyReviewModels(models []string, listed map[shareddomain.Host][]string) (string, bool) {
	host, list := fullestList(models, listed)
	found := noModel
	if len(models) > 0 {
		found = strings.Join(models, modelSeparator)
	}
	var missing []string
	for _, model := range list {
		if !slices.Contains(models, model) {
			missing = append(missing, model)
		}
	}
	if len(models) < minReviewModels {
		reason := fmt.Sprintf("the review of this HEAD needs %d different models, and it has %s", minReviewModels, found)
		if len(missing) > 0 {
			reason += fmt.Sprintf(" and misses %s of %s", strings.Join(missing, modelSeparator), host)
		}
		nextStep := nextStepReviewModels
		if len(list) > 0 && len(list) < minReviewModels {
			nextStep = NextStepReviewerList
		} else if len(missing) > 0 {
			nextStep = nextStepMissingModels
		}
		return deny(reason, nextStep)
	}
	if len(missing) == 0 {
		return "", false
	}
	return deny(
		fmt.Sprintf("the review of this HEAD needs every reviewer model of %s, and it has %s and misses %s",
			host, found, strings.Join(missing, modelSeparator)),
		nextStepMissingModels,
	)
}

func fullestList(models []string, listed map[shareddomain.Host][]string) (shareddomain.Host, []string) {
	hosts := slices.Sorted(maps.Keys(listed))
	var best shareddomain.Host
	var bestList []string
	bestHits := -1
	for _, host := range hosts {
		hits := 0
		for _, model := range listed[host] {
			if slices.Contains(models, model) {
				hits++
			}
		}
		if hits > bestHits {
			best, bestList, bestHits = host, listed[host], hits
		}
	}
	return best, bestList
}
