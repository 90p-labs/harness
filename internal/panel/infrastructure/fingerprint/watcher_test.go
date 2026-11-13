package fingerprint

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const tick = 10 * time.Millisecond

// fakeRuns is a Runs whose cards a test changes, and whose calls it counts.
type fakeRuns struct {
	mu    sync.Mutex
	cards map[string]domain.Card
	calls int
}

func newFakeRuns(ids ...string) *fakeRuns {
	f := &fakeRuns{cards: map[string]domain.Card{}}
	for _, id := range ids {
		f.cards[id] = domain.Card{ID: id, Phase: "planning", Version: domain.Version{Plan: "p1", Run: "r1"}}
	}
	return f
}

func (f *fakeRuns) setRun(id, run string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.cards[id]
	c.Version.Run = run
	f.cards[id] = c
}

func (f *fakeRuns) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeRuns) List(context.Context) ([]domain.Card, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	out := make([]domain.Card, 0, len(f.cards))
	for _, c := range f.cards {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeRuns) Get(_ context.Context, id string) (domain.Card, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	c, ok := f.cards[id]
	if !ok {
		return domain.Card{}, domain.ErrNotFound
	}
	return c, nil
}

func start(t *testing.T, w *Watcher) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func expect(t *testing.T, ch <-chan string, want string) {
	t.Helper()
	select {
	case got, ok := <-ch:
		if !ok || got != want {
			t.Fatalf("got %q open=%v, want %q", got, ok, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no signal, want %q", want)
	}
}

func TestWatcher(t *testing.T) {
	t.Run("a change signals a run subscriber", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		start(t, w)
		ch, cancel, err := w.Subscribe(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		runs.setRun("a", "r2")
		expect(t, ch, domain.SignalRunUpdated)
	})

	t.Run("a change signals a board subscriber", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		start(t, w)
		ch, cancel, err := w.Subscribe(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		runs.setRun("a", "r2")
		expect(t, ch, domain.SignalRunsChanged)
	})

	t.Run("no change sends no signal", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		start(t, w)
		ch, cancel, err := w.Subscribe(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		select {
		case got := <-ch:
			t.Fatalf("unexpected signal %q", got)
		case <-time.After(15 * tick):
		}
	})

	t.Run("no subscriber means no Runs call", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		start(t, w)
		time.Sleep(10 * tick)
		if n := runs.count(); n != 0 {
			t.Fatalf("Runs called %d times with no subscriber", n)
		}
		_, cancel, err := w.Subscribe(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		before := runs.count()
		time.Sleep(10 * tick)
		if n := runs.count(); n != before {
			t.Fatalf("Runs called %d times after the last cancel", n-before)
		}
	})

	t.Run("a slow subscriber does not block another", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		start(t, w)
		_, cancelSlow, err := w.Subscribe(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		defer cancelSlow()
		fast, cancelFast, err := w.Subscribe(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		defer cancelFast()
		for i := 2; i < 6; i++ {
			runs.setRun("a", string(rune('0'+i)))
			expect(t, fast, domain.SignalRunUpdated)
		}
	})

	t.Run("cancel closes the channel once", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		ch, cancel, err := w.Subscribe(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		cancel()
		select {
		case _, ok := <-ch:
			if ok {
				t.Fatal("channel delivered a value after cancel")
			}
		case <-time.After(time.Second):
			t.Fatal("channel not closed")
		}
	})

	t.Run("Run returns when the context ends", func(t *testing.T) {
		w := NewWatcher(newFakeRuns("a"), tick)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { w.Run(ctx); close(done) }()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return")
		}
	})

	t.Run("an unknown run gives ErrNotFound", func(t *testing.T) {
		w := NewWatcher(newFakeRuns("a"), tick)
		ch, cancel, err := w.Subscribe(context.Background(), "nope")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if ch != nil || cancel != nil {
			t.Fatal("an error must return no channel and no cancel")
		}
	})

	t.Run("one tick reads each key once", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, time.Hour)
		for range 3 {
			_, cancel, err := w.Subscribe(context.Background(), "a")
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
		}
		before := runs.count()
		w.tick(context.Background())
		if n := runs.count() - before; n != 1 {
			t.Fatalf("one tick made %d reads for one key", n)
		}
	})
}
