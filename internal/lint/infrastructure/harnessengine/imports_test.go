package harnessengine

import (
	"context"
	"slices"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/fixture"
)

func TestImportGraph(t *testing.T) {
	dir := fixture.Extract(t, "testdata/imports.txtar")
	tree, err := loadTree(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	graph, err := importGraph(tree, []string{"src/**"})
	if err != nil {
		t.Fatal(err)
	}

	const from = "src/main.ts"
	want := []domain.Import{
		{From: from, To: "src/first.ts", Specifier: "./first", Line: 2},
		{From: from, To: "src/types/shape.ts", Specifier: "@/types/shape", TypeOnly: true, Line: 3},
		{From: from, To: "src/second.ts", Specifier: "@/second", Line: 4},
		{From: from, Specifier: "react", Package: true, Line: 8},
		{From: from, To: "src/side-effect.ts", Specifier: "./side-effect", Line: 9},
		{From: from, To: "src/lib/index.ts", Specifier: "./lib", Line: 10},
		{From: from, To: "src/ui/index.ts", Specifier: "~ui", Line: 11},
		{From: from, To: "src/kinds.ts", Specifier: "./kinds", TypeOnly: true, Line: 12},
		{From: from, To: "src/lazy.ts", Specifier: "./lazy", Line: 15},
		{From: from, Specifier: "node:fs", Package: true, Line: 16},
		{From: from, To: "src/view.tsx", Specifier: "./view", Line: 17},
		{From: from, To: "src/first.ts", Specifier: "./first", Line: 19},
	}
	var got []domain.Import
	for _, edge := range graph.Edges {
		if edge.From == from {
			got = append(got, edge)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("edges of %s\n got %+v\nwant %+v", from, got, want)
	}
}
