package gitdiff

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

const (
	nul            = "\x00"
	recordMark     = "\x1e"
	fieldMark      = "\x1f"
	rangeFormat    = "%s..HEAD"
	mergeBaseDiff  = "%s...HEAD"
	logFormat      = "--format=%x1e%H%x1f%s"
	plainPathsFlag = "core.quotePath=false"
)

type Reader struct{}

var _ domain.BranchReader = Reader{}

func (Reader) Read(ctx context.Context, dir, base string, testFiles []string) (domain.Branch, error) {
	span := fmt.Sprintf(rangeFormat, base)
	diff, err := git.Run(ctx, dir, nil, "diff", "--name-status", "--no-renames", "-z", fmt.Sprintf(mergeBaseDiff, base))
	if err != nil {
		return domain.Branch{}, err
	}
	log, err := git.Run(ctx, dir, nil, "-c", plainPathsFlag, "log", "--reverse", logFormat, "--name-only", span)
	if err != nil {
		return domain.Branch{}, err
	}
	tests, err := readTests(dir, testFiles)
	if err != nil {
		return domain.Branch{}, err
	}
	return domain.Branch{Files: parseDiff(diff), Commits: parseLog(log), Tests: tests}, nil
}

func parseDiff(out string) []domain.FileChange {
	parts := strings.Split(out, nul)
	var files []domain.FileChange
	for i := 0; i+1 < len(parts); i += 2 {
		files = append(files, domain.FileChange{Status: parts[i], Path: parts[i+1]})
	}
	return files
}

func parseLog(out string) []domain.BranchCommit {
	var commits []domain.BranchCommit
	for _, record := range strings.Split(out, recordMark)[1:] {
		sha, rest, _ := strings.Cut(record, fieldMark)
		subject, names, _ := strings.Cut(rest, "\n")
		commit := domain.BranchCommit{SHA: sha, Subject: subject}
		for _, name := range strings.Split(names, "\n") {
			if name = strings.TrimSpace(name); name != "" {
				commit.Files = append(commit.Files, name)
			}
		}
		commits = append(commits, commit)
	}
	return commits
}

func readTests(dir string, files []string) (map[string]string, error) {
	tests := make(map[string]string, len(files))
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		tests[file] = string(body)
	}
	return tests, nil
}
