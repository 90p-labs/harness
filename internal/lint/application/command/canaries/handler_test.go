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

	"github.com/90p-labs/harness/internal/lint/application/command/canaries"
	"github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	purity    = "ddd/domain-purity"
	direction = "ddd/layer-direction"
	suffix    = "ddd/file-suffix"
	engine    = domain.EngineName("deptrac")

	proofFolder     = "lint/proof"
	purityCanary    = "ddd-domain-purity"
	directionCanary = "ddd-layer-direction"
	waiver          = "deptrac checks dependencies, not file names"
	canaryFile      = "src/Order/Domain/Order.php"
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
		{ID: suffix, Waived: &reason},
	}}
}

var reason = waiver

func proofDir(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(proofFolder))
}

func repoWithCanaries(t *testing.T, folders ...string) string {
	t.Helper()
	repo := t.TempDir()
	for _, folder := range folders {
		file := filepath.Join(proofDir(repo), filepath.FromSlash(folder), filepath.FromSlash(canaryFile))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("<?php\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func fresh(string) error { return nil }

func handle(t *testing.T, repo string, manifest domain.Manifest, rules map[domain.EngineName][]domain.Rule, set *engines) ([]domain.CanaryResult, error) {
	t.Helper()
	return handleWith(t, repo, func(string) (domain.Manifest, error) { return manifest, nil }, fresh, rules, set)
}

func handleWith(t *testing.T, repo string, read canaries.ReadManifest, check canaries.CheckVersion, rules map[domain.EngineName][]domain.Rule, set *engines) ([]domain.CanaryResult, error) {
	t.Helper()
	inProof := func(dir string) (domain.Manifest, error) {
		if dir != proofDir(repo) {
			t.Fatalf("manifest read in %s, want %s", dir, proofDir(repo))
		}
		return read(dir)
	}
	return canaries.NewHandler(catalogue(), inProof, rules, set, check).Handle(context.Background(), canaries.Command{Dir: repo, Proof: proofFolder})
}

func finding(id string) shareddomain.Finding {
	return findingAt(id, canaryFile)
}

func findingAt(id, file string) shareddomain.Finding {
	return shareddomain.Finding{Rule: id, Severity: shareddomain.SeverityError, File: file, Line: 3, Message: "broken"}
}

func TestFindingOutsideCanaryFails(t *testing.T) {
	const outside = "../../../cmd/app/main.go"
	cases := []struct {
		name   string
		file   string
		status domain.CanaryStatus
		detail string
	}{
		{name: "a canary file", file: canaryFile, status: domain.CanaryPass},
		{name: "a folder that holds a canary file", file: "src/Order", status: domain.CanaryPass},
		{name: "a file outside the canary", file: outside, status: domain.CanaryFail, detail: "the engine deptrac gave a ddd/domain-purity finding for " + outside + ", which is outside the canary"},
		{name: "a file that the canary does not hold", file: "src/Order/Domain/Other.php", status: domain.CanaryFail, detail: "the engine deptrac gave a ddd/domain-purity finding for src/Order/Domain/Other.php, which is outside the canary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoWithCanaries(t, purityCanary, directionCanary)
			set := &engines{findings: map[string][]shareddomain.Finding{
				filepath.Base(purityCanary): {findingAt(purity, tc.file)},
			}}

			got, err := handle(t, repo, fullManifest(), active(), set)

			if err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if got[0].Status != tc.status || got[0].Detail != tc.detail {
				t.Fatalf("result = %+v, want status %s and detail %q", got[0], tc.status, tc.detail)
			}
		})
	}
}

func TestStaleLintFiles(t *testing.T) {
	repo := repoWithCanaries(t, purityCanary, directionCanary)
	set := &engines{}
	stale := func(string) error { return fmt.Errorf("%w (have %q, want %q)", domain.ErrStaleLintFiles, "1", "2") }

	_, err := handleWith(t, repo, func(string) (domain.Manifest, error) { return fullManifest(), nil }, stale, active(), set)

	if !errors.Is(err, domain.ErrStaleLintFiles) || len(set.checks) != 0 {
		t.Fatalf("Handle = %v after %d engine checks, want ErrStaleLintFiles before any check", err, len(set.checks))
	}
}

func TestBuiltInNoManifest(t *testing.T) {
	builtIn := map[domain.EngineName][]domain.Rule{domain.EngineHarness: {{ID: purity, Engine: domain.EngineHarness}}}
	noManifest := func(string) (domain.Manifest, error) { return domain.Manifest{}, domain.ErrNoManifest }
	cases := []struct {
		name  string
		rules map[domain.EngineName][]domain.Rule
		want  error
	}{
		{name: "only built-in rules need no canary", rules: builtIn},
		{name: "a plugin rule with a catalogue id needs the manifest", rules: active(), want: domain.ErrNoManifest},
		{name: "no pack keeps the setup error", rules: map[domain.EngineName][]domain.Rule{}, want: domain.ErrNoManifest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := handleWith(t, t.TempDir(), noManifest, fresh, tc.rules, &engines{})

			if !errors.Is(err, tc.want) || (tc.want == nil && got != nil) {
				t.Fatalf("Handle = %+v, %v, want no result and %v", got, err, tc.want)
			}
		})
	}
}

func TestBuiltInEngineHint(t *testing.T) {
	repo := repoWithCanaries(t, purityCanary, directionCanary)
	rules := map[domain.EngineName][]domain.Rule{
		domain.EngineHarness: {{ID: purity, Engine: domain.EngineHarness}, {ID: direction, Engine: domain.EngineHarness}},
	}

	got, err := handle(t, repo, fullManifest(), rules, &engines{})

	want := "the engine harness gave no ddd/domain-purity finding, and the built-in engine reads only TypeScript and JavaScript"
	if err != nil || got[0].Detail != want {
		t.Fatalf("Handle = %+v, %v, want the detail %q", got, err, want)
	}
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
		{dir: filepath.Join(proofDir(repo), purityCanary), engine: engine, rules: []domain.Rule{rule(purity)}},
		{dir: filepath.Join(proofDir(repo), directionCanary), engine: engine, rules: []domain.Rule{rule(direction)}},
	}
	if !reflect.DeepEqual(set.checks, wantChecks) {
		t.Fatalf("checks = %+v, want %+v", set.checks, wantChecks)
	}
}

func TestHandlerEnginesCanary(t *testing.T) {
	const tsCanary = "ts-domain-purity"
	repo := repoWithCanaries(t, purityCanary, tsCanary)
	builtIn := domain.Rule{ID: purity, Engine: domain.EngineHarness, Severity: shareddomain.SeverityError, Scope: "src/**"}
	rules := map[domain.EngineName][]domain.Rule{engine: {rule(purity)}, domain.EngineHarness: {builtIn}}
	manifest := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: purity, Engines: map[domain.EngineName]domain.ManifestProof{
			engine:               {Canary: purityCanary},
			domain.EngineHarness: {Canary: tsCanary},
		}},
		{ID: direction, Waived: &reason},
		{ID: suffix, Waived: &reason},
	}}
	set := &engines{findings: map[string][]shareddomain.Finding{filepath.Base(purityCanary): {finding(purity)}}}

	got, err := handle(t, repo, manifest, rules, set)

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := []domain.CanaryResult{
		{Rule: purity, Engine: engine, Canary: purityCanary, Status: domain.CanaryPass},
		{Rule: purity, Engine: domain.EngineHarness, Canary: tsCanary, Status: domain.CanaryFail, Detail: "the engine harness gave no ddd/domain-purity finding, and the built-in engine reads only TypeScript and JavaScript"},
		{Rule: direction, Status: domain.CanaryWaived, Detail: waiver},
		{Rule: suffix, Status: domain.CanaryWaived, Detail: waiver},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results = %+v, want %+v", got, want)
	}
	wantChecks := []check{
		{dir: filepath.Join(proofDir(repo), purityCanary), engine: engine, rules: []domain.Rule{rule(purity)}},
		{dir: filepath.Join(proofDir(repo), tsCanary), engine: domain.EngineHarness, rules: []domain.Rule{builtIn}},
	}
	if !reflect.DeepEqual(set.checks, wantChecks) {
		t.Fatalf("checks = %+v, want %+v", set.checks, wantChecks)
	}
}

func TestHandlerEnginesNeedAnEntryEach(t *testing.T) {
	repo := repoWithCanaries(t, purityCanary, directionCanary)
	rules := active()
	rules[domain.EngineHarness] = []domain.Rule{{ID: purity, Engine: domain.EngineHarness}}

	_, err := handle(t, repo, fullManifest(), rules, &engines{})

	if !errors.Is(err, domain.ErrOneProofForEngines) || !strings.Contains(err.Error(), "deptrac and harness") {
		t.Fatalf("Handle = %v, want ErrOneProofForEngines naming both engines", err)
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
		file    string
		rules   map[domain.EngineName][]domain.Rule
		err     error
		want    error
		names   string
	}{
		{name: "a missing canary folder", folders: []string{purityCanary}, rules: active(), want: domain.ErrCanaryMissing, names: direction},
		{name: "a canary that is a file", folders: []string{purityCanary}, file: directionCanary, rules: active(), want: domain.ErrCanaryNotFolder, names: direction},
		{name: "a canary rule with no active rule", folders: []string{purityCanary, directionCanary}, rules: map[domain.EngineName][]domain.Rule{engine: {rule(purity)}}, want: domain.ErrCanaryRuleInactive, names: direction},
		{name: "an engine error", folders: []string{purityCanary, directionCanary}, rules: active(), err: errEngine, want: errEngine, names: purity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoWithCanaries(t, tc.folders...)
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(proofDir(repo), tc.file), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}

			_, err := handle(t, repo, fullManifest(), tc.rules, &engines{err: tc.err})

			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.names) {
				t.Fatalf("Handle = %v, want %v naming %s", err, tc.want, tc.names)
			}
		})
	}
}

func TestHandlerProofFolder(t *testing.T) {
	repo := repoWithCanaries(t, purityCanary, directionCanary)
	read := func(string) (domain.Manifest, error) { return fullManifest(), nil }
	linkedRepo := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(repo, linkedRepo); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(repo, "lint", "outside")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		proof   string
		outside bool
	}{
		{name: "an absolute path inside the work tree", proof: proofDir(repo)},
		{name: "an absolute path through a symlinked parent", proof: proofDir(linkedRepo)},
		{name: "a parent path", proof: "../x", outside: true},
		{name: "an absolute path outside the work tree", proof: t.TempDir(), outside: true},
		{name: "a symlink to a folder outside the work tree", proof: "lint/outside", outside: true},
		{name: "the work tree itself", proof: ".", outside: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := canaries.NewHandler(catalogue(), read, active(), &engines{}, fresh)

			_, err := handler.Handle(context.Background(), canaries.Command{Dir: repo, Proof: tc.proof})

			if tc.outside && (!errors.Is(err, domain.ErrProofOutsideRepo) || !strings.Contains(err.Error(), tc.proof)) {
				t.Fatalf("Handle = %v, want ErrProofOutsideRepo naming %s", err, tc.proof)
			}
			if !tc.outside && errors.Is(err, domain.ErrProofOutsideRepo) {
				t.Fatalf("Handle = %v, want the proof folder accepted", err)
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
