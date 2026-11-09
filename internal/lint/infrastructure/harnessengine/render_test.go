package harnessengine

import (
	"context"
	"errors"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
)

const (
	testChart          = "deploy/chart"
	testDir            = "/work/repo"
	renderedDeployment = "---\n# Source: chart/templates/deployment.yaml\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n"
)

var errRender = errors.New("no plugin rendered the chart")

func TestRenderChart(t *testing.T) {
	chartRule := []domain.Rule{{ID: domain.RuleImmutableTags}}
	chartTree := domain.Tree{Files: []string{testChart + "/Chart.yaml"}}

	t.Run("reads the manifest of the render function", func(t *testing.T) {
		var gotDir, gotChart string
		render := func(_ context.Context, dir, chart string) ([]byte, error) {
			gotDir, gotChart = dir, chart
			return []byte(renderedDeployment), nil
		}

		resources, err := renderChart(context.Background(), render, testDir, chartTree, testChart, chartRule)

		if err != nil {
			t.Fatal(err)
		}
		if gotDir != testDir || gotChart != testChart {
			t.Errorf("render got dir %q and chart %q", gotDir, gotChart)
		}
		if len(resources) != 1 || resources[0].Kind != kindDeployment || resources[0].file != testChart+"/templates/deployment.yaml" {
			t.Errorf("resources = %+v", resources)
		}
	})

	t.Run("stops on a render error", func(t *testing.T) {
		render := func(context.Context, string, string) ([]byte, error) { return nil, errRender }

		_, err := renderChart(context.Background(), render, testDir, chartTree, testChart, chartRule)

		if !errors.Is(err, errRender) {
			t.Errorf("err = %v, want %v", err, errRender)
		}
	})

	unsourced := map[string]string{
		"only documents with no source":                 "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n",
		"a document with no source after a sourced one": renderedDeployment + "---\nkind: Deployment\nmetadata:\n  name: worker\n",
	}
	for name, manifest := range unsourced {
		t.Run("stops on "+name, func(t *testing.T) {
			render := func(context.Context, string, string) ([]byte, error) { return []byte(manifest), nil }

			_, err := renderChart(context.Background(), render, testDir, chartTree, testChart, chartRule)

			if !errors.Is(err, errNoSource) {
				t.Errorf("err = %v, want %v", err, errNoSource)
			}
		})
	}

	t.Run("reads a blank manifest as no resources", func(t *testing.T) {
		render := func(context.Context, string, string) ([]byte, error) { return []byte("---\n\n"), nil }

		resources, err := renderChart(context.Background(), render, testDir, chartTree, testChart, chartRule)

		if err != nil || resources != nil {
			t.Errorf("resources = %+v, err = %v, want none", resources, err)
		}
	})

	skips := map[string]struct {
		tree  domain.Tree
		rules []domain.Rule
	}{
		"no chart rule": {tree: chartTree, rules: []domain.Rule{{ID: domain.RuleRunbook}}},
		"no chart file": {tree: domain.Tree{Files: []string{"deploy/values.yaml"}}, rules: chartRule},
	}
	for name, tt := range skips {
		t.Run(name, func(t *testing.T) {
			render := func(context.Context, string, string) ([]byte, error) {
				t.Error("render called")
				return nil, nil
			}
			resources, err := renderChart(context.Background(), render, testDir, tt.tree, testChart, tt.rules)
			if err != nil || resources != nil {
				t.Errorf("resources = %+v, err = %v, want none", resources, err)
			}
		})
	}
}
