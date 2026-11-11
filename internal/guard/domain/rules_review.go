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

type Review struct {
	Model     string
	PatchID   string
	RunID     string
	FilesRead []string
}

type ReviewScope struct {
	PatchID string
	RunID   string
}

func ReviewedModels(reviews []Review) (counted, unread []string) {
	for _, review := range reviews {
		if len(review.FilesRead) > 0 {
			counted = append(counted, review.Model)
		} else {
			unread = append(unread, review.Model)
		}
	}
	slices.Sort(counted)
	counted = slices.Compact(counted)
	slices.Sort(unread)
	unread = slices.DeleteFunc(slices.Compact(unread), func(model string) bool { return slices.Contains(counted, model) })
	return counted, unread
}

// ReviewFloor is the count of different models a review needs: one on a light review, else the full panel floor.
func ReviewFloor(light bool) int {
	if light {
		return 1
	}
	return minReviewModels
}

// DenyReviewModels denies a review that has fewer counted models than floor, or misses a listed model.
func DenyReviewModels(models, unread []string, listed map[shareddomain.Host][]string, floor int) (string, bool) {
	host, list := FullestList(models, listed)
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
	if len(models) < floor {
		reason := fmt.Sprintf("the review of this HEAD needs %d different models, and it has %s", floor, found)
		if len(missing) > 0 {
			reason += fmt.Sprintf(" and misses %s of %s", strings.Join(missing, modelSeparator), host)
		}
		reason += unreadNote(unread)
		nextStep := nextStepReviewModels
		if len(list) > 0 && len(list) < floor {
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
			host, found, strings.Join(missing, modelSeparator))+unreadNote(unread),
		nextStepMissingModels,
	)
}

func unreadNote(unread []string) string {
	if len(unread) == 0 {
		return ""
	}
	return fmt.Sprintf("; %s read no changed file, so the review does not count", strings.Join(unread, modelSeparator))
}

// FullestList picks the host list with the most recorded models, the list the review-models rule checks.
func FullestList(models []string, listed map[shareddomain.Host][]string) (shareddomain.Host, []string) {
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
