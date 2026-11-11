package workspace

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/recipe/domain"
)

func TestWriteModes(t *testing.T) {
	defer syscall.Umask(syscall.Umask(0o077))
	root := t.TempDir()
	space := New(root)

	if err := space.Write("lint/architecture/pack.yaml", []byte("x")); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	file, err := os.Stat(filepath.Join(root, "lint/architecture/pack.yaml"))
	if err != nil {
		t.Fatalf("Stat(file) = %v", err)
	}
	if file.Mode().Perm() != 0o644 {
		t.Errorf("file mode = %v, want 0644", file.Mode().Perm())
	}
	for _, dir := range []string{"lint", "lint/architecture"} {
		folder, err := os.Stat(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("Stat(%s) = %v", dir, err)
		}
		if folder.Mode().Perm() != 0o755 {
			t.Errorf("%s mode = %v, want 0755", dir, folder.Mode().Perm())
		}
	}
}

func TestLookup(t *testing.T) {
	root := t.TempDir()
	must(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("data"), 0o644))
	space := New(root)

	cases := map[string]domain.Kind{"nothing": domain.Missing, "dir": domain.Folder, "file": domain.File}
	for path, want := range cases {
		entry, err := space.Lookup(path)
		if err != nil || entry.Kind != want {
			t.Errorf("Lookup(%s) = %v, %v, want kind %v", path, entry.Kind, err, want)
		}
	}
	entry, _ := space.Lookup("file")
	if !bytes.Equal(entry.Data, []byte("data")) {
		t.Errorf("data = %q", entry.Data)
	}
}

func TestSymlinkIsPresentAndNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	must(t, os.WriteFile(outside, []byte("secret"), 0o644))
	must(t, os.Symlink(outside, filepath.Join(root, "link")))
	must(t, os.Symlink(filepath.Join(root, "gone"), filepath.Join(root, "dangling")))
	space := New(root)

	for _, path := range []string{"link", "dangling"} {
		entry, err := space.Lookup(path)
		if err != nil || entry.Kind != domain.Symlink || entry.Data != nil {
			t.Errorf("Lookup(%s) = %+v, %v, want an unfollowed symlink", path, entry, err)
		}
		if err := space.Write(path, []byte("new")); err == nil {
			t.Errorf("Write(%s) through a symlink succeeded", path)
		}
	}
	got, _ := os.ReadFile(outside)
	if string(got) != "secret" {
		t.Errorf("symlink target = %q, want secret", got)
	}
	if _, err := os.Stat(filepath.Join(root, "gone")); err == nil {
		t.Errorf("a dangling symlink target was created")
	}
}

func TestLookupDoesNotBlockOnAFifo(t *testing.T) {
	root := t.TempDir()
	must(t, syscall.Mkfifo(filepath.Join(root, "pipe"), 0o644))
	space := New(root)

	done := make(chan struct{})
	var entry domain.Entry
	var err error
	go func() {
		defer close(done)
		entry, err = space.Lookup("pipe")
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Lookup(pipe) blocks on a FIFO")
	}

	if err != nil || entry.Kind != domain.Symlink || entry.Data != nil {
		t.Errorf("Lookup(pipe) = %+v, %v, want a present entry that is never read", entry, err)
	}
	if err := space.Write("pipe", []byte("x")); err == nil {
		t.Error("Write(pipe) over a FIFO succeeded")
	}
}

func TestWriteRemovesTheFileOnAWriteFailure(t *testing.T) {
	defer func(old func(*os.File, []byte) error) { writeData = old }(writeData)
	errDiskFull := errors.New("disk full")
	writeData = func(*os.File, []byte) error { return errDiskFull }
	root := t.TempDir()

	if err := New(root).Write("a.txt", []byte("x")); !errors.Is(err, errDiskFull) {
		t.Fatalf("Write() = %v, want the injected write error", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "a.txt")); err == nil {
		t.Error("a failed write left the new file on disk")
	}
}

func TestWriteRemovesTheFileOnAChmodFailure(t *testing.T) {
	defer func(old func(*os.File, fs.FileMode) error) { chmod = old }(chmod)
	errChmodDenied := errors.New("chmod denied")
	chmod = func(*os.File, fs.FileMode) error { return errChmodDenied }
	root := t.TempDir()
	space := New(root)

	if err := space.Write("a.txt", []byte("x")); !errors.Is(err, errChmodDenied) {
		t.Fatalf("Write() = %v, want the injected chmod error", err)
	}
	entry, err := space.Lookup("a.txt")
	if err != nil || entry.Kind != domain.Missing {
		t.Errorf("Lookup(a.txt) = %+v, %v, want missing so the next apply writes it", entry, err)
	}
}

func TestWriteRefusesEscape(t *testing.T) {
	space := New(t.TempDir())
	if err := space.Write("../x", []byte("x")); err == nil {
		t.Errorf("Write(../x) succeeded")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
