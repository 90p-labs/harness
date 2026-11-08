package canaries_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/application/command/canaries"
	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	purity    = "ddd/domain-purity"
	direction = "ddd/layer-direction"
	suffix    = "ddd/file-suffix"
	engine    = domain.EngineName("deptrac")

	purityCanary    = "lint/canaries/ddd-domain-purity"
	directionCanary = "lint/canaries/ddd-layer-direction"
	waiver          = "deptrac checks dependencies, not file names"
)

var errEngine = errors.New("deptrac crashed")

type check struct {
	dir    string
	engine domain.EngineName
	rules  []domain.Rule
}

type engines struct {
	findings map[string][]shareddomain.Finding
	err      error
	checks   []check
}

func (e *engines) Check(_ context.Context, dir string, engine domain.EngineName, rules []domain.Rule) ([]shareddomain.Finding, error) {
	e.checks = append(e.checks, check{dir: dir, engine: engine, rules: rules})
	return e.findings[filepath.Base(dir)], e.err
}

func (e *engines) Prepare(context.Context, string, domain.EngineName, []domain.Rule) error {
	return nil
}

func catalogue() domain.RuleCatalogue {
	return domain.RuleCatalogue{Packs: []domain.CataloguePack{{
		Name: domain.PackDDD,
		Rules: []domain.CatalogueRule{
			{ID: purity, Description: "purity"},
			{ID: direction, Description: "direction"},
			{ID: suffix, Description: "suffix"},
		},
	}}}
}

func rule(id string) domain.Rule {
	return domain.Rule{ID: id, Engine: engine, Severity: shareddomain.SeverityError, Scope: "src/**/*.php"}
}

func active() map[domain.EngineName][]domain.Rule {
	return map[domain.EngineName][]domain.Rule{
		engine:                 {rule(purity), rule(direction)},
		domain.EngineBiomeGrit: {{ID: "ddd/event-readonly", Engine: domain.EngineBiomeGrit}},
	}
}

func fullManifest() domain.Manifest {
	return domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: purity, Canary: purityCanary},
		{ID: direction, Canary: directionCanary},
		{ID: suffix, Waived: waiver},
	}}
}

func repoWithCanaries(t *testing.T, folders ...string) string {
	t.Helper()
	repo := t.TempDir()
	for _, folder := range folders {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(folder)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func handle(t *testing.T, repo string, manifest domain.Manifest, rules map[domain.EngineName][]domain.Rule, set *engines) ([]domain.CanaryResult, error) {
	t.Helper()
	read := func(dir string) (domain.Manifest, error) {
		if dir != repo {
			t.Fatalf("manifest read in %s, want %s", dir, repo)
		}
		return manifest, nil
	}
	return canaries.NewHandler(catalogue(), read, rules, set).Handle(context.Background(), canaries.Command{Dir: repo})
}

func finding(id string) shareddomain.Finding {
	return shareddomain.Finding{Rule: id, Severity: shareddomain.SeverityError, File: "src/Order/Domain/Order.php", Line: 3, Message: "broken"}
}

func TestHandlerRunsEachCanary(t *testing.T) {
	repo := repoWithCanaries(t, purityCanary, directionCanary)
	set := &engines{findings: map[string][]shareddomain.Finding{
		filepath.Base(purityCanary): {finding(purity)},
	}}

	got, err := handle(t, repo, fullManifest(), active(), set)

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := []domain.CanaryResult{
		{Rule: purity, Canary: purityCanary, Status: domain.CanaryPass},
		{Rule: direction, Canary: directionCanary, Status: domain.CanaryFail, Detail: "the engine deptrac gave no ddd/layer-direction finding"},
		{Rule: suffix, Status: domain.CanaryWaived, Detail: waiver},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results = %+v, want %+v", got, want)
	}
	wantChecks := []check{
		{dir: filepath.Join(repo, filepath.FromSlash(purityCanary)), engine: engine, rules: []domain.Rule{rule(purity)}},
		{dir: filepath.Join(repo, filepath.FromSlash(directionCanary)), engine: engine, rules: []domain.Rule{rule(direction)}},
	}
	if !reflect.DeepEqual(set.checks, wantChecks) {
		t.Fatalf("checks = %+v, want %+v", set.checks, wantChecks)
	}
}

func TestHandlerNamesTheEngineThatRan(t *testing.T) {
	repo := repoWithCanaries(t, purityCanary, directionCanary)
	rules := map[domain.EngineName][]domain.Rule{
		engine:                 {rule(purity)},
		domain.EngineBiomeGrit: {{ID: direction, Engine: domain.EngineBiomeRule}},
	}

	got, err := handle(t, repo, fullManifest(), rules, &engines{})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := fmt.Sprintf("the engine %s gave no %s finding", domain.EngineBiomeGrit, direction)
	if got[1].Detail != want {
		t.Fatalf("detail = %q, want %q", got[1].Detail, want)
	}
}

func TestHandlerStops(t *testing.T) {
	cases := []struct {
		name    string
		folders []string
		rules   map[domain.EngineName][]domain.Rule
		err     error
		want    error
		names   string
	}{
		{name: "a missing canary folder", folders: []string{purityCanary}, rules: active(), want: domain.ErrCanaryMissing, names: direction},
		{name: "a canary rule with no active rule", folders: []string{purityCanary, directionCanary}, rules: map[domain.EngineName][]domain.Rule{engine: {rule(purity)}}, want: domain.ErrCanaryRuleInactive, names: direction},
		{name: "an engine error", folders: []string{purityCanary, directionCanary}, rules: active(), err: errEngine, want: errEngine, names: purity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoWithCanaries(t, tc.folders...)

			_, err := handle(t, repo, fullManifest(), tc.rules, &engines{err: tc.err})

			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.names) {
				t.Fatalf("Handle = %v, want %v naming %s", err, tc.want, tc.names)
			}
		})
	}
}

func TestHandlerStopsAtABadManifest(t *testing.T) {
	_, err := handle(t, t.TempDir(), domain.Manifest{Pack: "hexagonal"}, active(), &engines{})

	if !errors.Is(err, domain.ErrUnknownPack) {
		t.Fatalf("Handle = %v, want ErrUnknownPack", err)
	}
}
