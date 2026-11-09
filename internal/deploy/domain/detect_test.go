package domain_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/deploy/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestHelmGates(t *testing.T) {
	tests := map[string]struct {
		files []string
		want  []shareddomain.ProposedGate
	}{
		"no chart": {files: []string{"go.mod", "deploy/values.yaml"}},
		"one chart": {
			files: []string{"go.mod", "deploy/chart/Chart.yaml", "deploy/chart/templates/deployment.yaml"},
			want:  []shareddomain.ProposedGate{{ID: domain.GateHelmRender, Run: "helm template deploy/chart > /dev/null"}},
		},
		"two charts in path order": {
			files: []string{"ops/worker/Chart.yaml", "deploy/api/Chart.yaml"},
			want: []shareddomain.ProposedGate{{
				ID:  domain.GateHelmRender,
				Run: "helm template deploy/api > /dev/null && helm template ops/worker > /dev/null",
			}},
		},
		"a chart below another chart": {
			files: []string{"deploy/chart/Chart.yaml", "deploy/chart/charts/redis/Chart.yaml", "deploy/chart-b/Chart.yaml"},
			want: []shareddomain.ProposedGate{{
				ID:  domain.GateHelmRender,
				Run: "helm template deploy/chart > /dev/null && helm template deploy/chart-b > /dev/null",
			}},
		},
		"a chart at the root covers every other chart": {
			files: []string{"Chart.yaml", "charts/redis/Chart.yaml"},
			want:  []shareddomain.ProposedGate{{ID: domain.GateHelmRender, Run: "helm template . > /dev/null"}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := domain.HelmGates(tt.files); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HelmGates = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPromtoolGates(t *testing.T) {
	tests := map[string]struct {
		files []string
		want  []shareddomain.ProposedGate
	}{
		"rules without tests": {files: []string{"deploy/prometheus/api.rules.yaml"}},
		"tests in one top folder": {
			files: []string{"deploy/prometheus/api.rules.test.yaml", "deploy/alerts/worker.rules.test.yaml"},
			want: []shareddomain.ProposedGate{{
				ID:  domain.GatePromtoolTest,
				Run: "find deploy -name '*.rules.test.yaml' -exec promtool test rules {} +",
			}},
		},
		"tests in two top folders": {
			files: []string{"ops/api.rules.test.yaml", "deploy/prometheus/api.rules.test.yaml"},
			want: []shareddomain.ProposedGate{{
				ID:  domain.GatePromtoolTest,
				Run: "find deploy ops -name '*.rules.test.yaml' -exec promtool test rules {} +",
			}},
		},
		"a test at the root": {
			files: []string{"api.rules.test.yaml", "deploy/prometheus/api.rules.test.yaml"},
			want: []shareddomain.ProposedGate{{
				ID:  domain.GatePromtoolTest,
				Run: "find . -name '*.rules.test.yaml' -exec promtool test rules {} +",
			}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := domain.PromtoolGates(tt.files); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PromtoolGates = %+v, want %+v", got, tt.want)
			}
		})
	}
}
