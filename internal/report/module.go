package report

import (
	"github.com/90p-labs/harness/internal/report/application/command/fileissue"
	"github.com/90p-labs/harness/internal/report/infrastructure/github"
	gh "github.com/90p-labs/harness/internal/shared/infrastructure/github"
)

func NewFileIssue() fileissue.Handler {
	return fileissue.NewHandler(github.New(gh.Binary))
}
