// Package process runs the panel server in the background. It holds the panel
// lock, the state files and the spawn, stop and status of the server.
package process

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// The names of the files in a panel state folder.
const (
	StateFile = "panel.json"
	LockFile  = "panel.lock"
	LogFile   = "panel.log"
)

const (
	fileMode = 0o600
	dirMode  = 0o700
)

// ErrClaimed marks a panel lock that another process holds.
var ErrClaimed = errors.New("panel already runs")

// state is the content of panel.json. It carries no URL: the URL is built
// from the port. It carries no repo: one panel serves every repo.
type state struct {
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	Version string `json:"version"`
}

// Claim holds the panel lock of one server.
type Claim struct {
	dir  string
	file *os.File
	once sync.Once
	err  error
}

// Claim takes the panel lock, retrying for ClaimWait since a status probe may
// hold it briefly. It fails with ErrClaimed when the lock stays held.
func (p *Process) Claim() (*Claim, error) {
	if err := os.MkdirAll(p.dir, dirMode); err != nil {
		return nil, fmt.Errorf("create panel state dir: %w", err)
	}
	deadline := time.Now().Add(p.ClaimWait)
	for {
		f, err := tryLock(filepath.Join(p.dir, LockFile))
		if err == nil {
			return &Claim{dir: p.dir, file: f}, nil
		}
		if !errors.Is(err, ErrClaimed) {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, err
		}
		time.Sleep(p.Poll)
	}
}

// Publish writes panel.json through a temp file and a rename.
func (c *Claim) Publish(inst domain.Instance) error {
	body, err := json.Marshal(state{PID: inst.PID, Port: inst.Port, Version: inst.Version})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.dir, StateFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("create panel.json temp file: %w", err)
	}
	cleanup := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(fileMode); err != nil {
		return cleanup(err)
	}
	if _, err := tmp.Write(body); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		return cleanup(err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(c.dir, StateFile)); err != nil {
		return cleanup(err)
	}
	return nil
}

// Release removes panel.json, then unlocks. It is safe to call twice.
func (c *Claim) Release() error {
	c.once.Do(func() {
		if err := os.Remove(filepath.Join(c.dir, StateFile)); err != nil && !os.IsNotExist(err) {
			c.err = err
		}
		// closing the file drops the flock too
		_ = syscall.Flock(int(c.file.Fd()), syscall.LOCK_UN)
		if err := c.file.Close(); err != nil && c.err == nil {
			c.err = err
		}
	})
	return c.err
}

// tryLock opens the lock file and takes LOCK_EX|LOCK_NB once. A held lock
// returns ErrClaimed.
func tryLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, fmt.Errorf("open panel lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrClaimed
		}
		return nil, fmt.Errorf("lock panel: %w", err)
	}
	return f, nil
}

// lockFree reports whether the lock is free. It releases a free lock at once.
func lockFree(dir string) (bool, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return true, nil
	}
	f, err := tryLock(filepath.Join(dir, LockFile))
	if errors.Is(err, ErrClaimed) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
	return true, nil
}
