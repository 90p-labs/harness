package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/90p-labs/harness/internal/recipe/domain"
)

const (
	fileMode   fs.FileMode = 0o644
	folderMode fs.FileMode = 0o755
)

var errOutside = errors.New("path is outside the target folder")

// writeData and chmod are seams, so a test can force a failure.
var (
	writeData = func(file *os.File, data []byte) error { _, err := file.Write(data); return err }
	chmod     = func(file *os.File, mode fs.FileMode) error { return file.Chmod(mode) }
)

// Workspace is a target folder on disk.
type Workspace struct {
	root string
}

var _ domain.Workspace = (*Workspace)(nil)

func New(root string) *Workspace {
	return &Workspace{root: root}
}

// Open is a domain.Opener over the disk.
func Open(target string) domain.Workspace {
	return New(target)
}

// Lookup uses Lstat, so a symlink counts as present and is never followed.
// It reads only a regular file; a FIFO or a socket is reported as Symlink.
func (w *Workspace) Lookup(path string) (domain.Entry, error) {
	full, err := w.resolve(path)
	if err != nil {
		return domain.Entry{}, err
	}
	info, err := os.Lstat(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return domain.Entry{Kind: domain.Missing}, nil
	case err != nil:
		return domain.Entry{}, err
	case info.IsDir():
		return domain.Entry{Kind: domain.Folder}, nil
	case !info.Mode().IsRegular():
		return domain.Entry{Kind: domain.Symlink}, nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return domain.Entry{}, err
	}
	return domain.Entry{Kind: domain.File, Data: data}, nil
}

// Write creates the file and its folders. It fails when the path exists,
// so it never overwrites a file or writes through a symlink.
func (w *Workspace) Write(path string, data []byte) error {
	full, err := w.resolve(path)
	if err != nil {
		return err
	}
	if err := w.mkdirs(filepath.Dir(full)); err != nil {
		return err
	}
	file, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return err
	}
	// The umask can clear bits at create, so set the mode again. It acts on the
	// open file, not the path, so a swapped symlink is never followed.
	err = chmod(file, fileMode)
	if err == nil {
		err = writeData(file, data)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// The file did not exist before, so no user data is lost.
		_ = os.Remove(full)
		return err
	}
	return nil
}

// mkdirs creates each missing folder under the root with mode 0755, whatever the umask.
func (w *Workspace) mkdirs(dir string) error {
	if dir == w.root {
		return nil
	}
	if _, err := os.Lstat(dir); err == nil {
		return nil
	}
	if err := w.mkdirs(filepath.Dir(dir)); err != nil {
		return err
	}
	if err := os.Mkdir(dir, folderMode); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return os.Chmod(dir, folderMode)
}

func (w *Workspace) resolve(path string) (string, error) {
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("%w: %s", errOutside, path)
	}
	return filepath.Join(w.root, filepath.FromSlash(path)), nil
}
