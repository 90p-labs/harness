package oxlint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const (
	binaryName    = "oxlint"
	localBinDir   = "node_modules/.bin"
	formatFlag    = "--format=json"
	configFlag    = "-c"
	findingsExit  = 1
	missingFormat = "%w: %s is not in %s or on the path, install it with npm"
)

type Runner struct{}

func New() Runner {
	return Runner{}
}

func (Runner) Name() domain.EngineName {
	return domain.EngineOxlint
}

func (Runner) Prepare(dir string, rules []domain.Rule) error {
	return writeConfig(dir, ownRules(rules))
}

func (Runner) Run(ctx context.Context, dir string, rules []domain.Rule) ([]shareddomain.Finding, error) {
	rules = ownRules(rules)
	if len(rules) == 0 {
		return nil, nil
	}
	binary, err := locate(dir)
	if err != nil {
		return nil, err
	}
	files, err := git.Files(ctx, dir, scopes(rules)...)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	args := append([]string{formatFlag, configFlag, filepath.Join(domain.LintDir, configFile)}, files...)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	parsed, parseErr := parseReport(stdout.Bytes())
	if parseErr != nil {
		return nil, fmt.Errorf("%w: oxlint failed: %v: %s", parseErr, runErr, bytes.TrimSpace(stderr.Bytes()))
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() != findingsExit {
		return nil, fmt.Errorf("oxlint failed: %w: %s", runErr, bytes.TrimSpace(stderr.Bytes()))
	}
	return parsed.findings(dir, rules), nil
}

func ownRules(rules []domain.Rule) []domain.Rule {
	return slices.DeleteFunc(slices.Clone(rules), func(rule domain.Rule) bool {
		return rule.Engine != domain.EngineOxlint
	})
}

func scopes(rules []domain.Rule) []string {
	globs := make([]string, 0, len(rules))
	for _, rule := range rules {
		globs = append(globs, rule.Scope)
	}
	return globs
}

func locate(dir string) (string, error) {
	local := filepath.Join(dir, localBinDir, binaryName)
	if found, err := exec.LookPath(local); err == nil {
		return found, nil
	}
	found, err := exec.LookPath(binaryName)
	if err != nil {
		return "", fmt.Errorf(missingFormat, domain.ErrEngineMissing, binaryName, localBinDir)
	}
	return found, nil
}
