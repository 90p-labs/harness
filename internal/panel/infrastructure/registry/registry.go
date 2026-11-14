// Package registry stores the panel repos in <dir>/repos.json.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const (
	fileMode = 0o600
	dirMode  = 0o700
	dataName = "repos.json"
	lockName = "repos.lock"
)

// File is the file adapter of domain.Registry.
type File struct {
	dir string
	log io.Writer
}

var _ domain.Registry = (*File)(nil)

// New returns a registry in dir. The log receives the drop lines.
func New(dir string, log io.Writer) *File {
	if log == nil {
		log = io.Discard
	}
	return &File{dir: dir, log: log}
}

// Register adds the repo, or refreshes its name, worktree and last use time. It
// also refreshes the state file when the new one is not empty, and keeps the
// known one otherwise.
func (f *File) Register(ctx context.Context, repo domain.Repo) error {
	return f.locked(func() error {
		repos, err := f.read()
		if err != nil {
			return err
		}
		found := false
		for i := range repos {
			if repos[i].Key == repo.Key {
				repos[i].Name = repo.Name
				repos[i].Worktree = repo.Worktree
				if repo.StateDB != "" {
					repos[i].StateDB = repo.StateDB
				}
				repos[i].UsedAt = repo.UsedAt
				found = true
			}
		}
		if !found {
			repos = append(repos, repo)
		}
		return f.write(repos)
	})
}

// List returns the live repos, newest use first. It writes the file back only on a change.
func (f *File) List(ctx context.Context) ([]domain.Repo, error) {
	var out []domain.Repo
	err := f.locked(func() error {
		repos, err := f.read()
		if err != nil {
			return err
		}
		changed := false
		out = make([]domain.Repo, 0, len(repos))
		for _, repo := range repos {
			if !exists(repo.CommonDir) || (repo.StateDB != "" && !exists(repo.StateDB)) {
				fmt.Fprintf(f.log, "panel: dropped repo %s\n", repo.Worktree)
				changed = true
				continue
			}
			if !exists(repo.Worktree) {
				main := filepath.Dir(repo.CommonDir)
				if !exists(main) {
					fmt.Fprintf(f.log, "panel: dropped repo %s\n", repo.Worktree)
					changed = true
					continue
				}
				repo.Worktree = main
				changed = true
			}
			out = append(out, repo)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].UsedAt.After(out[j].UsedAt) })
		if changed {
			return f.write(out)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// locked runs fn under an exclusive flock on repos.lock.
func (f *File) locked(fn func() error) error {
	if err := os.MkdirAll(f.dir, dirMode); err != nil {
		return fmt.Errorf("registry dir: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(f.dir, lockName), os.O_RDWR|os.O_CREATE, fileMode)
	if err != nil {
		return fmt.Errorf("registry lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("registry lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func (f *File) read() ([]domain.Repo, error) {
	path := filepath.Join(f.dir, dataName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(bytes.TrimSpace(data)) == 0) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dataName, err)
	}
	var repos []domain.Repo
	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("parse %s: %w; fix or delete that file, the panel registers repos again on their next start", path, err)
	}
	return repos, nil
}

// write replaces repos.json through a temp file and a rename.
func (f *File) write(repos []domain.Repo) error {
	if repos == nil {
		repos = []domain.Repo{}
	}
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(f.dir, dataName+".*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", dataName, err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(fileMode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(f.dir, dataName))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
