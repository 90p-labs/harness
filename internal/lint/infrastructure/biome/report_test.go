package biome_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/biome"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestBiomeReportMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/report.json")
	if err != nil {
		t.Fatal(err)
	}
	rules := []domain.Rule{
		{ID: "ddd/event-readonly", Engine: domain.EngineBiomeGrit, Severity: shareddomain.SeverityError},
		{ID: "next/no-img-element", Engine: domain.EngineBiomeRule, Severity: shareddomain.SeverityWarn, BiomeRule: "lint/performance/noImgElement"},
	}

	got, err := biome.MapReport(raw, rules)
	if err != nil {
		t.Fatalf("MapReport: %v", err)
	}

	want := []shareddomain.Finding{
		{Rule: "ddd/event-readonly", Severity: shareddomain.SeverityError, File: "src/x/a.event.ts", Line: 2, Message: "an event property must be readonly"},
		{Rule: "next/no-img-element", Severity: shareddomain.SeverityWarn, File: "src/x/p.tsx", Line: 1, Message: "Don't use <img> element."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %+v, want %+v", got, want)
	}
}

func TestBiomeReportRejectsGarbage(t *testing.T) {
	if _, err := biome.MapReport([]byte("not json"), nil); err == nil {
		t.Fatal("MapReport accepted a payload that is not JSON")
	}
}

func TestBiomeReportInternalError(t *testing.T) {
	raw := []byte(`{"diagnostics":[{"severity":"error","message":"No such file or directory (os error 2)","category":"internalError/io","location":{"path":"src/gone.ts"}}]}`)

	_, err := biome.MapReport(raw, nil)

	if err == nil || !strings.Contains(err.Error(), "src/gone.ts") {
		t.Fatalf("err = %v, want an error that names src/gone.ts", err)
	}
}
