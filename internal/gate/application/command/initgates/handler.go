package initgates

import (
	"context"

	"github.com/jorge07/harness/internal/gate/domain"
)

type Command struct {
	Dir string
}

type Detect func(dir string) ([]domain.Gate, error)

type HasGates func(dir string) (bool, error)

type WriteGates func(dir string, gates []domain.Gate) error

type Handler struct {
	detect Detect
	has    HasGates
	write  WriteGates
}

func NewHandler(detect Detect, has HasGates, write WriteGates) Handler {
	return Handler{detect: detect, has: has, write: write}
}

func (h Handler) Handle(_ context.Context, c Command) ([]domain.Gate, error) {
	exists, err := h.has(c.Dir)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrGatesExist
	}
	gates, err := h.detect(c.Dir)
	if err != nil {
		return nil, err
	}
	if err := h.write(c.Dir, gates); err != nil {
		return nil, err
	}
	return gates, nil
}
