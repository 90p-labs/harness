package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
)

const (
	purity    = "ddd/domain-purity"
	direction = "ddd/layer-direction"
	suffix    = "ddd/file-suffix"

	purityCanary    = "ddd-domain-purity"
	directionCanary = "ddd-layer-direction"
	suffixWaiver    = "the tool checks dependencies, not file names"
)

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

func reason(text string) *string {
	return &text
}

func TestManifestResults(t *testing.T) {
	manifest := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: suffix, Waived: reason(suffixWaiver)},
		{ID: purity, Canary: purityCanary},
	}}

	got, err := manifest.Results(catalogue())

	if err != nil {
		t.Fatalf("Results: %v", err)
	}
	want := []domain.CanaryResult{
		{Rule: purity, Canary: purityCanary, Status: domain.CanaryToRun},
		{Rule: direction, Status: domain.CanaryFail, Detail: "not in " + domain.ManifestFile},
		{Rule: suffix, Status: domain.CanaryWaived, Detail: suffixWaiver},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Results = %+v, want %+v", got, want)
	}
}

const (
	deptrac          = domain.EngineName("deptrac")
	tsPurityCanary   = "ts-domain-purity"
	builtInWaiver    = "the binary fixtures prove it"
	notInManifestMsg = "not in " + domain.ManifestFile
)

func proofs(entries map[domain.EngineName]domain.ManifestProof) []domain.ManifestRule {
	return []domain.ManifestRule{{ID: purity, Engines: entries}}
}

func TestManifestEngines(t *testing.T) {
	manifest := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: purity, Engines: map[domain.EngineName]domain.ManifestProof{
			domain.EngineHarness: {Canary: tsPurityCanary},
			deptrac:              {Canary: purityCanary},
		}},
		{ID: suffix, Engines: map[domain.EngineName]domain.ManifestProof{
			domain.EngineHarness: {Waived: reason(builtInWaiver)},
		}},
	}}

	got, err := manifest.Results(catalogue())

	if err != nil {
		t.Fatalf("Results: %v", err)
	}
	want := []domain.CanaryResult{
		{Rule: purity, Engine: deptrac, Canary: purityCanary, Status: domain.CanaryToRun},
		{Rule: purity, Engine: domain.EngineHarness, Canary: tsPurityCanary, Status: domain.CanaryToRun},
		{Rule: direction, Status: domain.CanaryFail, Detail: notInManifestMsg},
		{Rule: suffix, Engine: domain.EngineHarness, Status: domain.CanaryWaived, Detail: builtInWaiver},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Results = %+v, want %+v", got, want)
	}
}

func TestManifestEnginesConfigErrors(t *testing.T) {
	cases := []struct {
		name  string
		rules []domain.ManifestRule
		want  error
		names string
	}{
		{name: "engines and a canary", rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary, Engines: map[domain.EngineName]domain.ManifestProof{deptrac: {Canary: purityCanary}}}}, want: domain.ErrCanaryOrWaiver, names: purity},
		{name: "an empty engines map", rules: proofs(map[domain.EngineName]domain.ManifestProof{}), want: domain.ErrCanaryOrWaiver, names: purity},
		{name: "an engine with neither", rules: proofs(map[domain.EngineName]domain.ManifestProof{deptrac: {}}), want: domain.ErrCanaryOrWaiver, names: string(deptrac)},
		{name: "an engine with both", rules: proofs(map[domain.EngineName]domain.ManifestProof{deptrac: {Canary: purityCanary, Waived: reason(suffixWaiver)}}), want: domain.ErrCanaryOrWaiver, names: string(deptrac)},
		{name: "a short engine waiver", rules: proofs(map[domain.EngineName]domain.ManifestProof{deptrac: {Canary: purityCanary}, domain.EngineHarness: {Waived: reason("a")}}), want: domain.ErrShortWaiver, names: string(domain.EngineHarness)},
		{name: "an engine canary outside", rules: proofs(map[domain.EngineName]domain.ManifestProof{deptrac: {Canary: "../x"}}), want: domain.ErrCanaryOutsideProof, names: string(deptrac)},
		{name: "every engine waived", rules: proofs(map[domain.EngineName]domain.ManifestProof{deptrac: {Waived: reason(suffixWaiver)}, domain.EngineHarness: {Waived: reason(builtInWaiver)}}), want: domain.ErrAllWaived, names: domain.ManifestFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.Manifest{Pack: domain.PackDDD, Rules: tc.rules}.Results(catalogue())

			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.names) {
				t.Fatalf("Results = %v, want %v naming %s", err, tc.want, tc.names)
			}
		})
	}
}

func TestEnginesCheck(t *testing.T) {
	both := map[string][]domain.EngineName{purity: {deptrac, domain.EngineHarness}}
	one := map[string][]domain.EngineName{purity: {deptrac}}
	single := domain.CanaryResult{Rule: purity, Canary: purityCanary, Status: domain.CanaryToRun}
	onDeptrac := domain.CanaryResult{Rule: purity, Engine: deptrac, Canary: purityCanary, Status: domain.CanaryToRun}
	unlisted := domain.CanaryResult{Rule: purity, Status: domain.CanaryFail, Detail: notInManifestMsg}
	waived := domain.CanaryResult{Rule: suffix, Status: domain.CanaryWaived, Detail: suffixWaiver}

	t.Run("keeps a single entry on its one engine", func(t *testing.T) {
		got, err := domain.ForEngines([]domain.CanaryResult{single, waived}, one)

		if err != nil || !reflect.DeepEqual(got, []domain.CanaryResult{single, waived}) {
			t.Fatalf("ForEngines = %+v, %v, want the results unchanged", got, err)
		}
	})

	t.Run("fails each active engine with no entry", func(t *testing.T) {
		got, err := domain.ForEngines([]domain.CanaryResult{onDeptrac}, both)

		want := []domain.CanaryResult{onDeptrac, {Rule: purity, Engine: domain.EngineHarness, Status: domain.CanaryFail, Detail: notInManifestMsg}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("ForEngines = %+v, %v, want %+v", got, err, want)
		}
	})

	t.Run("fails an unlisted rule once for each engine", func(t *testing.T) {
		got, err := domain.ForEngines([]domain.CanaryResult{unlisted}, both)

		want := []domain.CanaryResult{
			{Rule: purity, Engine: deptrac, Status: domain.CanaryFail, Detail: notInManifestMsg},
			{Rule: purity, Engine: domain.EngineHarness, Status: domain.CanaryFail, Detail: notInManifestMsg},
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("ForEngines = %+v, %v, want %+v", got, err, want)
		}
	})

	errorCases := []struct {
		name    string
		results []domain.CanaryResult
		active  map[string][]domain.EngineName
		want    error
		names   []string
	}{
		{name: "a single entry for two engines", results: []domain.CanaryResult{single}, active: both, want: domain.ErrOneProofForEngines, names: []string{purity, string(deptrac), string(domain.EngineHarness)}},
		{name: "a single canary for no engine", results: []domain.CanaryResult{single}, active: nil, want: domain.ErrCanaryRuleInactive, names: []string{purity}},
		{name: "an entry for an inactive engine", results: []domain.CanaryResult{{Rule: purity, Engine: domain.EngineHarness, Status: domain.CanaryWaived, Detail: builtInWaiver}}, active: one, want: domain.ErrCanaryRuleInactive, names: []string{purity, string(domain.EngineHarness)}},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ForEngines(tc.results, tc.active)

			if !errors.Is(err, tc.want) {
				t.Fatalf("ForEngines = %v, want %v", err, tc.want)
			}
			for _, name := range tc.names {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error %q does not name %s", err, name)
				}
			}
		})
	}
}

func TestManifestConfigErrors(t *testing.T) {
	cases := []struct {
		name     string
		manifest domain.Manifest
		want     error
		names    string
	}{
		{name: "an unknown pack", manifest: domain.Manifest{Pack: "hexagonal"}, want: domain.ErrUnknownPack, names: "hexagonal"},
		{name: "an unknown id", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: "ddd/typo", Canary: purityCanary}}}, want: domain.ErrUnknownRule, names: "ddd/typo"},
		{name: "a duplicate id", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary}, {ID: purity, Waived: reason(suffixWaiver)}}}, want: domain.ErrDuplicateRule, names: purity},
		{name: "both canary and waived", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary, Waived: reason(suffixWaiver)}}}, want: domain.ErrCanaryOrWaiver, names: purity},
		{name: "neither canary nor waived", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity}}}, want: domain.ErrCanaryOrWaiver, names: purity},
		{name: "a parent path", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "../canaries/x"}}}, want: domain.ErrCanaryOutsideProof, names: purity},
		{name: "an escape from the proof folder", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "../escape"}}}, want: domain.ErrCanaryOutsideProof, names: purity},
		{name: "an absolute path", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "/tmp/x"}}}, want: domain.ErrCanaryOutsideProof, names: purity},
		{name: "no pack key", manifest: domain.Manifest{}, want: domain.ErrNoPackKey, names: "pack"},
		{name: "an empty waiver", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary}, {ID: suffix, Waived: reason("")}}}, want: domain.ErrShortWaiver, names: suffix},
		{name: "a one-word waiver", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary}, {ID: suffix, Waived: reason("a")}}}, want: domain.ErrShortWaiver, names: suffix},
		{name: "every rule waived", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Waived: reason(suffixWaiver)}, {ID: suffix, Waived: reason(suffixWaiver)}}}, want: domain.ErrAllWaived, names: domain.ManifestFile},
		{name: "the repo root", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "lint/.."}}}, want: domain.ErrCanaryOutsideProof, names: purity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.manifest.Results(catalogue())

			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.names) {
				t.Fatalf("Results = %v, want %v naming %s", err, tc.want, tc.names)
			}
		})
	}
}
