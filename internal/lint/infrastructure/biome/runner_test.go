//go:build linters

package biome_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/biome"
	"github.com/90p-labs/harness/internal/lint/infrastructure/fixture"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
)

func TestBiomeEngineFixtures(t *testing.T) {
	all, err := packs.All()
	if err != nil {
		t.Fatal(err)
	}
	engine := biome.New(packs.Files)
	checked := 0
	for _, pack := range all {
		for _, rule := range pack.Rules {
			if !slices.Contains([]domain.EngineName{domain.EngineBiomeGrit, domain.EngineBiomeRule}, rule.Engine) {
				continue
			}
			checked++
			t.Run(rule.ID, func(t *testing.T) {
				prefix, name, _ := strings.Cut(rule.ID, "/")
				dir := fixture.Extract(t, filepath.Join("testdata", prefix, name+".txtar"))
				rules := []domain.Rule{rule}
				if err := engine.Prepare(dir, rules); err != nil {
					t.Fatalf("Prepare: %v", err)
				}
				findings, err := engine.Run(context.Background(), dir, rules)
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
				if len(findings) != 1 || findings[0].Rule != rule.ID || findings[0].Line == 0 {
					t.Fatalf("findings = %+v, want exactly one for %s", findings, rule.ID)
				}
			})
		}
	}
	if checked == 0 {
		t.Fatal("the catalogue has no Biome rule")
	}
}

func TestBiomeEngineUnusableTool(t *testing.T) {
	oldBiome := t.TempDir()
	if err := os.WriteFile(filepath.Join(oldBiome, "biome"), []byte("#!/bin/sh\necho 'Version: 1.9.4'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{"no biome on the path": t.TempDir(), "biome below 2.0": oldBiome}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			dir := fixture.Extract(t, filepath.Join("testdata", "next", "no-img-element.txtar"))
			t.Setenv("PATH", path)
			rule := domain.Rule{ID: "next/no-img-element", Engine: domain.EngineBiomeRule, Scope: "src/**/*.tsx", BiomeRule: "lint/performance/noImgElement"}

			_, err := biome.New(packs.Files).Run(context.Background(), dir, []domain.Rule{rule})

			if !errors.Is(err, domain.ErrEngineMissing) {
				t.Fatalf("Run = %v, want ErrEngineMissing", err)
			}
		})
	}
}
