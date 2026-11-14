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

const (
	repoA = "aaaaaaaaaaaa"
	repoB = "bbbbbbbbbbbb"
)

// ref names a run of the first repo.
func ref(id string) domain.RunRef { return domain.RunRef{Repo: repoA, ID: id} }

// fakeRuns is a Runs whose cards a test changes, and whose calls it counts.
type fakeRuns struct {
	mu    sync.Mutex
	cards map[domain.RunRef]domain.Card
	calls int
}

// newFakeRuns holds one card of the first repo for each id.
func newFakeRuns(ids ...string) *fakeRuns {
	f := &fakeRuns{cards: map[domain.RunRef]domain.Card{}}
	for _, id := range ids {
		f.add(ref(id))
	}
	return f
}

func (f *fakeRuns) add(r domain.RunRef) {
	f.cards[r] = domain.Card{Repo: r.Repo, ID: r.ID, Phase: "planning", Version: domain.Version{Plan: "p1", Run: "r1"}}
}

func (f *fakeRuns) setRun(id, run string) { f.setRunOf(ref(id), run) }

func (f *fakeRuns) setRunOf(r domain.RunRef, run string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.cards[r]
	c.Version.Run = run
	f.cards[r] = c
}

func (f *fakeRuns) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeRuns) List(context.Context) (domain.Listing, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	l := domain.Listing{}
	seen := map[string]bool{}
	for r, c := range f.cards {
		l.Cards = append(l.Cards, c)
		if !seen[r.Repo] {
			seen[r.Repo] = true
			l.Repos = append(l.Repos, domain.Repo{Key: r.Repo, Name: r.Repo})
		}
	}
	return l, nil
}

func (f *fakeRuns) Get(_ context.Context, r domain.RunRef) (domain.Card, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	c, ok := f.cards[r]
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

func TestWatcherByRepo(t *testing.T) {
	const id = "20261113-same"
	a, b := domain.RunRef{Repo: repoA, ID: id}, domain.RunRef{Repo: repoB, ID: id}
	quiet := func(t *testing.T, ch <-chan string) {
		t.Helper()
		select {
		case got := <-ch:
			t.Fatalf("unexpected signal %q", got)
		case <-time.After(15 * tick):
		}
	}

	t.Run("one id in two repos is two keys", func(t *testing.T) {
		runs := newFakeRuns()
		runs.add(a)
		runs.add(b)
		w := NewWatcher(runs, tick)
		start(t, w)
		chA, cancelA, err := w.Subscribe(context.Background(), a)
		if err != nil {
			t.Fatal(err)
		}
		defer cancelA()
		chB, cancelB, err := w.Subscribe(context.Background(), b)
		if err != nil {
			t.Fatal(err)
		}
		defer cancelB()

		runs.setRunOf(b, "r2")
		expect(t, chB, domain.SignalRunUpdated)
		quiet(t, chA)

		runs.setRunOf(a, "r2")
		expect(t, chA, domain.SignalRunUpdated)
	})

	t.Run("the board signals a change in any repo", func(t *testing.T) {
		runs := newFakeRuns()
		runs.add(a)
		runs.add(b)
		w := NewWatcher(runs, tick)
		start(t, w)
		ch, cancel, err := w.Subscribe(context.Background(), domain.RunRef{})
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		runs.setRunOf(b, "r2")
		expect(t, ch, domain.SignalRunsChanged)
	})

	t.Run("a run of another repo is unknown", func(t *testing.T) {
		runs := newFakeRuns()
		runs.add(a)
		w := NewWatcher(runs, tick)
		if _, _, err := w.Subscribe(context.Background(), b); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestWatcher(t *testing.T) {
	t.Run("a change signals a run subscriber", func(t *testing.T) {
		runs := newFakeRuns("a")
		w := NewWatcher(runs, tick)
		start(t, w)
		ch, cancel, err := w.Subscribe(context.Background(), ref("a"))
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
		ch, cancel, err := w.Subscribe(context.Background(), domain.RunRef{})
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
		ch, cancel, err := w.Subscribe(context.Background(), ref("a"))
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
		_, cancel, err := w.Subscribe(context.Background(), ref("a"))
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
		_, cancelSlow, err := w.Subscribe(context.Background(), ref("a"))
		if err != nil {
			t.Fatal(err)
		}
		defer cancelSlow()
		fast, cancelFast, err := w.Subscribe(context.Background(), ref("a"))
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
		ch, cancel, err := w.Subscribe(context.Background(), ref("a"))
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
		ch, cancel, err := w.Subscribe(context.Background(), ref("nope"))
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
			_, cancel, err := w.Subscribe(context.Background(), ref("a"))
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
