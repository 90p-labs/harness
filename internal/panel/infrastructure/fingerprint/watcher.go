// Package fingerprint polls the run versions and signals the open pages when
// one changes.
package fingerprint

import (
	"context"
	"sync"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
)

var _ domain.Signals = (*Watcher)(nil)

// group holds the subscribers of one key, and the last fingerprint read for it.
type group struct {
	fp   string
	subs map[chan string]struct{}
}

// Watcher polls the fingerprint of each key that has a subscriber. The board
// key is the empty string, and a run key is the run id.
type Watcher struct {
	runs  domain.Runs
	every time.Duration

	mu     sync.Mutex
	groups map[string]*group
}

// NewWatcher returns a watcher that ticks every interval once Run starts.
func NewWatcher(runs domain.Runs, every time.Duration) *Watcher {
	return &Watcher{runs: runs, every: every, groups: map[string]*group{}}
}

// Run ticks until ctx ends. A tick with no subscriber reads nothing.
func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTicker(w.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// Subscribe records the current fingerprint of the key, so the first tick
// signals only a real change. An unknown run returns the Runs error.
func (w *Watcher) Subscribe(ctx context.Context, run string) (<-chan string, func(), error) {
	ch := make(chan string, 1)
	// The check of the group and its create or join share one lock, so a cancel
	// cannot remove the group between them. A group is created only with a
	// fingerprint read after the check found it missing.
	fp, read := "", false
	for {
		w.mu.Lock()
		g := w.groups[run]
		if g == nil && read {
			g = &group{fp: fp, subs: map[chan string]struct{}{}}
			w.groups[run] = g
		}
		if g != nil {
			g.subs[ch] = struct{}{}
			w.mu.Unlock()
			break
		}
		w.mu.Unlock()

		var err error
		if fp, err = w.read(ctx, run); err != nil {
			return nil, nil, err
		}
		read = true
	}

	cancel := func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		cur := w.groups[run]
		if cur == nil {
			return
		}
		if _, still := cur.subs[ch]; !still {
			return
		}
		delete(cur.subs, ch)
		close(ch)
		if len(cur.subs) == 0 {
			delete(w.groups, run)
		}
	}
	return ch, cancel, nil
}

// tick reads each key with a subscriber once, and signals its subscribers when
// the fingerprint changed. A send on a full buffer is dropped.
func (w *Watcher) tick(ctx context.Context) {
	w.mu.Lock()
	keys := make([]string, 0, len(w.groups))
	for k := range w.groups {
		keys = append(keys, k)
	}
	w.mu.Unlock()

	for _, key := range keys {
		fp, err := w.read(ctx, key)
		if err != nil {
			continue
		}
		w.mu.Lock()
		if g := w.groups[key]; g != nil && g.fp != fp {
			g.fp = fp
			name := domain.SignalRunUpdated
			if key == "" {
				name = domain.SignalRunsChanged
			}
			for ch := range g.subs {
				select {
				case ch <- name:
				default:
				}
			}
		}
		w.mu.Unlock()
	}
}

func (w *Watcher) read(ctx context.Context, key string) (string, error) {
	if key == "" {
		cards, err := w.runs.List(ctx)
		if err != nil {
			return "", err
		}
		return domain.NewBoard(cards).Version, nil
	}
	card, err := w.runs.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return card.Version.String(), nil
}
