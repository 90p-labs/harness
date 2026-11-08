package report

import (
	"github.com/jorge07/harness/internal/report/application/command/fileissue"
	"github.com/jorge07/harness/internal/report/infrastructure/github"
	gh "github.com/jorge07/harness/internal/shared/infrastructure/github"
)

func NewFileIssue() fileissue.Handler {
	return fileissue.NewHandler(github.New(gh.Binary))
}
