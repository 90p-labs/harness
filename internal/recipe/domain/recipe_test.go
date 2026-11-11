package domain_test

import (
	"errors"
	"testing"

	"github.com/90p-labs/harness/internal/recipe/domain"
)

func TestRecipeValidate(t *testing.T) {
	cases := []struct {
		name   string
		recipe string
		files  []string
		want   error
	}{
		{name: "a one word name", recipe: "go", files: []string{"a"}},
		{name: "a name with digits", recipe: "node20", files: []string{"a"}},
		{name: "a name with hyphens", recipe: "node-ddd-api", files: []string{"a"}},
		{name: "nested paths", recipe: "go", files: []string{".harness.yaml", "lint/architecture/pack.yaml"}},
		{name: "a dot folder that is not git", recipe: "go", files: []string{".harness/plugins/x.sh", ".github/ci.yaml"}},
		{name: "a file that starts with .git", recipe: "go", files: []string{".gitignore", ".gitattributes"}},

		{name: "an empty name", recipe: "", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "an upper case name", recipe: "Go", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a leading digit", recipe: "1go", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a leading hyphen", recipe: "-go", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a trailing hyphen", recipe: "go-", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a double hyphen", recipe: "a--b", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "an underscore", recipe: "a_b", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a slash", recipe: "a/b", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a parent name", recipe: "..", files: []string{"a"}, want: domain.ErrInvalidName},
		{name: "a trailing newline", recipe: "go\n", files: []string{"a"}, want: domain.ErrInvalidName},

		{name: "a nil file list", recipe: "go", want: domain.ErrNoFiles},
		{name: "an empty file list", recipe: "go", files: []string{}, want: domain.ErrNoFiles},
		{name: "an absolute path", recipe: "go", files: []string{"/etc/passwd"}, want: domain.ErrBadPath},
		{name: "a parent path", recipe: "go", files: []string{"../x"}, want: domain.ErrBadPath},
		{name: "a parent part", recipe: "go", files: []string{"a/../../x"}, want: domain.ErrBadPath},
		{name: "only a parent", recipe: "go", files: []string{".."}, want: domain.ErrBadPath},
		{name: "a git folder file", recipe: "go", files: []string{".git/config"}, want: domain.ErrBadPath},
		{name: "a git folder", recipe: "go", files: []string{".git"}, want: domain.ErrBadPath},
		{name: "a nested git folder", recipe: "go", files: []string{".git/hooks/pre-commit"}, want: domain.ErrBadPath},
		{name: "an empty path", recipe: "go", files: []string{""}, want: domain.ErrBadPath},
		{name: "a dot path", recipe: "go", files: []string{"."}, want: domain.ErrBadPath},
		{name: "a dot part", recipe: "go", files: []string{"./a"}, want: domain.ErrBadPath},
		{name: "a double slash", recipe: "go", files: []string{"a//b"}, want: domain.ErrBadPath},
		{name: "a trailing slash", recipe: "go", files: []string{"a/"}, want: domain.ErrBadPath},
		{name: "a backslash", recipe: "go", files: []string{`a\b`}, want: domain.ErrBadPath},
		{name: "a duplicate path", recipe: "go", files: []string{"a", "b", "a"}, want: domain.ErrDuplicatePath},
		{name: "an upper case git folder", recipe: "go", files: []string{".GIT/hooks/pre-commit"}, want: domain.ErrBadPath},
		{name: "a mixed case git folder", recipe: "go", files: []string{".Git"}, want: domain.ErrBadPath},
		{name: "a parent folder listed first", recipe: "go", files: []string{"a", "a/b"}, want: domain.ErrPathIsFolder},
		{name: "a parent folder listed last", recipe: "go", files: []string{"a/b/c", "x", "a/b"}, want: domain.ErrPathIsFolder},
		{name: "a sibling with the same prefix", recipe: "go", files: []string{"a", "ab", "a.b"}},
		{name: "a recipe.yaml", recipe: "go", files: []string{"recipe.yaml"}, want: domain.ErrNeverShips},
		{name: "a nested guide", recipe: "go", files: []string{"docs/guide.md"}, want: domain.ErrNeverShips},
		{name: "a readme", recipe: "go", files: []string{"README.md"}, want: domain.ErrNeverShips},
		{name: "a deep readme", recipe: "go", files: []string{"a/b/README.md"}, want: domain.ErrNeverShips},
		{name: "a go test file", recipe: "go", files: []string{"pkg/x_test.go"}, want: domain.ErrNeverShips},
		{name: "a file named like a guide folder", recipe: "go", files: []string{"guide.md.bak", "readme.md", "x_test.go.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recipe := domain.Recipe{Name: domain.Name(tc.recipe), Summary: "s", Files: tc.files}
			err := recipe.Validate()
			if !errors.Is(err, tc.want) {
				t.Errorf("Validate() = %v, want %v", err, tc.want)
			}
			if tc.want == nil && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestNameString(t *testing.T) {
	if got := domain.Name("go").String(); got != "go" {
		t.Errorf("String() = %q, want go", got)
	}
}

func TestActions(t *testing.T) {
	cases := map[domain.Action]string{domain.Wrote: "wrote", domain.Kept: "kept", domain.Differs: "differs"}
	for action, want := range cases {
		if string(action) != want {
			t.Errorf("action = %q, want %q", action, want)
		}
	}
}
