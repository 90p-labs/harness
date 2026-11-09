package domain

import (
	"fmt"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	RuleReviewModels = "review-models"

	minReviewModels = 2

	nextStepReviewModels = "run the reviewer on a second model, then approve the review again"
	NextStepReviewerList = "list 2 or more models under roles.reviewer, or add " + RuleReviewModels + " to guard.disabled"
)

func IsReviewerAgent(agentType string) bool {
	return agentRole(agentType) == shareddomain.RoleReviewer
}

func DenyReviewModels(models, listed []string) (string, bool) {
	if len(models) >= minReviewModels {
		return "", false
	}
	found := noModel
	if len(models) > 0 {
		found = strings.Join(models, modelSeparator)
	}
	nextStep := nextStepReviewModels
	if len(listed) > 0 && len(listed) < minReviewModels {
		nextStep = NextStepReviewerList
	}
	return deny(
		fmt.Sprintf("the review of this HEAD needs %d different models, and it has %s", minReviewModels, found),
		nextStep,
	)
}
