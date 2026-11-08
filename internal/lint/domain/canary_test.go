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

	purityCanary    = "lint/canaries/ddd-domain-purity"
	directionCanary = "lint/canaries/ddd-layer-direction"
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
		{name: "a parent path", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "../canaries/x"}}}, want: domain.ErrCanaryOutsideRepo, names: purity},
		{name: "an absolute path", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "/tmp/x"}}}, want: domain.ErrCanaryOutsideRepo, names: purity},
		{name: "no pack key", manifest: domain.Manifest{}, want: domain.ErrNoPackKey, names: "pack"},
		{name: "an empty waiver", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary}, {ID: suffix, Waived: reason("")}}}, want: domain.ErrShortWaiver, names: suffix},
		{name: "a one-word waiver", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: purityCanary}, {ID: suffix, Waived: reason("a")}}}, want: domain.ErrShortWaiver, names: suffix},
		{name: "every rule waived", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Waived: reason(suffixWaiver)}, {ID: suffix, Waived: reason(suffixWaiver)}}}, want: domain.ErrAllWaived, names: domain.ManifestFile},
		{name: "the repo root", manifest: domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{{ID: purity, Canary: "lint/.."}}}, want: domain.ErrCanaryOutsideRepo, names: purity},
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
