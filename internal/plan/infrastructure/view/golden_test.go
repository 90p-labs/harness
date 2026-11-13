package view_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden page hashes")

// TestPageGoldenBytes pins the bytes of each page by sha256, because the pages
// hold about 4 MB. Set GOLDEN_BASE to a folder with the base plan.html and
// program.html to get the first differing byte offset on a mismatch. To fill
// that folder, run on the base commit:
//
//	GOLDEN_BASE=/tmp/golden go test ./internal/plan/infrastructure/view/ -run TestPageGoldenBytes -args -update
func TestPageGoldenBytes(t *testing.T) {
	baseDir := os.Getenv("GOLDEN_BASE")
	_, program := programInput(t)
	pages := map[string]string{
		"plan":    render(t, baseInput(t)),
		"program": render(t, program),
	}
	for name, page := range pages {
		t.Run(name, func(t *testing.T) {
			sum := sha256.Sum256([]byte(page))
			got := hex.EncodeToString(sum[:])
			file := filepath.Join("testdata", name+".sha256")

			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(got+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if baseDir != "" {
					if err := os.MkdirAll(baseDir, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(baseDir, name+".html"), []byte(page), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				return
			}

			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%v\nrun: go test ./internal/plan/infrastructure/view/ -run TestPageGoldenBytes -args -update", err)
			}
			want := strings.TrimSpace(string(raw))
			if got == want {
				return
			}

			dir, err := os.MkdirTemp("", "golden-")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, name+".html")
			if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
				t.Fatal(err)
			}
			msg := "page differs, written to " + path + "\nwant sha256 " + want + "\ngot  sha256 " + got
			if baseDir == "" {
				msg += "\nset GOLDEN_BASE to a folder with the base plan.html and program.html for the first differing offset"
			} else if base, err := os.ReadFile(filepath.Join(baseDir, name+".html")); err != nil {
				msg += "\nGOLDEN_BASE: " + err.Error()
			} else {
				msg += "\nfirst differing offset " + strconv.Itoa(firstDiff(base, []byte(page)))
			}
			t.Error(msg)
		})
	}
}

func TestFirstDiff(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 3},
		{"abc", "xbc", 0},
		{"abc", "abx", 2},
		{"ab", "abc", 2},
		{"", "a", 0},
	} {
		if got := firstDiff([]byte(c.a), []byte(c.b)); got != c.want {
			t.Errorf("firstDiff(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func firstDiff(a, b []byte) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
