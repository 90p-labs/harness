package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func treeOf(paths ...string) domain.Tree {
	var tree domain.Tree
	for _, p := range paths {
		tree.Entries = append(tree.Entries, domain.TreeEntry{Path: p})
	}
	return tree
}

func TestClassifyTrack(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		files    []string
		want     shareddomain.Track
	}{
		{"two folders", "", []string{"internal/ping/ping.go", "internal/ping/ping_test.go", "cmd/main.go"}, shareddomain.TrackLight},
		{"three folders", "", []string{"a/x.go", "b/y.go", "c/z.go"}, shareddomain.TrackFull},
		{"domain folder", "", []string{"internal/ping/domain/pong.go"}, shareddomain.TrackFull},
		{"sql file", "", []string{"db/schema.sql"}, shareddomain.TrackFull},
		{"migrations folder", "", []string{"db/migrations/0001_init.go"}, shareddomain.TrackFull},
		{"command folder", "", []string{"internal/ping/application/command/ping.go"}, shareddomain.TrackFull},
		{"query folder", "", []string{"internal/ping/application/query/ping.go"}, shareddomain.TrackFull},
		{"declared full", "full", []string{"cmd/main.go"}, shareddomain.TrackFull},
		{"declared light on a full plan", "light", []string{"a/x.go", "b/y.go", "c/z.go"}, shareddomain.TrackFull},
		{"declared light", "light", []string{"cmd/main.go"}, shareddomain.TrackLight},
		{"no tree", "", nil, shareddomain.TrackLight},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := domain.Document{Track: c.declared, CodeTree: treeOf(c.files...)}
			if got := domain.Classify(doc); got != c.want {
				t.Fatalf("Classify = %q, want %q", got, c.want)
			}
		})
	}

	t.Run("directories do not count as folders", func(t *testing.T) {
		tree := treeOf("a/x.go", "b/y.go")
		tree.Entries = append(tree.Entries, domain.TreeEntry{Path: "c/", IsDir: true})
		if got := domain.Classify(domain.Document{CodeTree: tree}); got != shareddomain.TrackLight {
			t.Fatalf("Classify = %q", got)
		}
	})
}
