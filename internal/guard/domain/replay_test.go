package domain

import (
	"bufio"
	"encoding/json"
	"os"
	"slices"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	replayFile     = "testdata/deny-replay.jsonl"
	replayMaxBytes = 1 << 20
)

type replayRow struct {
	Command  string `json:"command"`
	Cwd      string `json:"cwd"`
	Class    string `json:"target_class"`
	Expected string `json:"expected_after_fix"`
	Want     string `json:"want"`
	Gap      string `json:"gap"`
}

func readReplayRows(t *testing.T) []replayRow {
	t.Helper()
	f, err := os.Open(replayFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, replayMaxBytes), replayMaxBytes)
	var rows []replayRow
	for n := 1; scanner.Scan(); n++ {
		var row replayRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("row %d: %v", n, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestDenyReplay(t *testing.T) {
	for n, row := range readReplayRows(t) {
		call := ToolCall{Kind: shareddomain.ToolKindShell, Command: row.Command}
		_, denied := denyShellWriteSource(call, RuleContext{Cwd: row.Cwd, Worktree: row.Cwd})
		got := string(shareddomain.VerdictAllow)
		if denied {
			got = string(shareddomain.VerdictDeny)
		}
		if got != row.Want {
			t.Errorf("row %d (%s): got %s, want %s\n%s", n+1, row.Class, got, row.Want, row.Command)
		}
		if (row.Want != row.Expected) != (row.Gap != "") {
			t.Errorf("row %d (%s): want %s, expected %s, gap %q", n+1, row.Class, row.Want, row.Expected, row.Gap)
		}
	}
}

var notFlatLines = []string{
	"cd sub && git commit -m \"$(echo x)\" -F note.txt",
	"set -e; cd /tmp/x; git commit -F m.txt",
	"(cat > /work/repo/a.go)",
	"echo $(cat > a.go)",
	"(S=/tmp; cat > $S/a.go)",
	"cd /missing &&\r\necho x > a.go",
	"D=/work/repo\ncat > /tmp/n.txt <<E\\\nF\ngo test \\\nEF\nD=/tmp/scratch\ncat > /tmp/o.txt <<'X'\nx\nX\necho hi > $D/a.go",
}

func TestNotFlatKeepsMainPath(t *testing.T) {
	for _, line := range notFlatLines {
		if _, flat := flatInvocations(line); flat {
			t.Errorf("%q is flat", line)
		}
	}
	type probe struct {
		line string
		rc   RuleContext
	}
	probes := make([]probe, 0, len(notFlatLines))
	for _, line := range notFlatLines {
		probes = append(probes, probe{line, baseContext()})
	}
	for _, row := range readReplayRows(t) {
		probes = append(probes, probe{row.Command, RuleContext{Cwd: row.Cwd, Worktree: row.Cwd}})
	}
	notFlat := 0
	for _, p := range probes {
		if _, flat := flatInvocations(p.line); flat {
			continue
		}
		notFlat++
		_, denied := denyShellWriteSource(bash(p.line), p.rc)
		if want := mainShellWriteSource(bash(p.line), p.rc); denied != want {
			t.Errorf("%q: shell-write-source denied %v, main %v", p.line, denied, want)
		}
		if got, want := TextFiles(p.line), mainTextFiles(invocations(p.line)); !slices.EqualFunc(got, want, slices.Equal) {
			t.Errorf("%q: TextFiles %q, main %q", p.line, got, want)
		}
	}
	if notFlat == 0 {
		t.Fatal("no line is not flat")
	}
}
