package initgates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jorge07/harness/internal/gate/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const proposalSeparator = "\n"

type Command struct {
	Dir string
}

type Detect func(dir string) ([]domain.Gate, error)

type Files func(ctx context.Context, dir string) ([]string, error)

type HasGates func(dir string) (bool, error)

type WriteGates func(dir string, gates []domain.Gate) error

type Handler struct {
	detect Detect
	files  Files
	bus    shareddomain.Dispatcher
	has    HasGates
	write  WriteGates
}

func NewHandler(detect Detect, files Files, bus shareddomain.Dispatcher, has HasGates, write WriteGates) Handler {
	return Handler{detect: detect, files: files, bus: bus, has: has, write: write}
}

func (h Handler) Handle(ctx context.Context, c Command) ([]domain.Gate, error) {
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
	proposed, err := h.propose(ctx, c.Dir)
	if err != nil {
		return nil, err
	}
	gates = append(gates, proposed...)
	if err := domain.ValidateGates(gates); err != nil {
		return nil, err
	}
	if err := h.write(c.Dir, gates); err != nil {
		return nil, err
	}
	return gates, nil
}

func (h Handler) propose(ctx context.Context, dir string) ([]domain.Gate, error) {
	files, err := h.files(ctx, dir)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(shareddomain.GatesDetectPayload{Files: files})
	if err != nil {
		return nil, err
	}
	result, err := h.bus.Dispatch(ctx, shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion,
		Event:   shareddomain.EventGatesDetect,
		Cwd:     dir,
		Payload: payload,
	})
	if err != nil {
		return nil, err
	}
	failures := make([]error, len(result.Failures))
	for i, failure := range result.Failures {
		failures[i] = failure.Err
	}
	if err := errors.Join(failures...); err != nil {
		return nil, fmt.Errorf("%s: %w", shareddomain.EventGatesDetect, err)
	}
	if result.Verdict.Kind == shareddomain.VerdictDeny {
		return nil, fmt.Errorf("%s denied: %s", shareddomain.EventGatesDetect, result.Verdict.Reason)
	}
	var gates []domain.Gate
	for line := range strings.SplitSeq(result.Verdict.Context, proposalSeparator) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var proposal shareddomain.ProposedGate
		if err := json.Unmarshal([]byte(line), &proposal); err != nil {
			return nil, fmt.Errorf("%s line %q: %w", shareddomain.EventGatesDetect, line, err)
		}
		gate, err := domain.NewGate(proposal.ID, proposal.Run, proposal.Paths, 0, "")
		if err != nil {
			return nil, err
		}
		gate.Needs = proposal.Needs
		gates = append(gates, gate)
	}
	return gates, nil
}
