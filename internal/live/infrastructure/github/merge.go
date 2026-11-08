package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jorge07/harness/internal/live/domain"
)

var _ domain.GitHub = (*Client)(nil)

const (
	prStateMerged = "MERGED"
	prStateClosed = "CLOSED"
)

type Client struct {
	bin string
}

func New(bin string) *Client {
	return &Client{bin: bin}
}

func (c *Client) Merge(ctx context.Context, prURL string) (domain.Merge, error) {
	out, err := c.run(ctx, "pr", "view", prURL, "--json", "state,mergedAt,mergeCommit")
	if err != nil {
		return domain.Merge{}, err
	}
	var view struct {
		State       string    `json:"state"`
		MergedAt    time.Time `json:"mergedAt"`
		MergeCommit *struct {
			OID string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err := json.Unmarshal(out, &view); err != nil {
		return domain.Merge{}, fmt.Errorf("decode pull request %s: %w", prURL, err)
	}
	switch {
	case view.State == prStateMerged && view.MergeCommit != nil && view.MergeCommit.OID != "":
		return domain.Merge{State: domain.MergeMerged, SHA: view.MergeCommit.OID, At: view.MergedAt}, nil
	case view.State == prStateClosed:
		return domain.Merge{State: domain.MergeClosed}, nil
	default:
		return domain.Merge{State: domain.MergeOpen}, nil
	}
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
