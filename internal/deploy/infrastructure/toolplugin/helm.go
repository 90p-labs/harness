package toolplugin

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/90p-labs/harness/internal/deploy/domain"
)

const (
	helmBinary   = "helm"
	helmTemplate = "template"
)

func Template(ctx context.Context, dir, chart string) ([]byte, error) {
	binary, err := exec.LookPath(helmBinary)
	if err != nil {
		return nil, fmt.Errorf("%w: rendering %s needs %s on the path", domain.ErrToolMissing, chart, helmBinary)
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, helmTemplate, chart)
	cmd.Dir = dir
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s %s: %w: %s", helmBinary, helmTemplate, chart, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}
