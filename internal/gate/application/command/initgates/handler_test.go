package initgates_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/gate/application/command/initgates"
	"github.com/jorge07/harness/internal/gate/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	repoDir    = "/work/repo"
	helmLine   = `{"id":"helm-render","run":"helm template deploy/chart > /dev/null"}`
	kubeLine   = `{"id":"kubeconform","run":"kubeconform deploy","paths":["deploy/**"]}`
	goTestLine = `{"id":"go-test","run":"go test ./..."}`
	keyGates   = "gates"
	keySetup   = "setup"
)

var (
	boom      = errors.New("boom")
	repoFiles = []string{"go.mod", "deploy/chart/Chart.yaml"}
)

type fakeStore struct {
	key     string
	written []domain.Gate
	setup   []domain.Setup
	dir     string
}

func (f *fakeStore) blockingKey(string) (string, error) { return f.key, nil }

func (f *fakeStore) write(dir string, setup []domain.Setup, gates []domain.Gate) error {
	f.dir, f.setup, f.written = dir, setup, gates
	return nil
}

type fakeBus struct {
	result shareddomain.DispatchResult
	err    error
	got    shareddomain.Envelope
}

func (b *fakeBus) Dispatch(_ context.Context, envelope shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	b.got = envelope
	return b.result, b.err
}

func answer(context string) *fakeBus {
	return &fakeBus{result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: context}}}
}

func detected(string) ([]domain.Gate, error) {
	return []domain.Gate{{ID: "go-test", Run: "go test ./...", Timeout: domain.DefaultTimeout}}, nil
}

func listFiles(context.Context, string) ([]string, error) { return repoFiles, nil }

func handler(store *fakeStore, bus shareddomain.Dispatcher) initgates.Handler {
	return initgates.NewHandler(detected, listFiles, bus, store.blockingKey, store.write)
}

func newGate(t *testing.T, id, run string, paths []string) domain.Gate {
	t.Helper()
	gate, err := domain.NewGate(id, run, paths, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	return gate
}

func TestInitGatesAppendsTheProposedGates(t *testing.T) {
	store := &fakeStore{}
	bus := answer(helmLine + "\n\n" + kubeLine + "\n")

	got, err := handler(store, bus).Handle(context.Background(), initgates.Command{Dir: repoDir})

	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	toolchain, _ := detected(repoDir)
	want := append(toolchain,
		newGate(t, "helm-render", "helm template deploy/chart > /dev/null", nil),
		newGate(t, "kubeconform", "kubeconform deploy", []string{"deploy/**"}),
	)
	if !reflect.DeepEqual(got.Gates, want) || !reflect.DeepEqual(store.written, want) || store.dir != repoDir {
		t.Errorf("got %+v, written %+v in %q, want %+v in %q", got, store.written, store.dir, want, repoDir)
	}
	var payload shareddomain.GatesDetectPayload
	if err := json.Unmarshal(bus.got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if bus.got.Event != shareddomain.EventGatesDetect || bus.got.Cwd != repoDir || !reflect.DeepEqual(payload.Files, repoFiles) {
		t.Errorf("envelope = %+v, payload = %+v", bus.got, payload)
	}
}

func TestHandleProposedNeeds(t *testing.T) {
	store := &fakeStore{}
	bus := answer(`{"id":"cs","run":"make cs"}` + "\n" + `{"id":"unit","run":"make unit","needs":["cs","go-test"]}`)

	_, err := handler(store, bus).Handle(context.Background(), initgates.Command{Dir: repoDir})

	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	cs := newGate(t, "cs", "make cs", nil)
	unit := newGate(t, "unit", "make unit", nil)
	unit.Needs = []string{"cs", "go-test"}
	toolchain, _ := detected(repoDir)
	if want := append(toolchain, cs, unit); !reflect.DeepEqual(store.written, want) {
		t.Errorf("written %+v, want %+v", store.written, want)
	}

	store = &fakeStore{}
	_, err = handler(store, answer(`{"id":"unit","run":"make unit","needs":["missing"]}`)).Handle(context.Background(), initgates.Command{Dir: repoDir})
	if !errors.Is(err, domain.ErrInvalidGate) || store.written != nil {
		t.Errorf("err = %v, written = %+v, want an unknown need refused before the write", err, store.written)
	}
}

func TestHandleProposedSetup(t *testing.T) {
	store := &fakeStore{}
	bus := answer(`{"id":"install","run":"make install","paths":["composer.lock"],"creates":"vendor/autoload.php","setup":true}` + "\n" + `{"id":"cs","run":"make cs"}`)

	got, err := handler(store, bus).Handle(context.Background(), initgates.Command{Dir: repoDir})

	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	install, _ := domain.NewSetup("install", "make install", []string{"composer.lock"}, "vendor/autoload.php")
	toolchain, _ := detected(repoDir)
	wantGates := append(toolchain, newGate(t, "cs", "make cs", nil))
	if !reflect.DeepEqual(store.setup, []domain.Setup{install}) || !reflect.DeepEqual(got.Setup, store.setup) || !reflect.DeepEqual(store.written, wantGates) {
		t.Errorf("setup = %+v, gates = %+v, want %+v and %+v", store.setup, store.written, install, wantGates)
	}

	for name, line := range map[string]string{
		"setup with needs":  `{"id":"install","run":"make install","setup":true,"needs":["cs"]}`,
		"gate with creates": `{"id":"cs","run":"make cs","creates":"out"}`,
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			_, err := handler(store, answer(line)).Handle(context.Background(), initgates.Command{Dir: repoDir})
			if err == nil || store.written != nil {
				t.Errorf("err = %v, written = %+v, want the line refused before the write", err, store.written)
			}
		})
	}
}

func TestInitGatesRefusesOverwrite(t *testing.T) {
	store := &fakeStore{key: keyGates}
	_, err := handler(store, answer("")).Handle(context.Background(), initgates.Command{Dir: repoDir})
	if !errors.Is(err, domain.ErrGatesExist) {
		t.Fatalf("err = %v, want ErrGatesExist", err)
	}
	if store.written != nil {
		t.Errorf("wrote %+v over existing gates", store.written)
	}
}

func TestBlockingKeyNamedInError(t *testing.T) {
	store := &fakeStore{key: keySetup}
	_, err := handler(store, answer("")).Handle(context.Background(), initgates.Command{Dir: repoDir})
	if !errors.Is(err, domain.ErrGatesExist) || !strings.Contains(err.Error(), keySetup) {
		t.Fatalf("err = %v, want ErrGatesExist naming the setup key", err)
	}
}

func TestInitGatesProposalFailures(t *testing.T) {
	tests := map[string]struct {
		result shareddomain.DispatchResult
		want   error
	}{
		"a plugin fails": {
			result: shareddomain.DispatchResult{Failures: []shareddomain.DispatchFailure{{Plugin: "kubeconform", Err: boom}}},
			want:   boom,
		},
		"a plugin denies": {result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "kubeconform: no schema"}}},
		"a bad line":      {result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: "helm-render"}}},
		"a gate with no run": {
			result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: `{"id":"kubeconform"}`}},
			want:   domain.ErrInvalidGate,
		},
		"a duplicate id": {
			result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: goTestLine}},
			want:   domain.ErrInvalidGate,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			_, err := handler(store, &fakeBus{result: tt.result}).Handle(context.Background(), initgates.Command{Dir: repoDir})
			if err == nil || (tt.want != nil && !errors.Is(err, tt.want)) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if store.written != nil {
				t.Errorf("wrote %+v after a failed proposal", store.written)
			}
		})
	}
}

func TestInitGatesPortFailures(t *testing.T) {
	ok := func(string, []domain.Setup, []domain.Gate) error { return nil }
	none := func(string) (string, error) { return "", nil }
	tests := map[string]initgates.Handler{
		"detect": initgates.NewHandler(func(string) ([]domain.Gate, error) { return nil, boom }, listFiles, answer(""), none, ok),
		"files":  initgates.NewHandler(detected, func(context.Context, string) ([]string, error) { return nil, boom }, answer(""), none, ok),
		"bus":    initgates.NewHandler(detected, listFiles, &fakeBus{err: boom}, none, ok),
		"has":    initgates.NewHandler(detected, listFiles, answer(""), func(string) (string, error) { return "", boom }, ok),
		"write":  initgates.NewHandler(detected, listFiles, answer(""), none, func(string, []domain.Setup, []domain.Gate) error { return boom }),
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := h.Handle(context.Background(), initgates.Command{Dir: repoDir}); !errors.Is(err, boom) {
				t.Errorf("err = %v, want boom", err)
			}
		})
	}
}
