package domain

import (
	"slices"
	"strings"
	"testing"
)

func TestShellWriteBypass(t *testing.T) {
	denied := []string{
		"cat <<'E' > /tmp/x.txt\nE\necho x > /work/repo/internal/a.go",
		"false && S=/tmp; cat > $S/a.go",
		"S=/tmp/x; cat > $S/a.go <<EOF\nuses $S here\nEOF",
		"D=/work/repo\ncat > /tmp/notes.txt <<EOF\ngo test \\\nEOF\nD=/tmp/scratch\ncat > /tmp/other.txt <<EOF\nx\nEOF\necho hi > $D/a.go",
		"cd /missing &&\r\necho x > a.go",
		"D=/work/repo\ncat > /tmp/n.txt <<E\\\nF\ngo test \\\nEF\nD=/tmp/scratch\ncat > /tmp/o.txt <<'X'\nx\nX\necho hi > $D/a.go",
		"cd /tmp && cdpath=/work && cd repo && cat > a.go",
		"_=/tmp; mkdir -p /work/repo/x; cat > $_/a.go",
		"/usr/bin/cd /tmp && echo x > a.go",
		"S=/work/repo; S=/tmp \r\necho x > $S/a.go",
		"cd /missing &&\f\necho x > a.go",
		"S=/work/repo; S=/tmp \v\necho x > $S/a.go",
		"S=/tmp | true; cat > $S/a.go",
		"if true; then\n  S=/tmp\nfi\ncat > $S/a.go",
		"cd /tmp/none; cat > a.go",
		"(\ncd /tmp\n)\ncat > a.go",
		"cd /tmp; pushd /work/repo; cat > a.go",
		"cd -- /tmp; cat > a.go",
		"cd /tmp && cat > $OLDPWD/a.go",
		"cd /tmp && cat > ~/a.go",
		"S=/work/repo; cat > $S/main.go",
		"S=.; cat > $S/main.go",
		"W=/work/repo; cd $W && cat > main.go",
		"S=/tmp cat > $S/a.go",
		"cd /tmp/x && cat > a.go; cat > b.go",
		"set -e; cd /tmp/x; set +e; cd ..; cat > a.go",
		"S=/work/repo\nf(){\nS=/tmp/x\n}\ncat > $S/main.go",
		"S=/work/repo\nf () {\nS=/tmp/x\n}\ncat > $S/main.go",
		"set -e; cd /tmp/x; if true; then cd /work/repo; fi; cat > a.go",
		"set -e; cd /tmp/x; { cd /work/repo; }; cat > a.go",
		"set -e; cd /tmp/x; eval 'cd /work/repo'; cat > a.go",
		"set -e; cd /tmp/x; if true; then set +e; fi; cd /nowhere; cat > a.go",
		"S=/work/repo; S=/tmp && true & cat > $S/main.go",
		"x=$(echo hi > main.go; echo y)",
		"v=$(make | tee out.md | tail -1)",
		"x=$(eval S=/z); cat > ${S}a.go",
		"x=$(eval S=/tmp); cat > $S/a.go",
		"x=$(true; S=/tmp); cat > $S/a.go",
		"S=\"/tmp/x /work/repo\"; tee $S/main.go",
		"S='/tmp/*'; tee $S/main.go",
		"IFS=/; S=/tmp/x; tee $S.go",
		"S=/tmp/x; IFS=/; tee $S.go",
		"f() { S=/work/repo; }; S=/tmp; f; cat > $S/a.go",
		"f() { cd /work/repo; }; cd /tmp && f && cat > a.go",
		"set -- -e; cd /tmp/nonexist; tee a.go",
		"set x -e; cd /tmp/nonexist; tee a.go",
		"cd /tmp; export CDPATH=/work; cd repo && cat > a.go",
		"S=(/work/repo); cd /tmp && cat > $S/a.go",
		"alias cd=true; cd /tmp && cat > a.go",
		"S=; : ${S:=/work/repo}; cat > $S/a.go",
		"S=/tmp; echo \"${S:=/work/repo}\"; cat > $S/a.go",
		"S=; ${S:=/work/repo} ; cat > $S/a.go",
		"S=/tmp; : $((S=0)); cat > $S/a.go",
		"S=/tmp; ((S=0)); cat > $S/a.go",
		"S=/tmp; : $[S=0]; cat > $S/a.go",
		"S=/tmp; for ((S=0; S<1; S++)); do :; done; cat > $S/a.go",
		"S=/tmp; S[0]=/work/repo; cat > $S/a.go",
		"S=/tmp; : ${(P)S}; cat > $S/a.go",
		"S=/tmp; cat > $S[1]/a.go",
		"cd /tmp && cd /wor[k]/repo && cat > a.go",
		"cd /tmp && cd /wor?/repo && cat > a.go",
		"x=$(builtin cd /tmp) && cat > a.go",
		"x=$(cd /tmp) && cat > a.go",
		"S=/tmp; printf -vS /work/repo; cat > $S/a.go",
		"S=/tmp; printf -vS[0] /work/repo; cat > $S/a.go",
		"S=/tmp; wait -pS; cat > $S/a.go",
		"S=/tmp; mapfile -C 'eval S=/work/repo' -c 1 X < /etc/hosts; cat > $S/a.go",
		"S=/tmp; for i in 1 2; do cat > $S/a.go; S=/work/repo; done",
		"S=/work/repo; : # x; S=/tmp\ncat > $S/a.go",
		"S=/work/repo; echo `true; S=/tmp; true`; cat > $S/a.go",
		"S=/tmp; let x=S=0; cat > $S/a.go",
		"S=/tmp; [[ x=S=0 -eq 0 ]]; cat > $S/a.go",
		"S=/tmp; : <<EOF\n$((S=0))\nEOF\ncat > $S/a.go",
		"S=/tmp; let $\"\"S=0; cat > $S/a.go",
		"S=/tmp/x; cat > $S:t/a.go",
		"S=/tmp >/nonexistent/q; cat > $S/a.go",
		"S=/tmp; : {S}>/dev/null; cat > $S/a.go",
		"S=/tmp; echo $S''x; let x=S=0; cat > $S/a.go",
		"S=/tmp; X=S; printf -v $X /work/repo; cat > $S/a.go",
		"S=/tmp; X='S=0'; [[ $X -eq 0 ]]; cat > $S/a.go",
		"SS=/tmp; printf -v S''S /work/repo; cat > $SS/a.go",
		"S=/work/repo; 'S'=/tmp; cat > $S/a.go",
		"S=/work/repo; \"S\"=/tmp; cat > $S/a.go",
		"S=/work/repo; \\S=/tmp; cat > $S/a.go",
		"S=/work/repo; S\"\"=/tmp; cat > $S/a.go",
		"S=/tmp; : <<EOF\n${S:=/work/repo}\nEOF\ncat > $S/a.go",
		"S=/work/repo; true <(true; S=/tmp; true); echo x > $S/p1.go",
		"set -e; true <(true; cd /tmp; true); echo x > p2.go",
		"S=/work/repo; true >(true; S=/tmp; true); echo x > $S/p1.go",
		"S=/work/repo; true =(true; S=/tmp; true); echo x > $S/q1.go",
		"S=/work/repo; (case a in a)\nS=/tmp;;\nesac); echo x > $S/p4.go",
		"S=/work/repo; x=$(case a in a) true; S=/tmp;; esac); echo x > $S/p4.go",
		"S=/work/repo; (echo x(N)\nS=/tmp; true); echo x > $S/p4.go",
		"S=/work/repo; coproc { true; S=/tmp; }; echo x > $S/p5.go",
		"S=/work/repo; repeat 0 do true; S=/tmp; done; echo x > $S/p7.go",
		"S=/tmp; S=/work/repo A=$S; echo x > $A/p6.go",
		"S=/tmp/x; for f in a b; do echo $f; done; cat > $S/a.go",
		"set -e; S=/tmp/x; cd $S; cat > b.go",
		"set -o errexit\ncd /tmp/x\ncat > b.go",
		"set -euo pipefail; cd /tmp; echo x > a.go",
		"set -e; cd /tmp; echo x > a.go",
		"env cd /tmp && echo x > a9.go",
		"nohup cd /tmp && echo x > a9.go",
		"cd /tmp && time cd /work/repo && echo x > a1.go",
		"cd /tmp && ! cd /work/repo && echo x > a1.go",
		"cd /tmp && chdir /work/repo && echo x > a1.go",
		"cd /tmp && noglob cd /work/repo && echo x > a1.go",
		"cd /tmp && - cd /work/repo && echo x > a1.go",
		"cd /tmp && echo cd && echo x > a1.go",
		"S=/tmp; ! eval 'S=/work/repo'; cat > $S/a.go",
		"set -o cdablevars; T=/work/repo; cd /tmp && cd T && echo x > a8.go",
		"set -o autopushd; cd /tmp && cd +1 && echo x > a8.go",
		"set -e; set +e; cd /tmp/none; echo x > a.go",
		"set -e; unsetopt errexit; cd /tmp/none; echo x > a.go",
		"SX=/work/repo; A=S; ${A}X=/tmp; echo x > $SX/a12.go",
		"SX=/tmp/x; A=S; C=${A}X; let $C=0; echo x > $SX.go",
		"SX=/tmp/x; A=S; C=${A}X; printf -v \"$C\" /work/repo; echo x > $SX/a.go",
		"E=true; S=/tmp; cat <<$E >/dev/null\n$E\nS=/work/repo\ntrue\necho x > $S/h2.go",
		"cd internal/guard/domain\nOUT=../ui/cli\ncat > $OUT/pr.go <<'EOF'\nx\nEOF",
		"cd internal/guard/domain; OUT=../ui/cli; cat > $OUT/pr.go",
		"cd /work/repo/internal/guard/domain\nOUT=..\ncat > $OUT/rule.go",
		"A=..; cat > $A/$B/x.go",
		"cd internal/guard/domain && OUT=../ui/cli && cat > $OUT/pr.go",
		"E=; cd /tmp/out && cp gen /work/repo/internal/b.go $E",
		"D=/work/repo/.claude; cp /tmp/settings.json $D",
		"D=/work/repo/v1.2; mv /tmp/a.go \"$D\"",
	}
	for _, line := range denied {
		t.Run(line, func(t *testing.T) {
			if _, ok := denyShellWriteSource(bash(line), baseContext()); !ok {
				t.Fatalf("%q was allowed", line)
			}
		})
	}
	allowed := []string{
		"S=/tmp; cat <<'E' > /tmp/y.txt\nx\nE\necho x > $S/a.go",
		"S=/tmp/x; cat > $S/a_test.go",
		"cd /tmp && echo x > a.go",
		"S=/tmp/x; cat > ${S}_v/a.go",
		"cd /tmp/x && cat > a.go",
		"S=/tmp/x && cat > $S/a.go",
		"cd /tmp/x && cd sub && cat > a.go",
		"cd /tmp/x && git -C /tmp/r status && cat > a.go",
		"S=/tmp/x; mkdir -p $S && cat > $S/a.go",
		"S=/tmp/x; cat > $S/a.go <<'EOF'\nuses $S and S here\nEOF",
		"S=/tmp/x; echo '(x)' \"done\" > $S/a.go",
		"S=/tmp/x A=/tmp/y; cat > $A/a.go",
		"echo $(cat > a.go)",
		"(S=/tmp; cat > $S/a.go)",
		"(cat > /work/repo/a.go)",
		"A=../..; cat > $A/repo/x.go",
		"cat > /tmp/$B/x.go",
		"echo x > \"internal/x.go)\"",
	}
	for _, line := range allowed {
		t.Run(line, func(t *testing.T) {
			if reason, ok := denyShellWriteSource(bash(line), baseContext()); ok {
				t.Fatalf("%q was denied: %s", line, reason)
			}
		})
	}
}

func TestShellNeverPanics(t *testing.T) {
	const maxLen = 70
	templates := []string{
		"cat <<E '\nx\nE \nS",
		"echo 'abc $S > a.go",
		"echo \"abc $S > a.go\\",
		"cat <<'E\nbody\nE",
		"cat <<\"E\nbody\nE",
		"cat <<E\nbody",
		"echo \"$(cat <<E 'x\nE\n",
		"S=/tmp; cat > \"$S/a.go",
		"echo \\",
		"echo \"$(cat <<\"E",
		"\"$(<<E\"x",
		"\" $(<<-{]E\"é",
		"S=/tmp; echo ${S}x \"${S:-a}\" $(true) `true` <(true) > a.go",
		"sed -i '' x ${A.go",
		"sed -i '' x $(a.go",
		"sed -i '' x `a.go",
		"tee ~",
	}
	for _, template := range templates {
		runes := []rune(template)
		for n := 0; n <= min(maxLen, len(runes)); n++ {
			line := string(runes[:n])
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%q panicked: %v", line, r)
					}
				}()
				Commands(line)
				flatCommands(line)
				TextFiles(line)
				for _, rule := range Rules() {
					rule.Denies(bash(line), baseContext())
				}
			}()
		}
	}
}

func TestShellWriteReason(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"relative file", "echo x > internal/a.go", []string{"writes the source file `internal/a.go`.", "Edit or Write"}},
		{"file through a variable", "S=/work/repo/internal; cat > $S/a.go", []string{"source file `internal/a.go`.", "Edit or Write"}},
		{"file after a cd", "cd /work/repo/web && cat > a.ts", []string{"source file `web/a.ts`."}},
		{"unknown variable", "cat > $S/a.go", []string{"cannot resolve the target `$S/a.go`.", "use a literal absolute path"}},
		{"tilde", "cat > ~/a.go", []string{"cannot resolve the target `~/a.go`.", "use a literal absolute path"}},
		{"not flat with a literal path", "for f in a; do cat > /work/repo/a.go; done", []string{"the command is not a flat line, and it writes `/work/repo/a.go`.", "Edit or Write"}},
		{"not flat with a relative target", "for f in a; do cat > a.go; done", []string{"the command is not a flat line, and it writes `a.go`.", "Edit or Write"}},
		{"not flat with one part", "for f in a; do cat > $S/a.go; done", []string{"and the target `$S/a.go` holds `$S`.", "use a literal absolute path"}},
		{"not flat with two parts", "for f in a; do cat > $S/${T}/a.go; done", []string{"holds `$S` and `${T}`.", "use a literal absolute path"}},
		{"not flat with three parts", "for f in a; do cat > ~/$S/${T}.go; done", []string{"holds `~`, `$S` and `${T}`."}},
		{"not flat with a positional part", "for f in a; do cat > a/$1.go; done", []string{"holds `$1`.", "use a literal absolute path"}},
		{"not flat with an unclosed part", "for f in a; do cat > ${S/a.go; done", []string{"the target `${S/a.go` holds `${S/a.go`.", "use a literal absolute path"}},
		{"issue 93 payload 2", "R=.harness/runs/r1 && H=$(git rev-parse HEAD) && /usr/bin/sed -i '' \"s/^Head: .*/Head: $H/\" /work/repo/$R/self-check.md", []string{"`$R`", "use a literal absolute path"}},
		{"heredoc body line ends in a backslash", "D=/work/repo\ncat > /tmp/notes.txt <<EOF\ngo test \\\nEOF\nD=/tmp/scratch\ncat > /tmp/other.txt <<EOF\nx\nEOF\necho hi > $D/a.go", []string{"the command is not a flat line, and the target `$D/a.go` holds `$D`."}},
		{"carriage return", "cd /missing &&\r\necho x > a.go", []string{"the command is not a flat line, and it writes `a.go`."}},
		{"resolved source after an unresolved target", "cat > $X/notes.txt; cat > a.go", []string{"source file `a.go`.", "Edit or Write"}},
		{"unknown folder", "cd -- /tmp; cat > a.go", []string{"cannot resolve the target `a.go`.", "use a literal absolute path"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, ok := denyShellWriteSource(bash(tc.line), baseContext())
			if !ok {
				t.Fatalf("%q was allowed", tc.line)
			}
			for _, part := range tc.want {
				if !strings.Contains(reason, part) {
					t.Fatalf("reason %q lacks %q", reason, part)
				}
			}
		})
	}
}

func TestShellWriteSedInPlace(t *testing.T) {
	denied := []struct {
		name string
		line string
		want []string
	}{
		{"empty suffix after a cd", "cd /x; sed -i '' 's/a/b/' docs/readme_test.go", []string{"`docs/readme_test.go`"}},
		{"empty suffix", "sed -i '' 's/a/b/' docs/readme_test.go", []string{"`docs/readme_test.go`", "Edit or Write"}},
		{"script word is a source file", "sed -i '' a.go", nil},
		{"script word after a cd", "cd /x; sed -i '' a.go", nil},
		{"w command in the script", "sed -i 's/a/b/w a.go' notes.txt", nil},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			reason, ok := denyShellWriteSource(bash(tc.line), baseContext())
			if !ok {
				t.Fatalf("%q was allowed", tc.line)
			}
			if strings.Contains(reason, "``") {
				t.Fatalf("reason %q names an empty target", reason)
			}
			for _, part := range tc.want {
				if !strings.Contains(reason, part) {
					t.Fatalf("reason %q lacks %q", reason, part)
				}
			}
		})
	}
	allowed := []string{
		"sed -i '' 's/a/b/' notes.txt",
		"sed -i '' 's/a/b/' /work/repo/.harness/runs/r1/self-check.md",
	}
	for _, line := range allowed {
		t.Run(line, func(t *testing.T) {
			if reason, ok := denyShellWriteSource(bash(line), baseContext()); ok {
				t.Fatalf("%q was denied: %s", line, reason)
			}
		})
	}
}

func TestUnresolvedParts(t *testing.T) {
	cases := []struct {
		target string
		want   []string
	}{
		{"$A/b/${C}/$(d e)/`f`.go", []string{"$A", "${C}", "$(d e)", "`f`"}},
		{"~/x.go", []string{"~"}},
		{"~user/x.go", []string{"~user"}},
		{"a/$(b $(c))/d.go", []string{"$(b $(c))"}},
		{"${A/x.go", []string{"${A/x.go"}},
		{"$(x", []string{"$(x"}},
		{"a/`b.go", []string{"a/`b.go"}},
		{"a/$1/$?/$@.go", []string{"$1", "$?", "$@"}},
		{"a/$/b.go", nil},
		{"/work/repo/a.go", nil},
		{"a/~/b.go", nil},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			if got := unresolvedParts(tc.target); !slices.Equal(got, tc.want) {
				t.Fatalf("unresolvedParts(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}

func TestShellWriteStateDir(t *testing.T) {
	allowed := []string{
		"sed -i '' x .harness/runs/$RUN/self-check.md",
		"tee .kodify/$X/a.go",
		"H=$(date) && sed -i '' \"s/x/$H/\" .harness/runs/r1/self-check.md",
		"H=$(date) && tee .kodify/$X/a.go",
		"sed -i '' x /work/repo/.harness/runs/$RUN/self-check.md",
		"tee /work/repo/.kodify/$X/a.go",
		"H=$(date) && sed -i '' \"s/x/$H/\" /work/repo/.harness/runs/r1/self-check.md",
	}
	for _, line := range allowed {
		t.Run(line, func(t *testing.T) {
			if reason, ok := denyShellWriteSource(bash(line), baseContext()); ok {
				t.Fatalf("%q was denied: %s", line, reason)
			}
		})
	}
	for _, line := range []string{"sed -i '' x .harness/../a.go", "H=$(date) && sed -i '' x .harness/../a.go"} {
		t.Run(line, func(t *testing.T) {
			if _, ok := denyShellWriteSource(bash(line), baseContext()); !ok {
				t.Fatalf("%q was allowed", line)
			}
		})
	}
}

func TestIssue93Payloads(t *testing.T) {
	denied := []struct {
		name string
		line string
		want string
	}{
		{"payload 1", "cd /tmp/repro; sed -i '' 's/a/b/' docs/readme_test.go && go test ./docs/; git checkout docs/readme_test.go", "`docs/readme_test.go`"},
		{"payload 2", "R=.harness/runs/r1 && H=$(git rev-parse HEAD) && /usr/bin/sed -i '' \"s/^Head: .*/Head: $H/\" /work/repo/$R/self-check.md", "`$R`"},
	}
	for _, tc := range denied {
		t.Run(tc.name, func(t *testing.T) {
			reason, ok := denyShellWriteSource(bash(tc.line), baseContext())
			if !ok {
				t.Fatalf("%q was allowed", tc.line)
			}
			if !strings.Contains(reason, tc.want) {
				t.Fatalf("reason %q lacks %q", reason, tc.want)
			}
			if strings.Contains(reason, "$(git") {
				t.Fatalf("reason %q names the substitution", reason)
			}
		})
	}
	t.Run("payload 3", func(t *testing.T) {
		line := "/usr/bin/sed -i '' 's/^Head: .*/Head: y/' /work/repo/.harness/runs/r1/self-check.md"
		if reason, ok := denyShellWriteSource(bash(line), baseContext()); ok {
			t.Fatalf("%q was denied: %s", line, reason)
		}
	})
}

func TestShellWriteOptions(t *testing.T) {
	writesSource := []string{
		"D=/tmp/gen; cp -t sub $D/a.go",
		"D=/tmp/gen; mv -t sub $D/a.go",
		"D=/tmp/gen; OPTS=\"-t sub\"; cp $OPTS $D/a.go",
	}
	notFlat := append(slices.Clone(writesSource),
		"D=/tmp/gen; cp --target-directory=sub $D/a.go",
		"cp -T a.go /tmp/x",
		"cp -- a.go /tmp/x",
		"sed -e s/a/b/ /tmp/x",
		"sed -i.bak s/a/b/ /tmp/x",
		"perl -e 1 /tmp/x",
		"perl -pi.bak -e 1 /tmp/x",
		"tee --output-error=warn /tmp/x",
	)
	for _, line := range notFlat {
		t.Run(line, func(t *testing.T) {
			if _, flat := flatInvocations(line); flat {
				t.Fatalf("%q is flat", line)
			}
			_, got := denyShellWriteSource(bash(line), baseContext())
			if want := mainShellWriteSource(bash(line), baseContext()); got != want {
				t.Fatalf("%q: deny %v, origin deny %v", line, got, want)
			}
		})
	}
	for _, line := range writesSource {
		if _, ok := denyShellWriteSource(bash(line), baseContext()); !ok {
			t.Fatalf("%q was allowed", line)
		}
	}
	for _, line := range []string{"tee -a /tmp/x", "sed -ni s/a/b/ /tmp/x", "perl -pi /tmp/x", "cp -rfv a /tmp/x", "mv -fv a /tmp/x"} {
		t.Run(line, func(t *testing.T) {
			if _, flat := flatInvocations(line); !flat {
				t.Fatalf("%q is not flat", line)
			}
		})
	}
}

func TestTextFilesFollowsCd(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"variable", "B=/tmp/x; gh pr create --body-file $B/body.md", "/tmp/x/body.md"},
		{"cd in a chain", "cd /tmp/x && git commit -F m.txt", "/tmp/x/m.txt"},
		{"cd after a semicolon", "cd /tmp/x; git commit -F m.txt", "/tmp/x/m.txt"},
		{"relative cd after a semicolon", "cd sub; git commit -F clean.txt", "sub/clean.txt"},
		{"cd after set -e", "set -e; cd /tmp/x; git commit -F m.txt", "/tmp/x/m.txt"},
		{"unknown variable", "git commit -F $B/m.txt", "$B/m.txt"},
		{"not flat line follows every cd", "cd sub && git commit -m \"$(echo x)\" -F note.txt", "sub/note.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			for _, names := range TextFiles(tc.line) {
				for _, name := range names {
					got = name
				}
			}
			if got != tc.want {
				t.Fatalf("TextFiles(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}
