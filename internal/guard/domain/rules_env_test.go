package domain

import (
	"slices"
	"strings"
	"testing"
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	wantDumpReason  = "the command prints the process environment, which can hold a secret"
	wantSecretStep  = ".env.example"
	heredocCommit   = "git commit -m \"$(cat <<'EOF'\nfix: stop calling `printenv` in tests\n\nBody names $(env) and `env` too.\nEOF\n)\""
	caseArmCommand  = "case $x in\nset)\n  echo a\n  ;;\nesac"
	backtickEscaped = "echo \\`env\\`"
)

func envVerdict(call ToolCall) (string, bool) {
	return denySecretsRead(call, baseContext())
}

func checkDenied(t *testing.T, call ToolCall, wantReason string) {
	t.Helper()
	reason, denied := envVerdict(call)
	if !denied {
		t.Fatalf("denied = false for %q, want true", call.Command)
	}
	if !strings.Contains(reason, wantSecretStep) {
		t.Errorf("reason = %q, want it to contain %q", reason, wantSecretStep)
	}
	if !strings.Contains(reason, wantReason) {
		t.Errorf("reason = %q, want it to contain %q", reason, wantReason)
	}
}

func commandCases(t *testing.T, commands []string, wantReason string) {
	t.Helper()
	for _, command := range commands {
		t.Run(command, func(t *testing.T) { checkDenied(t, bash(command), wantReason) })
	}
}

func TestSecretsReadEnvDump(t *testing.T) {
	commandCases(t, []string{
		"env",
		"env -0",
		"env -i",
		"env FOO=bar",
		"env -u FOO",
		"env -i FOO=bar",
		"/usr/bin/env",
		"sudo env",
		"sudo -u root env",
		"nohup env",
		"exec env",
		"command env",
		"FOO=bar env",
		"printenv",
		"printenv HOME",
		"printenv -0",
		"sudo printenv TOKEN",
		"export",
		"export -p",
		"set",
		"declare",
		"declare -x",
		"declare -p",
		"declare -p FOO",
		"declare -xp",
		"typeset",
		"typeset -p",
		"typeset -x",
		"! env",
		"time env",
		"time -p env",
		"{ env; }",
		"if true; then env; fi",
		"while env; do :; done",
		"f() { env; }",
		"f() env",
		"f(){ env; }",
		"env FOO=bar --",
		"env FOO=bar BAZ=1 --",
		"echo $'it\\'s'; env",
		"echo $'a\\tb'; env",
		"x=$'it\\'s' && env",
		"echo $'a\\'b' $'c\\'d'; printenv",
		"x=$(printf $'it\\'s'); env",
		"echo $(echo $'it\\'s'); env",
		"echo `printf $'a\\'b'`; env",
		"echo \"$(printf $'it\\'s')\"; env",
		"(echo $'it\\'s'); env",
		"env -S 'printenv HOME'",
		"env -S 'env -i'",
		"env -S printenv",
		"env -iS printenv",
		"env -S'printenv HOME'",
		"env --split-string 'printenv HOME'",
		"env --split-string='env -i'",
		"env -i -S 'printenv HOME'",
		"sudo env -S 'printenv'",
		"env -S 'FOO=1'",
		"env -S 'FOO=1' BAR=2",
		"env -S '-0'",
		"env -S '-S env'",
		"env --split-string=",
		"env -S ''",
		"env -S ' '",
		"env -i -S ''",
		"env FOO=1 env -S printenv",
		"env env -S 'printenv HOME'",
		"env -i env -S printenv",
		"sudo env -S env",
		"env -S '#' env",
		"env -P /usr/bin printenv",
		"env -P /bin -S 'printenv'",
		"env -vu HOME env",
		"env --split 'printenv HOME'",
		"env --split-str='printenv HOME'",
		"env -S 'sh -c env'",
		"env -S 'sh -c \"printenv HOME\"'",
		"env --s 'printenv HOME'",
		"env --un HOME printenv",
		"env --ch /tmp printenv",
		"env --a x printenv",
		"env --argv0 x printenv",
		"env --argv0=x printenv",
		"env -a x printenv",
		"env -ax printenv",
		"env -S 'printenv\\c'",
		"env -S 'printenv\\c HOME'",
		"env -S 'sh\\_-c\\_env'",
		"env -S \"printenv\\_HOME\"",
		"env -S \"printenv\vHOME\"",
		"env -S 'printenv\fHOME'",
		"env -S 'printenv\rHOME'",
		"env -S '\\printenv'",
		"env -S '${NOPE}'",
		"env -S '${NOPE} printenv'",
		"env -S '${X}printenv'",
		"declare +x",
		"typeset +x",
		"declare +p",
		"declare +x +r",
		"echo $'it\\'s'; env",
	}, wantDumpReason)
}

func TestSecretsReadProcEnviron(t *testing.T) {
	for _, kind := range []shareddomain.ToolKind{shareddomain.ToolKindRead, shareddomain.ToolKindSearch} {
		for _, path := range []string{"/proc/self/environ", "/proc/1234/environ", "/proc/thread-self/environ"} {
			t.Run(string(kind)+" "+path, func(t *testing.T) {
				checkDenied(t, readOf(kind, path), reasonSecret)
			})
		}
	}
	commandCases(t, []string{
		"cat /proc/self/environ",
		"cat /proc/1234/environ",
		"strings /proc/self/environ",
		"grep TOKEN /proc/self/environ",
		"tr '\\0' '\\n' < /proc/self/environ",
		"cat < /proc/42/environ",
		"cp /proc/self/environ /tmp/x",
	}, reasonSecret)
	for _, command := range []string{
		"cat /proc/self/status",
		"cat /proc/cpuinfo",
		"cat environ",
		"cat /proc/self/environment",
		"cat /tmp/proc/self/environ",
		"echo /proc/self/environ",
	} {
		t.Run("allowed "+command, func(t *testing.T) {
			if reason, denied := envVerdict(bash(command)); denied {
				t.Errorf("denied %q: %s", command, reason)
			}
		})
	}
	for _, kind := range []shareddomain.ToolKind{shareddomain.ToolKindRead, shareddomain.ToolKindSearch} {
		if reason, denied := envVerdict(readOf(kind, "/proc/self/status")); denied {
			t.Errorf("denied /proc/self/status: %s", reason)
		}
	}
}

func TestSecretsReadNestedEnvDump(t *testing.T) {
	commandCases(t, []string{
		"echo hi | env",
		"true && printenv",
		"echo a; env",
		"echo a\nenv",
		"false || printenv TOKEN",
		"sleep 1 & env",
		"(env)",
		"(cd x && env)",
		"( env )",
		"echo a; (env)",
		"{ (env); }",
		"echo $(env)",
		"echo $(printenv FOO)",
		"echo \"$(printenv FOO)\"",
		"echo \"$(env)\"",
		"x=$(env)",
		"echo $(env | sort)",
		"echo $(echo $(env))",
		"echo \"$(echo `env`)\"",
		"echo `env`",
		"echo `printenv FOO`",
		"echo \"`env`\"",
		"diff <(env) other.txt",
		"tee >(env)",
		"bash -c env",
		"bash -c 'env'",
		"sh -c 'printenv'",
		"zsh -c 'export -p'",
		"dash -c 'set'",
		"bash -lc env",
		"bash -o pipefail -c 'env'",
		"sudo bash -c env",
		"bash -c 'echo a; env'",
		"bash -c 'echo $(env)'",
		"bash -c \"bash -c 'env'\"",
		"echo \"it's fine\"; (env)",
		"echo 'a'; echo $(env)",
		"echo $((1+2)) $(env)",
		"cat <<'EOF'\nbody\nEOF\nenv",
		"echo $(cat <<'EOF'\nbody\nEOF\n; env)",
		"cat <<<x\n$(printenv)",
		"cat <<<x\nenv",
		"cat <<EOF\n$(env)\nEOF",
		"cat <<EOF\n$(printenv)\nEOF",
		"cat <<EOF\nsee `env`\nEOF",
		"cat <<-EOF\n\t$(env)\n\tEOF",
		"cat <<EOF\ndon't $(env)\nEOF",
		"cat <<EOF\n$(env)\nEOF\nenv",
		"gh pr create --body \"$(cat <<EOF\nsee `printenv`\nEOF\n)\"",
		"echo \"$(cat <<EOF\n$(env)\nEOF\n)\"",
		"echo $(cat <<EOF\n$(printenv)\nEOF\n)",
		"arr=(1 2); env",
		"f() { :; }; (env)",
		"echo hi # don't\necho $(printenv)",
		"# don't print it\necho `env`",
		"echo $(echo a # it's\necho $(env))",
		"echo ${#x} $(env)",
		"echo a#b $(env)",
		"echo \"# it's\" $(env)",
		"echo hi # note\n(env)",
		"printenv --help FOO",
		"printenv --version HOME",
		"echo $(date)#x $(env)",
		"echo <(date)#x $(env)",
		"echo >(date)#x $(env)",
		"echo `date`#x $(env)",
		"bash -eo pipefail -c env",
		"bash -euo pipefail -c env",
		"bash +O extglob -c env",
		"bash --rcfile f -c env",
		"bash --init-file f -c env",
		"bash --norc -c env",
		"bash -o pipefail -c env",
		"bash -c -- env",
		"bash -co pipefail env",
		"sh -c 'env'",
		"bash -ce env",
		"bash -ec -- env",
		"bash -oc pipefail env",
		"bash -Oc extglob env",
		"bash -c -x env",
		"bash -c -o pipefail env",
	}, wantDumpReason)
	commandCases(t, []string{
		"bash -c 'cat .env | head'",
		"echo \"$(cat .env)\"",
		"echo $(cat .env.local)",
		"(cat certs/server.key)",
		"echo `cat ~/.ssh/id_rsa`",
		"bash -c 'cat /proc/self/environ'",
		"echo \"$(cat /proc/self/environ)\"",
	}, reasonSecret)
}

func TestSecretsReadEnvAllowed(t *testing.T) {
	commands := []string{
		"env FOO=bar some-command",
		"env -u FOO cmd",
		"env -i FOO=bar cmd arg",
		"env FOO=bar -- cmd",
		"echo $'a\\tb'",
		"echo $'it\\'s' $'x'",
		"grep $'\\t' export file.go",
		"echo $'a' env",
		"echo $'it\\'s' env",
		"echo $'it\\'s' set",
		"ls $'a\\'b'/env",
		"printf $'%s\\'' printenv",
		"echo $'a\\'b' $'c\\'d' env",
		"git commit -m $'don\\'t' -m 'see; env'",
		"echo $'a\\'b' 'x; env'",
		"printf $'it\\'s %s\\n' 'a; env'",
		"FOO=bar",
		"FOO=bar cmd",
		"sudo -u root ls",
		"export FOO=bar",
		"export FOO",
		"export -n FOO",
		"set -euo pipefail",
		"set -x",
		"set -o",
		"set -- a b",
		"echo $HOME",
		"grep export file.go",
		"git log --grep export",
		"git log --grep env",
		"echo exported environment",
		"ls environment",
		"cat environment.go",
		"echo env",
		"echo printenv",
		"echo printenv | cat",
		"grep printenv README.md",
		"go env",
		"go env GOPATH",
		"command -v printenv",
		"command -V env",
		"command -v env",
		"declare -x FOO",
		"declare -x FOO=bar",
		"declare -f",
		"declare -F",
		"typeset -f",
		"env --help",
		"env --version",
		"printenv --help",
		"printenv --version",
		"set)",
		caseArmCommand,
		heredocCommit,
		"git commit -m x <<'EOF'\n$(printenv)\n`env`\nEOF",
		"cat <<'EOF'\n$(env)\nEOF",
		"cat <<\"EOF\"\n$(env)\n`env`\nEOF",
		"cat <<\\EOF\n$(env)\nEOF",
		"cat <<-'EOF'\n\t$(env)\n\tEOF",
		"echo \"$(cat <<'EOF'\nsee `printenv`\nEOF\n)\"",
		"cat <<EOF\nprintenv\nenv\ndon't\nEOF",
		"cat <<EOF\ndon't $(date) it's\nEOF",
		"cat <<EOF\n\\$(env) \\`env\\`\nEOF",
		"cat <<-EOF\n\tenv\n\tEOF",
		"cat <<<\"$HOME\"",
		"cat <<<x\necho hi",
		"grep '$(printenv)' notes.md",
		"echo '(env)'",
		"echo 'a `env` b'",
		"echo \"\\$(env)\"",
		"echo \\$(env)",
		backtickEscaped,
		"echo $((x+1))",
		"echo $(( (1+2)*3 ))",
		"printf '%s' \"$((1+2))\"",
		"echo \"it's fine\"",
		"echo \"it's (env) fine\"",
		"echo \"it's fine\" 'and (env)'",
		"echo \"a (env) b\"",
		"arr=(1 2)",
		"bash -c 'echo hi'",
		"bash -c 'go env'",
		"bash script.sh",
		"bash ./run.sh -c set",
		"sh script.sh -c env",
		"bash -- -c env",
		"bash -- -- -c env",
		"bash -x script.sh -c env",
		"bash -o -c env",
		"bash --norc script.sh -c env",
		"echo hi # don't $(env)\necho ok",
		"echo hi # it's (env)",
		"echo $(date)",
		"echo `date`",
		"(cd x && go build)",
		"cat .env.example",
		"env -S 'node --flag' script.js",
		"env -S 'FOO=1 make'",
		"env -S 'make ${X}'",
		"env -S 'FOO=${X} make'",
		"env -S \"'\\${X}' make\"",
		"env -S'FOO=1 make' all",
		"env --split-string 'node --flag' script.js",
		"env --split-string='FOO=1 make'",
		"env -S 'FOO=bar' some-command",
		"env -u FOO cmd",
		"env FOO=bar some-command",
		"env -P /usr/bin make",
		"env --unset FOO cmd",
		"env --ch /tmp make",
		"env --un FOO make",
		"env -a x make",
		"env --argv0 x make",
		"env --d make",
		"env --i make",
		"env -S 'FOO=1\\_make'",
		"env -S 'make\\c printenv'",
		"declare +x FOO",
		"typeset +x FOO",
		"echo $$'a\\'b' ; env; echo '",
		"",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			if reason, denied := envVerdict(bash(command)); denied {
				t.Errorf("denied %q: %s", command, reason)
			}
		})
	}
}

func TestSplitWordsEscapes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`a\cb c`, []string{"a"}},
		{`a\_b`, []string{"a", "b"}},
		{`"a\_b"`, []string{"a b"}},
		{`'a\_b'`, []string{`a\_b`}},
		{`a\nb`, []string{"a\nb"}},
		{`a\tb\rc\fd\ve`, []string{"a\tb\rc\fd\ve"}},
		{`a\xb`, []string{"axb"}},
		{`"a\cb"`, []string{"a"}},
		{`'a\\b\'c\nd'`, []string{`a\b'c\nd`}},
		{"a\vb\fc\rd", []string{"a", "b", "c", "d"}},
		{`''`, []string{""}},
		{`${NOPE}`, nil},
		{`${NOPE} printenv`, []string{"printenv"}},
		{`${X}printenv`, []string{"printenv"}},
		{`make ${X}`, []string{"make"}},
		{`FOO=${X} make`, []string{"FOO=", "make"}},
		{`"${X}"`, []string{""}},
		{`"a${X}b"`, []string{"ab"}},
		{`'${X}' make`, []string{"${X}", "make"}},
		{`\${X} make`, []string{"${X}", "make"}},
		{`${X`, []string{"${X"}},
		{`${a-b}`, []string{"${a-b}"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := splitWords(c.in); !slices.Equal(got, c.want) {
				t.Errorf("splitWords(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestResolveEnvLong(t *testing.T) {
	cases := map[string]string{
		"--s":                  "--split-string",
		"--split-string":       "--split-string",
		"--un":                 "--unset",
		"--ch":                 "--chdir",
		"--a":                  "--argv0",
		"--d":                  "--d",
		"--i":                  "--i",
		"--debug":              "--debug",
		"--zzz":                "--zzz",
		"--splitx":             "--splitx",
		"--ignore-environment": "--ignore-environment",
	}
	for in, want := range cases {
		if got := resolveEnvLong(in); got != want {
			t.Errorf("resolveEnvLong(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEnvValuedOptionsShareLaunchers checks the shared options come from launchers.
func TestEnvValuedOptionsShareLaunchers(t *testing.T) {
	want := append(slices.Clone(launchers[cmdEnv]), "-P", "-a", "--argv0", "-S", "--split-string")
	for _, option := range want {
		if !slices.Contains(envValuedOptions, option) {
			t.Errorf("envValuedOptions lacks %q", option)
		}
	}
	for _, letter := range "uCPSa" {
		if !strings.ContainsRune(envValuedLetters, letter) {
			t.Errorf("envValuedLetters = %q, lacks %q", envValuedLetters, letter)
		}
	}
}

// verdictWithin fails the test when the verdict for the line takes over 5 s.
// The budget is loose for a loaded runner. Deep nesting must still end well inside it.
func verdictWithin(t *testing.T, line string) (bool, string) {
	t.Helper()
	type result struct {
		reason string
		denied bool
	}
	done := make(chan result, 1)
	go func() {
		reason, denied := envVerdict(bash(line))
		done <- result{reason, denied}
	}()
	var r result
	select {
	case r = <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("no verdict within 5s for %d bytes", len(line))
	}
	return r.denied, r.reason
}

func TestSecretsReadANSICNestingIsFast(t *testing.T) {
	verdictWithin(t, strings.Repeat("echo $'\\'' $(", 30))
}

// wrapShellSubstitution wraps the line in bash -c with a quoted substitution, depth times.
func wrapShellSubstitution(line string, depth int) string {
	for range depth {
		line = "bash -c \"$(" + line + ")\""
	}
	return line
}

func TestSecretsReadQuotedShellNestingIsFast(t *testing.T) {
	for _, depth := range []int{22, 40} {
		line := wrapShellSubstitution("true", depth)
		if denied, reason := verdictWithin(t, line); denied {
			t.Errorf("depth %d denied: %s", depth, reason)
		}
	}
}

func TestSecretsReadQuotedShellNestingStillDenies(t *testing.T) {
	for _, depth := range []int{1, 2, 3, 5} {
		line := wrapShellSubstitution("env", depth)
		if denied, _ := verdictWithin(t, line); !denied {
			t.Errorf("depth %d: denied = false for %q, want true", depth, line)
		}
	}
}

func TestNeutralANSIC(t *testing.T) {
	changed := map[string]string{
		"echo $'it\\'s'":                   "echo $'it\\\\s'",
		"echo $(printf $'it\\'s')":         "echo $(printf $'it\\\\s')",
		"echo `printf $'a\\'b'`":           "echo `printf $'a\\\\b'`",
		"echo \"$(printf $'a\\'b')\"":      "echo \"$(printf $'a\\\\b')\"",
		"(echo $'a\\'b')":                  "(echo $'a\\\\b')",
		"cat <<EOF\n$(echo $'a\\'b')\nEOF": "cat <<EOF\n$(echo $'a\\\\b')\nEOF",
		"echo $'a\\\\' $'b\\'c'":           "echo $'a\\\\' $'b\\\\c'",
	}
	for in, want := range changed {
		if got := neutralANSIC(in); got != want {
			t.Errorf("neutralANSIC(%q) = %q, want %q", in, got, want)
		}
	}
	unchanged := []string{
		"echo # $'a\\'b",
		"cat <<'EOF'\n$(echo $'a\\'b)\nEOF",
		"echo \"$'a\\'b\"",
		"echo $'a\\tb'",
		"",
	}
	for _, in := range unchanged {
		if got := neutralANSIC(in); got != in {
			t.Errorf("neutralANSIC(%q) = %q, want no change", in, got)
		}
	}
}

func TestNeutralANSICFixedPoint(t *testing.T) {
	inputs := []string{
		"echo $'\\'' $(echo $'\\'' $(",
		"x=$(printf $'it\\'s'); env",
		"echo $'a\\\\\\'b' $'c\\'d'",
		"echo $'unclosed\\'",
		"$(", "$'", "$'\\",
	}
	for _, in := range inputs {
		once := neutralANSIC(in)
		if len(once) != len(in) {
			t.Errorf("neutralANSIC(%q) has length %d, want %d", in, len(once), len(in))
		}
		if twice := neutralANSIC(once); twice != once {
			t.Errorf("neutralANSIC(%q) = %q, want a fixed point", once, twice)
		}
	}
}

func TestSecretsReadNoPanic(t *testing.T) {
	inputs := []string{
		"$(", "$((", "$(()", "`", "``", "(", ")", "((", "))", "$", "\\", "'", "\"", "\"$(", "'$(",
		"<<", "<<-", "<<'", "<<EOF", "<<'EOF'\n", "cat <<EOF\n$(", "$(cat <<EOF", "$(cat <<EOF\n",
		"<(", ">(", "<<<", "bash -c", "bash -c ''", "sh -lc", "sudo", "env -u", "command -", "declare -",
		"time", "time -p", "!", "{", "f()", "\x00", "\xff$(\xff", "echo \"$(echo \"$(echo `",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			envVerdict(bash(input))
			nestedScripts(input)
			isEnvDump(expandSplitStrings(strings.Fields(input)))
		})
	}
}
