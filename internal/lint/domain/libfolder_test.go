package domain_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
)

func TestNoLibFolder(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  []domain.Violation
	}{
		{name: "lib at depth one", files: []string{"src/lib/x.ts"}, want: []domain.Violation{{File: "src/lib"}}},
		{name: "lib in app", files: []string{"src/app/lib/x.ts"}, want: []domain.Violation{{File: "src/app/lib"}}},
		{
			name:  "one finding for each folder",
			files: []string{"src/features/lib/x.ts", "src/features/lib/y.css"},
			want:  []domain.Violation{{File: "src/features/lib"}},
		},
		{name: "library is not lib", files: []string{"src/library/x.ts"}},
		{name: "root lib is outside src", files: []string{"lib/x.ts"}},
		{name: "file named lib is not a folder", files: []string{"src/features/lib"}},
		{name: "shared folder", files: []string{"src/components/shared/utils.ts"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NoLibFolder(domain.Tree{Files: tc.files}, domain.Graph{}, domain.Settings{})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NoLibFolder = %+v, want %+v", got, tc.want)
			}
		})
	}
}
