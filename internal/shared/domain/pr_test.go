package domain_test

import (
	"errors"
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    domain.PR
		wantErr bool
	}{
		{"valid", "https://github.com/jorge07/harness/pull/9", domain.PR{Owner: "jorge07", Repo: "harness", Number: 9}, false},
		{"dotted repo", "https://github.com/acme/my.repo/pull/1234", domain.PR{Owner: "acme", Repo: "my.repo", Number: 1234}, false},
		{"issue url", "https://github.com/jorge07/harness/issues/9", domain.PR{}, true},
		{"trailing path", "https://github.com/jorge07/harness/pull/9/files", domain.PR{}, true},
		{"http", "http://github.com/jorge07/harness/pull/9", domain.PR{}, true},
		{"other host", "https://gitlab.com/jorge07/harness/pull/9", domain.PR{}, true},
		{"no number", "https://github.com/jorge07/harness/pull/", domain.PR{}, true},
		{"number too large", "https://github.com/jorge07/harness/pull/99999999999999999999", domain.PR{}, true},
		{"empty", "", domain.PR{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ParsePRURL(tc.url)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidPRURL) {
					t.Fatalf("err = %v, want ErrInvalidPRURL", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != tc.want {
				t.Errorf("pr = %+v, want %+v", got, tc.want)
			}
		})
	}
}
