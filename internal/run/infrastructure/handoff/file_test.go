package handoff_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/handoff"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const runID = "20261110-180018-end-the-lead-session"

func TestReadHandoff(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, worktree string)
		want    []byte
		wantErr bool
		wantMsg string
	}{
		{"missing file gives no content and no error", func(*testing.T, string) {}, nil, false, ""},
		{"a regular file is read", func(t *testing.T, w string) { write(t, w, []byte("Phase: working\n")) }, []byte("Phase: working\n"), false, ""},
		{"an empty file is content, not missing", func(t *testing.T, w string) { write(t, w, []byte{}) }, []byte{}, false, ""},
		{"an oversize file is refused with its real size", func(t *testing.T, w string) {
			write(t, w, bytes.Repeat([]byte("x"), domain.HandoffLimit*3))
		}, nil, true, fmt.Sprintf(domain.HandoffSizeFormat, domain.HandoffLimit*3, domain.HandoffLimit)},
		{"a symlink is refused", func(t *testing.T, w string) {
			target := filepath.Join(t.TempDir(), "secret")
			if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(w, shareddomain.RunHandoffFile(runID))); err != nil {
				t.Fatal(err)
			}
		}, nil, true, ""},
		{"a directory is refused", func(t *testing.T, w string) {
			if err := os.Mkdir(filepath.Join(w, shareddomain.RunHandoffFile(runID)), 0o700); err != nil {
				t.Fatal(err)
			}
		}, nil, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree := t.TempDir()
			if err := os.MkdirAll(filepath.Join(worktree, filepath.Dir(shareddomain.RunHandoffFile(runID))), 0o700); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, worktree)

			got, err := handoff.File{}.ReadHandoff(context.Background(), worktree, runID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && err.Error() != tc.wantMsg {
				t.Fatalf("err = %q, want %q", err, tc.wantMsg)
			}
			if !bytes.Equal(got, tc.want) || (got == nil) != (tc.want == nil) {
				t.Fatalf("got %d bytes (nil %v), want %d bytes (nil %v)", len(got), got == nil, len(tc.want), tc.want == nil)
			}
		})
	}
}

func write(t *testing.T, worktree string, data []byte) {
	t.Helper()
	path := filepath.Join(worktree, shareddomain.RunHandoffFile(runID))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
