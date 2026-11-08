package domain

import (
	"errors"
	"fmt"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const hostSeparator = ", "

var (
	ErrNoHost    = errors.New("no host found: pass --host claude or --host codex")
	ErrManyHosts = errors.New("more than one host found: pass --host")
)

func OneHost(found []shareddomain.Host) (shareddomain.Host, error) {
	switch len(found) {
	case 0:
		return "", ErrNoHost
	case 1:
		return found[0], nil
	}
	names := make([]string, len(found))
	for i, host := range found {
		names[i] = string(host)
	}
	return "", fmt.Errorf("%w (%s)", ErrManyHosts, strings.Join(names, hostSeparator))
}
