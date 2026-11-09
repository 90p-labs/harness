package start

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/domain"
)

var (
	now = time.Date(2026, 11, 8, 1, 18, 5, 0, time.UTC)
	key = domain.RunKey{Repo: "github.com/90p-labs/harness", Worktree: "/work/harness"}
)

const prompt = "Add the login page"

type fakeRepository struct {
	active  domain.Run
	found   bool
	saved   []domain.Run
	saveErr error
}

func (f *fakeRepository) Active(context.Context, domain.RunKey) (domain.Run, error) {
	if f.found {
		return f.active, nil
	}
	return domain.Run{}, domain.ErrRunNotFound
}

func (f *fakeRepository) Get(context.Context, string) (domain.Run, error) {
	return domain.Run{}, domain.ErrRunNotFound
}

func (f *fakeRepository) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	return nil, nil
}

func (f *fakeRepository) Save(_ context.Context, run domain.Run) (domain.Run, error) {
	if f.saveErr != nil {
		return domain.Run{}, f.saveErr
	}
	f.saved = append(f.saved, run)
	return run.Persisted(), nil
}

type fakeHooks struct {
	veto       error
	beforeRuns []domain.Run
	afterTo    []domain.Run
}

func (f *fakeHooks) Before(_ context.Context, run domain.Run, _ domain.Command, _ domain.Phase) error {
	f.beforeRuns = append(f.beforeRuns, run)
	return f.veto
}

func (f *fakeHooks) After(_ context.Context, _ domain.Run, to domain.Run) {
	f.afterTo = append(f.afterTo, to)
}

type noTrace struct{}

func (noTrace) Under(ctx context.Context, _ domain.Run) context.Context { return ctx }
func (noTrace) End(context.Context, domain.Run)                         {}

func newHandler(repo *fakeRepository, hooks *fakeHooks) Handler {
	clock := func() time.Time { return now }
	flow := transition.NewFlow(repo, hooks, nil, nil, domain.Policy{}, clock, noop.NewTracerProvider().Tracer("test"), noTrace{})
	return NewHandler(repo, flow, clock)
}

func TestNewRunID(t *testing.T) {
	cases := []struct {
		name, prompt, want string
	}{
		{"words", "Add the login page", "20261108-011805-add-the-login-page"},
		{"collapses and trims symbols", "  --Fix: the (bug)!!  ", "20261108-011805-fix-the-bug"},
		{"keeps digits", "Bump go to 1.27", "20261108-011805-bump-go-to-1-27"},
		{"no letter or digit", "?!. ...", "20261108-011805"},
		{"empty", "", "20261108-011805"},
		{"cut at 40", strings.Repeat("a", 60), "20261108-011805-" + strings.Repeat("a", 40)},
		{"no trailing dash after the cut", strings.Repeat("a", 39) + " bbb", "20261108-011805-" + strings.Repeat("a", 39)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newRunID(now, tc.prompt); got != tc.want {
				t.Errorf("newRunID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStartNewRun(t *testing.T) {
	repo, hooks := &fakeRepository{}, &fakeHooks{}

	if err := newHandler(repo, hooks).Handle(context.Background(), Command{Key: key, Prompt: prompt}); err != nil {
		t.Fatal(err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("saved = %d, want 1", len(repo.saved))
	}
	got := repo.saved[0]
	if got.ID != "20261108-011805-add-the-login-page" || got.Key != key || got.Prompt != prompt || got.Phase != domain.PhasePlanning {
		t.Errorf("saved = %+v", got)
	}
	if len(hooks.afterTo) != 1 {
		t.Errorf("After calls = %d, want 1", len(hooks.afterTo))
	}
}

func TestStartWithActiveRun(t *testing.T) {
	t.Run("an open run stops the start", func(t *testing.T) {
		repo, hooks := &fakeRepository{found: true, active: domain.Run{ID: "open", Phase: domain.PhaseWorking}}, &fakeHooks{}

		err := newHandler(repo, hooks).Handle(context.Background(), Command{Key: key, Prompt: prompt})

		if !errors.Is(err, domain.ErrActiveRunExists) {
			t.Fatalf("err = %v", err)
		}
		if len(repo.saved) != 0 || len(hooks.beforeRuns) != 0 {
			t.Errorf("saved %d, Before calls %d, want none", len(repo.saved), len(hooks.beforeRuns))
		}
	})

	t.Run("a veto stops a new run", func(t *testing.T) {
		repo, hooks := &fakeRepository{}, &fakeHooks{veto: domain.ErrTransitionVetoed}

		err := newHandler(repo, hooks).Handle(context.Background(), Command{Key: key, Prompt: prompt})

		if !errors.Is(err, domain.ErrTransitionVetoed) {
			t.Fatalf("err = %v", err)
		}
		if len(repo.saved) != 0 || len(hooks.afterTo) != 0 {
			t.Errorf("saved %d, After calls %d, want none", len(repo.saved), len(hooks.afterTo))
		}
		if len(hooks.beforeRuns) != 1 || hooks.beforeRuns[0].Phase != domain.PhaseNone {
			t.Errorf("Before runs = %+v, want one run with an empty phase", hooks.beforeRuns)
		}
	})

	t.Run("a lookup failure other than not found goes back", func(t *testing.T) {
		boom := errors.New("disk full")
		repo := &failingRepository{err: boom}

		err := NewHandler(repo, transition.Flow{}, func() time.Time { return now }).Handle(context.Background(), Command{Key: key, Prompt: prompt})

		if !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}

type failingRepository struct {
	fakeRepository
	err error
}

func (f *failingRepository) Active(context.Context, domain.RunKey) (domain.Run, error) {
	return domain.Run{}, f.err
}

func TestStartRecordsTheStartTime(t *testing.T) {
	repo := &fakeRepository{}
	local := time.FixedZone("plus2", 2*60*60)
	clock := func() time.Time { return now.In(local) }
	flow := transition.NewFlow(repo, &fakeHooks{}, nil, nil, domain.Policy{}, clock, noop.NewTracerProvider().Tracer("test"), noTrace{})

	if err := NewHandler(repo, flow, clock).Handle(context.Background(), Command{Key: key, Prompt: prompt}); err != nil {
		t.Fatal(err)
	}

	got := repo.saved[0].StartedAt
	if !got.Equal(now) || got.Location() != time.UTC {
		t.Fatalf("StartedAt = %v, want %v in UTC", got, now)
	}
}
