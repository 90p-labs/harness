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

func TestLightReview(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		plan     []string
		changed  []string
		want     bool
	}{
		{"light plan and light files", "", []string{"cmd/main.go"}, []string{"cmd/main.go", "cmd/main_test.go"}, true},
		{"declared full plan", "full", []string{"cmd/main.go"}, []string{"cmd/main.go"}, false},
		{"guard path in the plan", "", []string{"internal/guard/rules.go"}, []string{"cmd/main.go"}, false},
		{"guard path in the changed files", "", []string{"cmd/main.go"}, []string{"internal/guard/rules.go"}, false},
		{"security folder changed", "", []string{"cmd/main.go"}, []string{"internal/security/key.go"}, false},
		{"hooks folder changed", "", []string{"cmd/main.go"}, []string{"internal/hooks/pre.go"}, false},
		{"empty changed list", "", []string{"cmd/main.go"}, nil, false},
		{"many folders changed", "", []string{"cmd/main.go"}, []string{"a/x.go", "b/y.go", "c/z.go"}, false},
		{"domain folder changed", "", []string{"cmd/main.go"}, []string{"internal/ping/domain/pong.go"}, false},
		{"sql file changed", "", []string{"cmd/main.go"}, []string{"db/schema.sql"}, false},
		{"declared light", "light", []string{"cmd/main.go"}, []string{"cmd/main.go"}, true},
		{"unknown track ful", "ful", []string{"cmd/main.go"}, []string{"cmd/main.go"}, false},
		{"unknown track lite", "lite", []string{"cmd/main.go"}, []string{"cmd/main.go"}, false},
		{"full folder only in the plan tree", "", []string{"internal/ping/domain/pong.go"}, []string{"cmd/main.go"}, false},
		{"security folder only in the plan tree", "", []string{"internal/security/key.go"}, []string{"cmd/main.go"}, false},
		{"hooks folder only in the plan tree", "", []string{"internal/hooks/pre.go"}, []string{"cmd/main.go"}, false},
		{"light declared on a risky change", "light", []string{"cmd/main.go"}, []string{"internal/guard/rules.go"}, false},
		{"light declared on a risky plan", "light", []string{"internal/guard/rules.go"}, []string{"cmd/main.go"}, false},
		{"guard folder in mixed case", "", []string{"cmd/main.go"}, []string{"internal/GUARD/rules.go"}, false},
		{"guard plugin path under a mixed case guard folder", "", []string{"cmd/main.go"}, []string{"internal/Guard/application/guardplugin/evil.go"}, false},
		{"hooks folder in upper case", "", []string{"cmd/main.go"}, []string{"HOOKS/hooks.json"}, false},
		{"security folder with a long s", "", []string{"cmd/main.go"}, []string{"ſecurity/x.go"}, false},
		{"mixed case guard folder in the plan tree", "", []string{"internal/Guard/rules.go"}, []string{"cmd/main.go"}, false},
		{"domain folder in mixed case", "", []string{"cmd/main.go"}, []string{"internal/ping/Domain/pong.go"}, false},
		{"command folder in mixed case", "", []string{"cmd/main.go"}, []string{"internal/ping/application/Command/do.go"}, false},
		{"sql file in upper case", "", []string{"cmd/main.go"}, []string{"db/schema.SQL"}, false},
		{"migrations folder in mixed case", "", []string{"cmd/main.go"}, []string{"Migrations/001.go"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := domain.Document{Track: c.declared, CodeTree: treeOf(c.plan...)}
			if got := domain.LightReview(doc, c.changed); got != c.want {
				t.Errorf("LightReview = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNeedsAdvisor(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		files    []string
		want     bool
	}{
		{"full track", "full", []string{"cmd/main.go"}, true},
		{"domain folder", "", []string{"internal/ping/domain/pong.go"}, true},
		{"guard folder", "", []string{"internal/guard/rules.go"}, true},
		{"security folder", "", []string{"internal/security/key.go"}, true},
		{"hooks folder", "", []string{"internal/hooks/pre.go"}, true},
		{"pluginhooks folder", "", []string{"plugin/pluginhooks/pre.sh"}, true},
		{"hooks as a file name", "", []string{"internal/ping/hooks.go"}, false},
		{"hooks in a folder name", "", []string{"internal/webhooks/pre.go"}, false},
		{"light track", "", []string{"cmd/main.go"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := domain.Document{Track: c.declared, CodeTree: treeOf(c.files...)}
			if got := domain.NeedsAdvisor(doc); got != c.want {
				t.Errorf("NeedsAdvisor = %v, want %v", got, c.want)
			}
		})
	}
}
