package initgates_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jorge07/harness/internal/gate/application/command/initgates"
	"github.com/jorge07/harness/internal/gate/domain"
)

const repoDir = "/work/repo"

type fakeStore struct {
	exists  bool
	written []domain.Gate
	dir     string
}

func (f *fakeStore) has(string) (bool, error) { return f.exists, nil }

func (f *fakeStore) write(dir string, gates []domain.Gate) error {
	f.dir, f.written = dir, gates
	return nil
}

func detected() ([]domain.Gate, error) {
	return []domain.Gate{{ID: "go-test", Run: "go test ./...", Timeout: domain.DefaultTimeout}}, nil
}

func handler(store *fakeStore) initgates.Handler {
	return initgates.NewHandler(func(string) ([]domain.Gate, error) { return detected() }, store.has, store.write)
}

func TestInitGatesRefusesOverwrite(t *testing.T) {
	t.Run("writes when the key is absent", func(t *testing.T) {
		store := &fakeStore{}
		got, err := handler(store).Handle(context.Background(), initgates.Command{Dir: repoDir})
		if err != nil {
			t.Fatalf("handle: %v", err)
		}
		want, _ := detected()
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(store.written, want) || store.dir != repoDir {
			t.Errorf("got %+v, written %+v in %q, want %+v in %q", got, store.written, store.dir, want, repoDir)
		}
	})

	t.Run("refuses when the key exists", func(t *testing.T) {
		store := &fakeStore{exists: true}
		_, err := handler(store).Handle(context.Background(), initgates.Command{Dir: repoDir})
		if !errors.Is(err, domain.ErrGatesExist) {
			t.Fatalf("err = %v, want ErrGatesExist", err)
		}
		if store.written != nil {
			t.Errorf("wrote %+v over existing gates", store.written)
		}
	})
}

func TestInitGatesFailures(t *testing.T) {
	boom := errors.New("boom")
	h := initgates.NewHandler(
		func(string) ([]domain.Gate, error) { return nil, boom },
		func(string) (bool, error) { return false, nil },
		func(string, []domain.Gate) error { return nil },
	)
	if _, err := h.Handle(context.Background(), initgates.Command{Dir: repoDir}); !errors.Is(err, boom) {
		t.Errorf("detect failure: err = %v", err)
	}
	h = initgates.NewHandler(
		func(string) ([]domain.Gate, error) { return detected() },
		func(string) (bool, error) { return false, boom },
		func(string, []domain.Gate) error { return nil },
	)
	if _, err := h.Handle(context.Background(), initgates.Command{Dir: repoDir}); !errors.Is(err, boom) {
		t.Errorf("has failure: err = %v", err)
	}
	h = initgates.NewHandler(
		func(string) ([]domain.Gate, error) { return detected() },
		func(string) (bool, error) { return false, nil },
		func(string, []domain.Gate) error { return boom },
	)
	if _, err := h.Handle(context.Background(), initgates.Command{Dir: repoDir}); !errors.Is(err, boom) {
		t.Errorf("write failure: err = %v", err)
	}
}
