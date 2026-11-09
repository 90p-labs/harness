package domain_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/guard/domain"
)

func TestShellCommands(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []domain.Command
	}{
		{
			name: "plain words",
			line: "git push origin main",
			want: []domain.Command{{Args: []string{"git", "push", "origin", "main"}}},
		},
		{
			name: "empty line",
			line: "   ",
			want: nil,
		},
		{
			name: "double quoted text stays whole",
			line: `gh pr create --title "feat: add guard" --body "one && two; three | four"`,
			want: []domain.Command{{Args: []string{"gh", "pr", "create", "--title", "feat: add guard", "--body", "one && two; three | four"}}},
		},
		{
			name: "single quoted text keeps backslash and dollar",
			line: `echo 'a\nb $HOME'`,
			want: []domain.Command{{Args: []string{"echo", `a\nb $HOME`}}},
		},
		{
			name: "escaped quote inside double quotes",
			line: `echo "say \"hi\""`,
			want: []domain.Command{{Args: []string{"echo", `say "hi"`}}},
		},
		{
			name: "empty quoted argument is kept",
			line: `git commit -m ""`,
			want: []domain.Command{{Args: []string{"git", "commit", "-m", ""}}},
		},
		{
			name: "adjacent quoted parts join",
			line: `echo a"b c"'d e'f`,
			want: []domain.Command{{Args: []string{"echo", "ab cd ef"}}},
		},
		{
			name: "split at and, or, semicolon, pipe",
			line: "a && b || c ; d | e",
			want: []domain.Command{
				{Args: []string{"a"}}, {Args: []string{"b"}}, {Args: []string{"c"}}, {Args: []string{"d"}}, {Args: []string{"e"}},
			},
		},
		{
			name: "split at newline",
			line: "a\nb",
			want: []domain.Command{{Args: []string{"a"}}, {Args: []string{"b"}}},
		},
		{
			name: "newline inside quotes does not split",
			line: "echo \"a\nb\"",
			want: []domain.Command{{Args: []string{"echo", "a\nb"}}},
		},
		{
			name: "line continuation joins lines",
			line: "git push \\\n origin",
			want: []domain.Command{{Args: []string{"git", "push", "origin"}}},
		},
		{
			name: "redirect targets",
			line: "echo hi > a.go && echo yo >> b.go 2> c.log",
			want: []domain.Command{
				{Args: []string{"echo", "hi"}, Redirects: []string{"a.go"}},
				{Args: []string{"echo", "yo"}, Redirects: []string{"b.go", "c.log"}},
			},
		},
		{
			name: "redirect without spaces and quoted target",
			line: `echo hi >a.go; echo yo >"b c.go"`,
			want: []domain.Command{
				{Args: []string{"echo", "hi"}, Redirects: []string{"a.go"}},
				{Args: []string{"echo", "yo"}, Redirects: []string{"b c.go"}},
			},
		},
		{
			name: "fd duplication is not a target",
			line: "make 2>&1 | tee out.log",
			want: []domain.Command{
				{Args: []string{"make"}},
				{Args: []string{"tee", "out.log"}},
			},
		},
		{
			name: "input redirect is an input, not a target",
			line: "wc -l < in.txt > out.txt",
			want: []domain.Command{{Args: []string{"wc", "-l"}, Redirects: []string{"out.txt"}, Inputs: []string{"in.txt"}}},
		},
		{
			name: "heredoc in a quoted command substitution keeps its quotes and parentheses",
			line: "gh pr create --body \"$(cat <<'EOF'\nsays \"a b\" (1)\nEOF\n)\" && echo done",
			want: []domain.Command{
				{Args: []string{"gh", "pr", "create", "--body", "$(cat <<'EOF'\nsays \"a b\" (1)\nEOF\n)"}},
				{Args: []string{"echo", "done"}},
			},
		},
		{
			name: "quoted parenthesis in a command substitution does not close it",
			line: `echo "$(printf '%s)' "a)b")" | wc`,
			want: []domain.Command{{Args: []string{"echo", `$(printf '%s)' "a)b")`}}, {Args: []string{"wc"}}},
		},
		{
			name: "heredoc body is not split",
			line: "cat <<EOF\nline one && two\n; three\nEOF\necho after",
			want: []domain.Command{
				{Args: []string{"cat"}, Heredoc: "line one && two\n; three"},
				{Args: []string{"echo", "after"}},
			},
		},
		{
			name: "quoted heredoc delimiter",
			line: "git commit -F - <<'MSG'\nfix: a\n\nbody $x\nMSG",
			want: []domain.Command{{Args: []string{"git", "commit", "-F", "-"}, Heredoc: "fix: a\n\nbody $x"}},
		},
		{
			name: "dash heredoc strips leading tabs of the terminator",
			line: "cat <<-END\n\tindented\n\tEND\nls",
			want: []domain.Command{
				{Args: []string{"cat"}, Heredoc: "\tindented"},
				{Args: []string{"ls"}},
			},
		},
		{
			name: "heredoc with redirect on the same line",
			line: "cat <<EOF > main.go\npackage main\nEOF",
			want: []domain.Command{{Args: []string{"cat"}, Redirects: []string{"main.go"}, Heredoc: "package main"}},
		},
		{
			name: "heredoc command followed by operator on the same line",
			line: "cat <<EOF && echo done\nbody\nEOF",
			want: []domain.Command{
				{Args: []string{"cat"}, Heredoc: "body"},
				{Args: []string{"echo", "done"}},
			},
		},
		{
			name: "unterminated heredoc takes the rest",
			line: "cat <<EOF\nbody\nmore",
			want: []domain.Command{{Args: []string{"cat"}, Heredoc: "body\nmore"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Commands(tc.line)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Commands(%q)\n got %#v\nwant %#v", tc.line, got, tc.want)
			}
		})
	}
}
