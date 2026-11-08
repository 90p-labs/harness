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

const (
	proposalSeparator = "\n"
	blockedFormat     = "%w: the %s key is set, so edit it by hand, or remove the %s key"
)

type Command struct {
	Dir string
}

type Result struct {
	Setup []domain.Setup
	Gates []domain.Gate
}

type Detect func(dir string) ([]domain.Gate, error)

type Files func(ctx context.Context, dir string) ([]string, error)

type BlockingKey func(dir string) (string, error)

type WriteGates func(dir string, setup []domain.Setup, gates []domain.Gate) error

type Handler struct {
	detect      Detect
	files       Files
	bus         shareddomain.Dispatcher
	blockingKey BlockingKey
	write       WriteGates
}

func NewHandler(detect Detect, files Files, bus shareddomain.Dispatcher, blockingKey BlockingKey, write WriteGates) Handler {
	return Handler{detect: detect, files: files, bus: bus, blockingKey: blockingKey, write: write}
}

func (h Handler) Handle(ctx context.Context, c Command) (Result, error) {
	key, err := h.blockingKey(c.Dir)
	if err != nil {
		return Result{}, err
	}
	if key != "" {
		return Result{}, fmt.Errorf(blockedFormat, domain.ErrGatesExist, key, key)
	}
	gates, err := h.detect(c.Dir)
	if err != nil {
		return Result{}, err
	}
	proposed, err := h.propose(ctx, c.Dir)
	if err != nil {
		return Result{}, err
	}
	proposed.Gates = append(gates, proposed.Gates...)
	if err := domain.ValidateSetup(proposed.Setup); err != nil {
		return Result{}, err
	}
	if err := domain.ValidateGates(proposed.Gates); err != nil {
		return Result{}, err
	}
	if err := h.write(c.Dir, proposed.Setup, proposed.Gates); err != nil {
		return Result{}, err
	}
	return proposed, nil
}

func (h Handler) propose(ctx context.Context, dir string) (Result, error) {
	files, err := h.files(ctx, dir)
	if err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(shareddomain.GatesDetectPayload{Files: files})
	if err != nil {
		return Result{}, err
	}
	result, err := h.bus.Dispatch(ctx, shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion,
		Event:   shareddomain.EventGatesDetect,
		Cwd:     dir,
		Payload: payload,
	})
	if err != nil {
		return Result{}, err
	}
	failures := make([]error, len(result.Failures))
	for i, failure := range result.Failures {
		failures[i] = failure.Err
	}
	if err := errors.Join(failures...); err != nil {
		return Result{}, fmt.Errorf("%s: %w", shareddomain.EventGatesDetect, err)
	}
	if result.Verdict.Kind == shareddomain.VerdictDeny {
		return Result{}, fmt.Errorf("%s denied: %s", shareddomain.EventGatesDetect, result.Verdict.Reason)
	}
	var proposed Result
	for line := range strings.SplitSeq(result.Verdict.Context, proposalSeparator) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var proposal shareddomain.ProposedGate
		if err := json.Unmarshal([]byte(line), &proposal); err != nil {
			return Result{}, fmt.Errorf("%s line %q: %w", shareddomain.EventGatesDetect, line, err)
		}
		if proposal.Setup {
			step, err := proposeSetup(proposal)
			if err != nil {
				return Result{}, err
			}
			proposed.Setup = append(proposed.Setup, step)
			continue
		}
		if proposal.Creates != "" {
			return Result{}, fmt.Errorf("%w: gate %q: creates is only for a setup step", domain.ErrInvalidGate, proposal.ID)
		}
		gate, err := domain.NewGate(proposal.ID, proposal.Run, proposal.Paths, 0, "")
		if err != nil {
			return Result{}, err
		}
		gate.Needs = proposal.Needs
		proposed.Gates = append(proposed.Gates, gate)
	}
	return proposed, nil
}

func proposeSetup(proposal shareddomain.ProposedGate) (domain.Setup, error) {
	if len(proposal.Needs) > 0 {
		return domain.Setup{}, fmt.Errorf("%w: setup step %q: a setup step has no needs", domain.ErrInvalidSetup, proposal.ID)
	}
	return domain.NewSetup(proposal.ID, proposal.Run, proposal.Paths, proposal.Creates)
}
