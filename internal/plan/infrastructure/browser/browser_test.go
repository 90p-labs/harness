package browser_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/plan/infrastructure/browser"
)

const page = "/work/tree/.harness/runs/r1/plan.html"

var errRun = errors.New("no opener")

func TestOpener(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		runErr   error
		wantCall []string
		wantErr  error
	}{
		{name: "darwin runs open", goos: "darwin", wantCall: []string{"open", page}},
		{name: "linux runs xdg-open", goos: "linux", wantCall: []string{"xdg-open", page}},
		{name: "the env switch runs nothing", goos: "darwin", env: map[string]string{"HARNESS_NO_BROWSER": "1"}},
		{name: "a run error is returned", goos: "linux", runErr: errRun, wantCall: []string{"xdg-open", page}, wantErr: errRun},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var call []string
			opener := browser.Opener{
				GOOS:   tt.goos,
				Getenv: func(key string) string { return tt.env[key] },
				Run: func(_ context.Context, name string, args ...string) error {
					call = append([]string{name}, args...)
					return tt.runErr
				},
			}

			err := opener.Open(t.Context(), page)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(call, tt.wantCall) {
				t.Errorf("call = %v, want %v", call, tt.wantCall)
			}
		})
	}
}
