package domain_test

import (
	"errors"
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

func TestParseBreach(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    domain.Breach
		wantErr bool
	}{
		{"greater", "> 100", domain.Breach{Op: domain.OperatorGreater, Value: 100}, false},
		{"greater or equal", ">= 0.5", domain.Breach{Op: domain.OperatorGreaterEqual, Value: 0.5}, false},
		{"less", "< 3", domain.Breach{Op: domain.OperatorLess, Value: 3}, false},
		{"less or equal", "<= 0", domain.Breach{Op: domain.OperatorLessEqual, Value: 0}, false},
		{"backticks", "`> 0.01`", domain.Breach{Op: domain.OperatorGreater, Value: 0.01}, false},
		{"no space", ">0.01", domain.Breach{Op: domain.OperatorGreater, Value: 0.01}, false},
		{"negative", "< -2.5", domain.Breach{Op: domain.OperatorLess, Value: -2.5}, false},
		{"bad operator", "= 5", domain.Breach{}, true},
		{"missing operator", "5", domain.Breach{}, true},
		{"not a number", "> many", domain.Breach{}, true},
		{"missing value", ">=", domain.Breach{}, true},
		{"not a number value", "> NaN", domain.Breach{}, true},
		{"infinite value", "< -Inf", domain.Breach{}, true},
		{"empty", "", domain.Breach{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ParseBreach(tc.text)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidBreach) {
					t.Fatalf("err = %v, want ErrInvalidBreach", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != tc.want {
				t.Errorf("breach = %+v, want %+v", got, tc.want)
			}
		})
	}
}
