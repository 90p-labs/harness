package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/90p-labs/harness/internal/live/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	statusSuccess  = "success"
	statusFailure  = "failure"
	statusError    = "error"
	statusInactive = "inactive"
)

type deploymentRecord struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type statusRecord struct {
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) Deployment(ctx context.Context, prURL, sha, environment string) (domain.Deployment, error) {
	pr, err := shareddomain.ParsePRURL(prURL)
	if err != nil {
		return domain.Deployment{}, err
	}
	repo := fmt.Sprintf("repos/%s/%s", pr.Owner, pr.Repo)
	out, err := c.run(ctx, "api", fmt.Sprintf("%s/deployments?sha=%s&environment=%s", repo, url.QueryEscape(sha), url.QueryEscape(environment)))
	if err != nil {
		return domain.Deployment{}, err
	}
	var deployments []deploymentRecord
	if err := json.Unmarshal(out, &deployments); err != nil {
		return domain.Deployment{}, fmt.Errorf("decode deployments of %s: %w", prURL, err)
	}
	if len(deployments) == 0 {
		return domain.Deployment{State: domain.DeployPending}, nil
	}
	newest := deployments[0]
	for _, d := range deployments[1:] {
		if d.CreatedAt.After(newest.CreatedAt) || (d.CreatedAt.Equal(newest.CreatedAt) && d.ID > newest.ID) {
			newest = d
		}
	}
	out, err = c.run(ctx, "api", fmt.Sprintf("%s/deployments/%d/statuses", repo, newest.ID))
	if err != nil {
		return domain.Deployment{}, err
	}
	var statuses []statusRecord
	if err := json.Unmarshal(out, &statuses); err != nil {
		return domain.Deployment{}, fmt.Errorf("decode deployment statuses of %s: %w", prURL, err)
	}
	var latest statusRecord
	for _, s := range statuses {
		if s.State != statusInactive && (latest.State == "" || s.CreatedAt.After(latest.CreatedAt)) {
			latest = s
		}
	}
	switch latest.State {
	case statusSuccess:
		return domain.Deployment{State: domain.DeploySuccess, At: latest.CreatedAt}, nil
	case statusFailure, statusError:
		return domain.Deployment{State: domain.DeployFailed}, nil
	default:
		return domain.Deployment{State: domain.DeployPending}, nil
	}
}
