package github

import (
	"context"
	"strings"

	"github.com/90p-labs/harness/internal/report/domain"
	gh "github.com/90p-labs/harness/internal/shared/infrastructure/github"
)

const (
	harnessRepo = "jorge07/harness"
	bugLabel    = "bug"
	stdinFile   = "-"
)

var _ domain.Tracker = Tracker{}

type Tracker struct {
	bin string
}

func New(bin string) Tracker {
	return Tracker{bin: bin}
}

func (t Tracker) Create(ctx context.Context, title, body string) (string, error) {
	out, err := gh.Run(ctx, t.bin, strings.NewReader(body),
		"issue", "create", "-R", harnessRepo, "--title", title, "--label", bugLabel, "--body-file", stdinFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
