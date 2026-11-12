package domain

import (
	"errors"
	"strings"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	emDash     = "—"
	emoji      = "\U0001F600"
	arrow      = "→"
	testWork   = "/work/repo"
	defaultTip = "main"
)

var errNoHead = errors.New("no commit yet")

type ruleCase struct {
	name string
	call ToolCall
	rc   *RuleContext
	deny bool
	step string
}

func bash(command string) ToolCall {
	return ToolCall{Kind: shareddomain.ToolKindShell, Command: command}
}

func readOf(kind shareddomain.ToolKind, path string) ToolCall {
	return ToolCall{Kind: kind, Paths: []string{path}}
}

func editOf(path, text string) ToolCall {
	return ToolCall{Kind: shareddomain.ToolKindEdit, Paths: []string{path}, NewText: text}
}

func agentOf(agentType, model string) ToolCall {
	return ToolCall{Kind: shareddomain.ToolKindAgent, AgentType: agentType, Model: model}
}

func baseContext() RuleContext {
	return RuleContext{
		Cwd:           testWork,
		Worktree:      testWork,
		DefaultBranch: func() (string, error) { return defaultTip, nil },
		AgentModels:   map[string][]string{"explorer": {"haiku"}, "worker": {"sonnet", "haiku"}},
		CommitChecked: func() (bool, error) { return true, nil },
	}
}

func TestAgentModelEmpty(t *testing.T) {
	cases := []struct {
		name     string
		model    string
		optional bool
		deny     bool
	}{
		{"empty and optional", "", true, false},
		{"empty and required", "", false, true},
		{"wrong and optional", "opus", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := baseContext()
			rc.ModelOptional = tc.optional
			_, denied := denyAgentModel(agentOf("worker", tc.model), rc)
			if denied != tc.deny {
				t.Errorf("denied = %v, want %v", denied, tc.deny)
			}
		})
	}
}

func commitCheck(checked bool, err error) *RuleContext {
	rc := baseContext()
	rc.CommitChecked = func() (bool, error) { return checked, err }
	return &rc
}

func focusCases() []ruleCase {
	var cases []ruleCase
	for _, marker := range focusSkipMarkers {
		text := "  " + marker + "x)"
		cases = append(cases,
			ruleCase{name: "go test " + marker, call: editOf("a/b_test.go", text), deny: true, step: "focus"},
			ruleCase{name: "spec " + marker, call: editOf("web/a.spec.ts", text), deny: true, step: "focus"},
			ruleCase{name: "non test " + marker, call: editOf("a/b.go", text)},
		)
	}
	return append(cases,
		ruleCase{name: "word before fit", call: editOf("a/b_test.go", "pro"+focusSkipMarkers[1]+"x)")},
		ruleCase{name: "plain test edit", call: editOf("a/b_test.go", "t.Run(x)")},
		ruleCase{name: "patch of a test file", call: ToolCall{
			Kind:    shareddomain.ToolKindOther,
			Paths:   []string{"a/b.test.js"},
			NewText: focusSkipMarkers[0] + "x)",
		}, deny: true, step: "focus"},
	)
}

func ruleCases() map[string][]ruleCase {
	noDefault := baseContext()
	noDefault.DefaultBranch = nil
	failingDefault := baseContext()
	failingDefault.DefaultBranch = func() (string, error) { return "", errors.New("no remote") }
	unreadable := baseContext()
	unreadable.TextErr = errors.New("open msg.txt: no such file")
	withText := func(text string) ToolCall {
		call := bash("git commit -F msg.txt")
		call.Texts = [][]string{{text}}
		return call
	}

	return map[string][]ruleCase{
		RulePRNoMerge: {
			{name: "pr merge", call: bash("gh pr merge 12 --squash"), deny: true, step: "user"},
			{name: "api merge path", call: bash("gh api -X PUT repos/o/r/pulls/12/merge"), deny: true, step: "user"},
			{name: "env assignment before gh", call: bash("GH_TOKEN=x gh pr merge 12"), deny: true, step: "user"},
			{name: "pr view", call: bash("gh pr view 12")},
			{name: "api comments path", call: bash("gh api repos/o/r/pulls/12/comments")},
		},
		RulePRDraftOnly: {
			{name: "create without draft", call: bash("gh pr create --title t"), deny: true, step: "--draft"},
			{name: "ready", call: bash("gh pr ready 3"), deny: true, step: "user"},
			{name: "create draft", call: bash("gh pr create --draft --title t")},
			{name: "create short draft", call: bash("gh pr create -d -t t")},
			{name: "title that starts with a dash", call: bash("gh pr create --title '- add docs'"), deny: true, step: "--draft"},
			{name: "pr list", call: bash("gh pr list")},
		},
		RulePRBodyShape: {
			{name: "no body", call: bash("gh pr create --draft --title t")},
			{name: "inline body without sections", call: bash("gh pr edit 3 --body 'fix it'"), deny: true, step: "## How"},
			{name: "other command", call: bash("gh pr view 3 --body x")},
		},
		RuleGitNoForce: {
			{name: "long flag", call: bash("git push --force origin feat"), deny: true, step: "--force-with-lease"},
			{name: "short flag", call: bash("git push -f origin feat"), deny: true, step: "--force-with-lease"},
			{name: "short cluster", call: bash("git push -fu origin feat"), deny: true, step: "--force-with-lease"},
			{name: "plus refspec", call: bash("git push origin +feat"), deny: true, step: "--force-with-lease"},
			{name: "git directory option", call: bash("git -C /work/repo push --force origin feat"), deny: true, step: "--force-with-lease"},
			{name: "env assignment", call: bash("GIT_TRACE=1 git push -f origin feat"), deny: true, step: "--force-with-lease"},
			{name: "env launcher", call: bash("env -u HOME A=1 git push origin +feat"), deny: true, step: "--force-with-lease"},
			{name: "program path and pager option", call: bash("/usr/bin/git --no-pager push -f origin feat"), deny: true, step: "--force-with-lease"},
			{name: "git directory option on a plain push", call: bash("git -C /work/repo push origin feat")},
			{name: "with lease", call: bash("git push --force-with-lease origin feat")},
			{name: "if includes", call: bash("git push --force-if-includes origin feat")},
			{name: "plain push", call: bash("git push origin feat")},
			{name: "force on another command", call: bash("git branch -f feat")},
		},
		RuleGitNoPushDefault: {
			{name: "explicit default", call: bash("git push origin main"), deny: true, step: "pull request"},
			{name: "head to default", call: bash("git push origin HEAD:main"), deny: true, step: "pull request"},
			{name: "full ref", call: bash("git push origin HEAD:refs/heads/main"), deny: true, step: "pull request"},
			{name: "git config option", call: bash("git -c push.default=current push origin main"), deny: true, step: "pull request"},
			{name: "delete of the default", call: bash("git push origin --delete main"), deny: true, step: "pull request"},
			{name: "short delete of the default", call: bash("git push --delete origin -d main"), deny: true, step: "pull request"},
			{name: "colon delete of the default", call: bash("git push origin :main"), deny: true, step: "pull request"},
			{name: "default error", call: bash("git push origin feat"), rc: &failingDefault, deny: true, step: "set git.default_branch"},
			{name: "feature branch", call: bash("git push origin feat")},
			{name: "default as source", call: bash("git push origin main:feat")},
			{name: "bare push", call: bash("git push"), rc: &noDefault},
			{name: "remote only", call: bash("git push -u origin"), rc: &noDefault},
			{name: "default branch name in another command", call: bash("git checkout main")},
		},
		RuleGitNoVerifySkip: {
			{name: "commit no verify", call: bash("git commit --no-verify -m x"), deny: true, step: "hooks"},
			{name: "push no verify", call: bash("git push --no-verify"), deny: true, step: "hooks"},
			{name: "commit short", call: bash("git commit -n -m x"), deny: true, step: "hooks"},
			{name: "commit cluster", call: bash("git commit -an -m x"), deny: true, step: "hooks"},
			{name: "plain commit", call: bash("git commit -m x")},
			{name: "message that starts with a dash", call: bash("git commit -m feat -m '- fix login'")},
			{name: "push dry run", call: bash("git push -n origin feat")},
			{name: "log count", call: bash("git log -n 3")},
		},
		RuleSecretsRead: {
			{name: "read env", call: readOf(shareddomain.ToolKindRead, "/work/repo/.env"), deny: true, step: ".env.example"},
			{name: "grep pem", call: readOf(shareddomain.ToolKindSearch, "/work/repo/ca.pem"), deny: true, step: ".env.example"},
			{name: "cat env variant", call: bash("cat .env.local"), deny: true, step: ".env.example"},
			{name: "cat key", call: bash("cat certs/server.key"), deny: true, step: ".env.example"},
			{name: "cat ssh key", call: bash("cat ~/.ssh/id_rsa.pub"), deny: true, step: ".env.example"},
			{name: "redirect from env", call: bash("echo x > .env"), deny: true, step: ".env.example"},
			{name: "input redirect from env", call: bash("cat < .env"), deny: true, step: ".env.example"},
			{name: "search a secret file", call: bash("grep KEY .env.local"), deny: true, step: ".env.example"},
			{name: "search pattern", call: bash(`grep -rn '\.pem|\.key' internal`)},
			{name: "search pattern flag", call: bash("rg -e id_rsa docs")},
			{name: "include flag", call: bash("grep -r --include=*.key x src")},
			{name: "echo into gitignore", call: bash("echo .env.local >> .gitignore")},
			{name: "env example", call: readOf(shareddomain.ToolKindRead, "/work/repo/.env.example")},
			{name: "source file", call: readOf(shareddomain.ToolKindRead, "/work/repo/main.go")},
			{name: "bash readme", call: bash("cat README.md")},
			{name: "name that holds env", call: bash("cat environment.go")},
			{name: "keyboard file", call: bash("cat keyboard.go")},
			{name: "bare printenv", call: bash("printenv"), deny: true, step: ".env.example"},
			{name: "bare env", call: bash("env"), deny: true, step: ".env.example"},
			{name: "proc environ", call: bash("cat /proc/self/environ"), deny: true, step: ".env.example"},
			{name: "proc environ read", call: readOf(shareddomain.ToolKindRead, "/proc/1/environ"), deny: true, step: ".env.example"},
			{name: "dump in a substitution", call: bash("echo $(printenv)"), deny: true, step: ".env.example"},
			{name: "dump in bash -c", call: bash("bash -c 'export -p'"), deny: true, step: ".env.example"},
			{name: "secret file in a substitution", call: bash(`echo "$(cat .env)"`), deny: true, step: ".env.example"},
			{name: "env with a command", call: bash("env FOO=bar cmd")},
			{name: "set with options", call: bash("set -euo pipefail")},
			{name: "command lookup", call: bash("command -v printenv")},
		},
		RuleTestNoFocusSkip: focusCases(),
		RuleShellWriteSource: {
			{name: "redirect", call: bash("echo x > main.go"), deny: true, step: "Edit or Write"},
			{name: "append", call: bash("echo x >> docs/a.md"), deny: true, step: "Edit or Write"},
			{name: "tee", call: bash("echo x | tee internal/a.go"), deny: true, step: "Edit or Write"},
			{name: "sed in place with empty suffix", call: bash("sed -i '' s/a/b/ a.go"), deny: true, step: "Edit or Write"},
			{name: "sed in place", call: bash("sed -i s/a/b/ web/a.ts"), deny: true, step: "Edit or Write"},
			{name: "perl in place", call: bash("perl -pi -e s/a/b/ a.py"), deny: true, step: "Edit or Write"},
			{name: "cp into source", call: bash("cp /tmp/x.go main.go"), deny: true, step: "Edit or Write"},
			{name: "mv into source", call: bash("mv /tmp/x docs/main.md"), deny: true, step: "Edit or Write"},
			{name: "cp into a directory", call: bash("cp /tmp/new.go internal/"), deny: true, step: "Edit or Write"},
			{name: "cp into the worktree root", call: bash("cp -r /tmp/a.go /tmp/b.txt ."), deny: true, step: "Edit or Write"},
			{name: "cp to a scratch directory", call: bash("cp main.go /tmp/")},
			{name: "absolute path in worktree", call: bash("echo x > /work/repo/a.yaml"), deny: true, step: "Edit or Write"},
			{name: "scratchpad", call: bash("echo x > /tmp/scratch.go")},
			{name: "parent of worktree", call: bash("echo x > ../other/a.go")},
			{name: "log file", call: bash("echo x > out.log")},
			{name: "run state", call: bash("echo x > .kodify/runs/a.md")},
			{name: "harness state", call: bash("echo x > .harness/a.json")},
			{name: "copy from source", call: bash("cp main.go /tmp/main.go")},
			{name: "sed print", call: bash("sed s/a/b/ a.go")},
			{name: "tee to scratchpad", call: bash("cat a.go | tee /tmp/a.go")},
			{name: "dev null", call: bash("go test ./... > /dev/null")},
		},
		RuleWritingStyle: {
			{name: "em dash in message", call: bash("git commit -m 'fix " + emDash + " it'"), deny: true, step: "commas"},
			{name: "emoji in title", call: bash("gh pr create --draft --title 'ship " + emoji + "'"), deny: true, step: "commas"},
			{name: "co author in second message", call: bash("git commit -m fix -m 'Co-Authored-By: A <a@b.c>'"), deny: true, step: "Co-Authored-By"},
			{name: "co author lower case", call: bash("git commit -m 'co-authored-by: A'"), deny: true, step: "Co-Authored-By"},
			{name: "em dash in heredoc", call: bash("git commit -F - <<EOF\nfix " + emDash + " it\nEOF"), deny: true, step: "commas"},
			{name: "em dash in body", call: bash("gh pr comment 3 --body 'a " + emDash + " b'"), deny: true, step: "commas"},
			{name: "em dash in body file text", call: withText("a " + emDash + " b"), deny: true, step: "commas"},
			{name: "em dash in a heredoc substitution with quotes", call: bash("git commit -m \"$(cat <<'EOF'\nfeat: the \"a b\" thing " + emDash + " x\nEOF\n)\""), deny: true, step: "commas"},
			{name: "message file not readable", call: bash("git commit -F msg.txt"), rc: &unreadable, deny: true, step: "pass a literal absolute path"},
			{name: "read error on an inline message", call: bash("git commit -m 'fix it'"), rc: &unreadable},
			{name: "plain message", call: bash("git commit -m 'fix it'")},
			{name: "arrow and accent", call: bash("git commit -m 'café " + arrow + " ok'")},
			{name: "em dash in another command", call: bash("echo " + emDash)},
			{name: "co author word in prose", call: bash("git commit -m 'drop the co-authored trailer'")},
		},
		RuleCommitsChecked: {
			{name: "push without a check", call: bash("git push origin feat/x"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "deny names the way out with no gates", call: bash("git push origin feat/x"), rc: commitCheck(false, nil), deny: true, step: "guard.disabled"},
			{name: "pr create without a check", call: bash("gh pr create --draft"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "push after a launcher", call: bash("env FOO=1 git -C sub push"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "push without a head", call: bash("git push"), rc: commitCheck(false, errNoHead), deny: true, step: "commit, then run harness verify --commits"},
			{name: "push with a check", call: bash("git push origin feat/x"), rc: commitCheck(true, nil)},
			{name: "pr create with a check", call: bash("gh pr create --draft"), rc: commitCheck(true, nil)},
			{name: "other commands never ask", call: bash("git status && gh pr view 3"), rc: commitCheck(false, errNoHead)},
			{name: "delete flag", call: bash("git push origin --delete feat/x feat/y"), rc: commitCheck(false, nil)},
			{name: "short delete flag", call: bash("git push origin -d feat/x"), rc: commitCheck(false, nil)},
			{name: "delete flag before the remote", call: bash("git push --delete origin feat/x"), rc: commitCheck(false, nil)},
			{name: "colon delete", call: bash("git push origin :feat/x :refs/heads/feat/y"), rc: commitCheck(false, nil)},
			{name: "delete never reads the check", call: bash("git push origin :feat/x"), rc: commitCheck(false, errNoHead)},
			{name: "delete and a normal ref", call: bash("git push origin feat/y :feat/x"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "delete then a normal push", call: bash("git push origin -d feat/x && git push origin feat/y"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "lone colon pushes the matching branches", call: bash("git push origin :"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "delete flag with no ref", call: bash("git push origin --delete"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
			{name: "push option d is not a delete", call: bash("git push -od origin feat/x"), rc: commitCheck(false, nil), deny: true, step: "harness verify --commits"},
		},
		RuleApprovalNoForge: approvalNoForgeCases(),
		RuleAgentModelRouting: {
			{name: "explorer without model", call: agentOf("Explore", ""), deny: true, step: "haiku"},
			{name: "90p explorer with another model", call: agentOf("90p:explorer", "sonnet"), deny: true, step: "haiku"},
			{name: "worker with an unlisted model", call: agentOf("kodify:worker", "opus"), deny: true, step: "one of sonnet, haiku"},
			{name: "worker without model", call: agentOf("worker", ""), deny: true, step: "one of sonnet, haiku"},
			{name: "worker with the second listed model", call: agentOf("worker", "haiku")},
			{name: "explorer model", call: agentOf("explore", "haiku")},
			{name: "verifier without model", call: agentOf("90p:verifier", ""), deny: true, step: "haiku"},
			{name: "verifier on the explorer model", call: agentOf("90p:verifier", "haiku")},
			{name: "claude agent with an unlisted model", call: agentOf("claude", "opus"), deny: true, step: "one of sonnet, haiku"},
			{name: "general purpose on a worker model", call: agentOf("general-purpose", "sonnet")},
			{name: "prefixed upper case worker", call: agentOf("kodify:Worker", "sonnet")},
			{name: "unknown type", call: agentOf("Plan", "")},
			{name: "role without model", call: agentOf("code-reviewer", "")},
		},
	}
}

func TestRuleMatches(t *testing.T) {
	rules := Rules()
	cases := ruleCases()
	if len(rules) != len(cases) {
		t.Fatalf("Rules() has %d rules, the table has %d", len(rules), len(cases))
	}
	for _, rule := range rules {
		table, ok := cases[rule.ID]
		if !ok {
			t.Fatalf("rule %s has no cases", rule.ID)
		}
		for _, c := range table {
			t.Run(rule.ID+"/"+c.name, func(t *testing.T) {
				rc := baseContext()
				if c.rc != nil {
					rc = *c.rc
				}
				reason, denied := rule.Denies(c.call, rc)
				if denied != c.deny {
					t.Fatalf("deny = %v (%q), want %v", denied, reason, c.deny)
				}
				if denied && !strings.Contains(reason, c.step) {
					t.Fatalf("reason %q does not name %q", reason, c.step)
				}
				if denied && strings.HasPrefix(reason, "guard.") {
					t.Fatalf("reason %q carries a prefix", reason)
				}
			})
		}
	}
}

func ruleByID(t *testing.T, id string) Rule {
	t.Helper()
	for _, rule := range Rules() {
		if rule.ID == id {
			return rule
		}
	}
	t.Fatalf("no rule %s", id)
	return Rule{}
}

func TestPRBodyShape(t *testing.T) {
	const marked = "```mermaid\nstateDiagram-v2\n    A --> B: go (new)\n```"
	const unmarked = "```mermaid\nstateDiagram-v2\n    A --> B\n```"
	sections := "Bottom line: the guard checks X.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort."
	long := strings.TrimSpace(strings.Repeat("word ", 41))
	create := bash("gh pr create --draft --title t --body-file body.md")
	withBody := func(body string) ToolCall {
		call := create
		call.Texts = [][]string{{body}}
		return call
	}

	cases := []struct {
		name string
		call ToolCall
		rc   RuleContext
		deny bool
		want string
	}{
		{name: "good body", call: withBody(sections + "\n\n" + marked)},
		{name: "good inline body", call: bash("gh pr edit 3 --body '" + strings.ReplaceAll(sections, "'", "") + "'")},
		{name: "good heredoc body", call: bash("gh pr create --draft -F - <<EOF\n" + sections + "\nEOF")},
		{name: "heredoc without text", call: bash("gh pr create --draft --body-file -")},
		{name: "missing section", call: withBody("Bottom line: x.\n\n## Why\nBecause.\n\n## What\n- one"), deny: true, want: "the body has no ## How line"},
		{name: "old sections", call: withBody("Bottom line: x.\n\n## Summary\nShort.\n\n## What\n- one\n\n## Why\nBecause."), deny: true, want: "the body has no ## How line"},
		{name: "what before why", call: withBody("Bottom line: x.\n\n## What\n- one\n\n## Why\nBecause.\n\n## How\nShort."), deny: true, want: "## What is before ## Why"},
		{name: "how before what", call: withBody("Bottom line: x.\n\n## Why\nBecause.\n\n## How\nShort.\n\n## What\n- one"), deny: true, want: "## How is before ## What"},
		{name: "plan diagram copied as it is", call: withBody(sections + "\n```mermaid\nflowchart TD\n" +
			"    classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e\n" +
			"    a[plan] --> b[skill]\n    class b new\n```")},
		{name: "heading in a code fence", call: withBody("Bottom line: x.\n\n## Why\nBecause.\n```\n## How\n```\n## What\n- one\n\n## How\nShort.")},
		{name: "heading only in a code fence", call: withBody("Bottom line: x.\n\n## Why\nBecause.\n\n## What\n- one\n```\n## How\n```"), deny: true, want: "the body has no ## How line"},
		{name: "no bottom line", call: withBody("## Why\nBecause.\n\n## What\n- one\n\n## How\nShort."), deny: true, want: "no bottom line"},
		{name: "bottom line only in a fence", call: withBody("```\nBottom line: x.\n```\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort."), deny: true, want: "no bottom line"},
		{name: "heading only above Why", call: withBody("# Title\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort."), deny: true, want: "no bottom line"},
		{name: "issue reference as bottom line", call: withBody("#42 is fixed.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort.")},
		{name: "41 word line", call: withBody(sections + "\n\n" + long), deny: true, want: "40 words"},
		{name: "long line in a code fence", call: withBody(sections + "\n```\n" + long + "\n```")},
		{name: "40 word line", call: withBody(sections + "\n\n" + strings.TrimSpace(strings.Repeat("word ", 40)))},
		{name: "unmarked diagram", call: withBody(sections + "\n\n" + unmarked), deny: true, want: "no mark"},
		{name: "diagram of another kind", call: withBody(sections + "\n```mermaid\nsequenceDiagram\n    A->>B: hi\n```")},
		{name: "read error", call: create, rc: RuleContext{TextErr: errors.New("open body.md: no such file")}, deny: true, want: "cannot read the body file: open body.md: no such file"},
		{name: "read error on another command", call: bash("git commit -F msg.txt"), rc: RuleContext{TextErr: errors.New("x")}},
		{name: "read error on an inline body", call: bash("gh pr edit 3 --body '" + strings.ReplaceAll(sections, "'", "") + "'"), rc: RuleContext{TextErr: errors.New("x")}},
		{name: "file text of another command", call: ToolCall{
			Kind:    shareddomain.ToolKindShell,
			Command: "git commit -F msg.txt && gh pr edit 12 --add-label ci",
			Texts:   [][]string{{"fix it"}},
		}},
		{name: "heredoc substitution with quotes", call: bash("gh pr create --draft --body \"$(cat <<'EOF'\n" +
			strings.Replace(sections, "Short.", `The "guard rule" denies.`, 1) + "\nEOF\n)\"")},
	}
	rule := ruleByID(t, RulePRBodyShape)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, denied := rule.Denies(c.call, c.rc)
			if denied != c.deny {
				t.Fatalf("deny = %v (%q), want %v", denied, reason, c.deny)
			}
			if denied && !strings.Contains(reason, c.want) {
				t.Fatalf("reason %q does not name %q", reason, c.want)
			}
		})
	}
}

func TestPRBodyShapeDiagramNextStep(t *testing.T) {
	call := bash("gh pr create --draft --body-file body.md")
	call.Texts = [][]string{{"Bottom line: x.\n## Why\nx\n## What\nx\n## How\nx\n```mermaid\nflowchart TD\n    a --> b\n```"}}
	reason, denied := ruleByID(t, RulePRBodyShape).Denies(call, RuleContext{})
	if !denied || !strings.HasSuffix(reason, "draw it with `harness diagram`") {
		t.Fatalf("reason = %q, denied = %v", reason, denied)
	}
}
