package process

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// EnvHome names the user state folder.
	EnvHome = "HARNESS_HOME"
	// homeName is the folder under the home of the user when EnvHome is unset.
	homeName = ".harness"
)

// UserDir returns the user state folder of the panel, and creates it with
// mode 0700. It is $HARNESS_HOME when that is an absolute path, else
// ~/.harness. A relative $HARNESS_HOME is an error, since it would put the
// state files in the current folder. A folder that exists is chmod to 0700.
func UserDir() (string, error) {
	dir := os.Getenv(EnvHome)
	if dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%s must be an absolute path, got %q", EnvHome, dir)
		}
		dir = filepath.Clean(dir)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find the home folder: %w", err)
		}
		dir = filepath.Join(home, homeName)
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", fmt.Errorf("create panel user dir: %w", err)
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		return "", fmt.Errorf("chmod panel user dir: %w", err)
	}
	return dir, nil
}
