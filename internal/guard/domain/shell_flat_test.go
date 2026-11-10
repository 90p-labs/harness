package domain

import (
	"reflect"
	"testing"
)

func flatArgs(line string) ([][]string, bool) {
	commands, flat := flatCommands(line)
	var out [][]string
	for _, c := range commands {
		out = append(out, c.Args)
	}
	return out, flat
}

func TestFlatCommandsExpansion(t *testing.T) {
	cases := []struct {
		name string
		line string
		want [][]string
	}{
		{"unquoted", "S=/t; cat $S/a", [][]string{{"S=/t"}, {"cat", "/t/a"}}},
		{"longest name", "S=/t; echo $S_test", [][]string{{"S=/t"}, {"echo", "$S_test"}}},
		{"name with underscore is its own variable", "S_test=/u; S=/t; echo $S_test $S", [][]string{{"S_test=/u"}, {"S=/t"}, {"echo", "/u", "/t"}}},
		{"double quoted", `S=/t; echo "$S/a b"`, [][]string{{"S=/t"}, {"echo", "/t/a b"}}},
		{"single quoted", "S=/t; echo '$S'", [][]string{{"S=/t"}, {"echo", "$S"}}},
		{"escaped dollar", `S=/t; echo \$S`, [][]string{{"S=/t"}, {"echo", "$S"}}},
		{"backgrounded list", "S=/t; S=/u && true & echo $S", [][]string{{"S=/t"}, {"S=/u"}, {"true"}, {"echo", "$S"}}},
		{"heredoc body stays literal", "S=/t; cat <<'EOF'\n$S\nEOF", [][]string{{"S=/t"}, {"cat"}}},
		{"unknown name keeps its text", "echo $S ${S} $1", [][]string{{"echo", "$S", "${S}", "$1"}}},
		{"value built from a known name", "A=/a; B=$A/b; echo $B", [][]string{{"A=/a"}, {"B=/a/b"}, {"echo", "/a/b"}}},
		{"value with a dollar stays unresolved", "A='$X'; echo $A", [][]string{{"A=$X"}, {"echo", "$X"}}},
		{"start of line assignment holds after and", "S=/t && echo $S; echo $S", [][]string{{"S=/t"}, {"echo", "/t"}, {"echo", "/t"}}},
		{"assignment after and holds to the end of the chain", "a && S=/t && echo $S; echo $S", [][]string{{"a"}, {"S=/t"}, {"echo", "/t"}, {"echo", "$S"}}},
		{"assignment after and is gone after or", "a && S=/t && echo $S || echo $S", [][]string{{"a"}, {"S=/t"}, {"echo", "/t"}, {"echo", "$S"}}},
		{"assignment after and holds through a pipeline", "a && S=/t && b | c $S; echo $S", [][]string{{"a"}, {"S=/t"}, {"b"}, {"c", "/t"}, {"echo", "$S"}}},
		{"assignment that may not run", "false && S=/t; echo $S", [][]string{{"false"}, {"S=/t"}, {"echo", "$S"}}},
		{"assignment after or", "a || S=/t; echo $S", [][]string{{"a"}, {"S=/t"}, {"echo", "$S"}}},
		{"assignment after or stays unknown in its chain", "a || S=/t && echo $S", [][]string{{"a"}, {"S=/t"}, {"echo", "$S"}}},
		{"assignment before a pipe", "S=/t | true; echo $S", [][]string{{"S=/t"}, {"true"}, {"echo", "$S"}}},
		{"assignment after a pipe", "true | S=/t; echo $S", [][]string{{"true"}, {"S=/t"}, {"echo", "$S"}}},
		{"assignment in the background", "S=/t & echo $S", [][]string{{"S=/t"}, {"echo", "$S"}}},
		{"quoted parenthesis and keyword stay flat", "S=/t; echo '(x)' \"(y)\" \"done\"; echo $S", [][]string{{"S=/t"}, {"echo", "(x)", "(y)", "done"}, {"echo", "/t"}}},
		{"values without a dollar are sure", "S=/t A=/u; echo $A", [][]string{{"S=/t", "A=/u"}, {"echo", "/u"}}},
		{"prefix assignment", "S=/t; S=/u cat; echo $S", [][]string{{"S=/t"}, {"S=/u", "cat"}, {"echo", "$S"}}},
		{"append assignment", "S=/t; S+=x; echo $S", [][]string{{"S=/t"}, {"S+=x"}, {"echo", "$S"}}},
		{"dollar at the end of a word stays in the form", "S=/t; grep \"x$\" $S", [][]string{{"S=/t"}, {"grep", "x$", "/t"}}},
		{"name as an argument makes it unknown", "S=/t; foo S; echo $S", [][]string{{"S=/t"}, {"foo", "S"}, {"echo", "$S"}}},
		{"name after a flag letter makes it unknown", "S=/t; wait -pS; echo $S", [][]string{{"S=/t"}, {"wait", "-pS"}, {"echo", "$S"}}},
		{"flag that is the name keeps it known", "S=/t; git -S x; echo $S", [][]string{{"S=/t"}, {"git", "-S", "x"}, {"echo", "/t"}}},
		{"name built from values is unsafe", "SX=/t; A=S; C=${A}X; echo $C $SX", [][]string{{"SX=/t"}, {"A=S"}, {"C=SX"}, {"echo", "SX", "$SX"}}},
		{"known name resolves beside other commands", "S=/tmp/x; mkdir -p $S && cat $S/a.go", [][]string{{"S=/tmp/x"}, {"mkdir", "-p", "/tmp/x"}, {"cat", "/tmp/x/a.go"}}},
		{"assignment with a redirect makes it unknown", "S=/t >/dev/null; echo $S", [][]string{{"S=/t"}, {"echo", "$S"}}},
		{"quoted hash stays in the form", "S=/t; echo \"## Why\" '#'; echo $S", [][]string{{"S=/t"}, {"echo", "## Why", "#"}, {"echo", "/t"}}},
		{"other names stay known", "S=/t; T=/u; echo $S", [][]string{{"S=/t"}, {"T=/u"}, {"echo", "/t"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, flat := flatArgs(tc.line)
			if !flat || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("flatCommands(%q)\n got %q (flat %v)\nwant %q", tc.line, got, flat, tc.want)
			}
		})
	}
}

func TestFlatCommandsNotFlat(t *testing.T) {
	lines := []string{
		"S=/t; echo $(echo $S)",
		"S=/t; echo \"$(echo $S)\"",
		"$(S=/t; echo $S); echo $S",
		"f(){\n S=/t\n}\necho $S",
		"f () {\n S=/t\n}\necho $S",
		"S=/t; (true); echo $S",
		"S=/t; for S in a; do :; done; echo $S",
		"S=/t; read S; echo $S",
		"S=/t; eval x; echo $S",
		"S=/t; source f; echo $S",
		"S=/t; . f; echo $S",
		"IFS=/; S=/t; echo $S",
		"f() { :; }; S=/t; echo $S",
		"set -- a; S=/t; echo $S",
		"set -o errexit; S=/t; echo $S",
		"export S=/t; echo ${S}x",
		"S=/t; local S; echo $S",
		"S=/t; unset S; echo $S",
		"S=/t; echo $S''x; let x=S=0; echo $S",
		"S=/t; cat <<EOF\n$S\nEOF",
		"A=(x); S=/t; echo $S",
		"S=/t; : ${S:=/u}; echo $S",
		"S=/t; : $((S=0)); echo $S",
		"S=/t; ((S=0)); echo $S",
		"S=/t; S[0]=/u; echo $S",
		"S=/t; echo $S[1]",
		"S=/t; : # x\necho $S",
		"S=/t; echo \"`true`\"; echo $S",
		"S=/t; echo $\"x\"; echo $S",
		"cat <<E\\\nF\nx\nEF",
		"_=/tmp; cat > $_/a.go",
		"cdpath=/work; cd repo",
		"/usr/bin/cd /tmp; cat > a.go",
		"env cd /tmp; cat > a.go",
		"echo x\r\ncat > a.go",
	}
	for _, line := range lines {
		t.Run(line, func(t *testing.T) {
			if _, flat := flatCommands(line); flat {
				t.Fatalf("flatCommands(%q) is flat", line)
			}
		})
	}
}

func TestFlatCommandsOp(t *testing.T) {
	line := "a && b || c ; d | e & f\ng"
	want := []string{opAnd, opOr, opSemi, opPipe, opBackground, opNewline, opNone}
	var got []string
	commands, _ := flatCommands(line)
	for _, c := range commands {
		got = append(got, c.Op)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flatCommands(%q)\n got %q\nwant %q", line, got, want)
	}
}
