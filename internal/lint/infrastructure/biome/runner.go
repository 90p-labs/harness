package biome

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	"github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const (
	binary       = "biome"
	minMajor     = 2
	localBinPath = "node_modules/.bin"
)

var versionPattern = regexp.MustCompile(`(\d+)\.\d+\.\d+`)

type Engine struct {
	plugins fs.FS
}

func New(plugins fs.FS) *Engine {
	return &Engine{plugins: plugins}
}

func (*Engine) Name() domain.EngineName {
	return domain.EngineBiomeGrit
}

func (e *Engine) Prepare(dir string, rules []domain.Rule) error {
	for glob, scoped := range groupByScope(rules) {
		for _, rule := range scoped {
			if rule.Plugin == "" {
				continue
			}
			if err := writePlugin(e.plugins, dir, rule.Plugin); err != nil {
				return err
			}
		}
		if err := writeConfig(dir, glob, buildConfig(scoped)); err != nil {
			return err
		}
	}
	return nil
}

func (*Engine) Run(ctx context.Context, dir string, rules []domain.Rule) ([]shareddomain.Finding, error) {
	scopes := groupByScope(rules)
	if len(scopes) == 0 {
		return nil, nil
	}
	tool, err := locate(ctx, dir)
	if err != nil {
		return nil, err
	}
	globs := make([]string, 0, len(scopes))
	for glob := range scopes {
		globs = append(globs, glob)
	}
	slices.Sort(globs)
	var findings []shareddomain.Finding
	for _, glob := range globs {
		files, err := git.Files(ctx, dir, glob)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}
		raw, err := lint(ctx, tool, dir, glob, files)
		if err != nil {
			return nil, err
		}
		scoped, err := MapReport(raw, scopes[glob])
		if err != nil {
			return nil, err
		}
		findings = append(findings, scoped...)
	}
	return findings, nil
}

func lint(ctx context.Context, tool, dir, glob string, files []string) ([]byte, error) {
	args := append([]string{"lint", "--reporter=json", "--config-path", scopeDir(glob)}, files...)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.Len() == 0 && err != nil {
		return nil, fmt.Errorf("biome lint: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

func locate(ctx context.Context, dir string) (string, error) {
	tool := filepath.Join(dir, filepath.FromSlash(localBinPath), binary)
	if _, err := exec.LookPath(tool); err != nil {
		tool, err = exec.LookPath(binary)
		if err != nil {
			return "", fmt.Errorf("%w: %s is not in %s or on the path, install Biome %d or later", domain.ErrEngineMissing, binary, localBinPath, minMajor)
		}
	}
	out, err := exec.CommandContext(ctx, tool, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s --version: %w", domain.ErrEngineMissing, tool, err)
	}
	match := versionPattern.FindSubmatch(out)
	if match == nil {
		return "", fmt.Errorf("%w: unreadable %s version %q", domain.ErrEngineMissing, binary, bytes.TrimSpace(out))
	}
	if major, _ := strconv.Atoi(string(match[1])); major < minMajor {
		return "", fmt.Errorf("%w: Biome %s is too old, install Biome %d or later", domain.ErrEngineMissing, match[0], minMajor)
	}
	return tool, nil
}

var _ domain.Engine = (*Engine)(nil)
