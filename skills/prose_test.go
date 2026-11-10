package skills_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
	"gopkg.in/yaml.v3"
)

const (
	proseBaseEnv   = "HARNESS_PROSE_BASE"
	proseGateID    = "prose"
	proseGateTest  = "-run '^TestChangedProse$' ./skills/"
	gitMainBranch  = "main"
	diffSrcPrefix  = "a/"
	diffDstPrefix  = "b/"
	diffNewFile    = "+++ "
	diffNameEnd    = "\t"
	markdownSuffix = ".md"
	testdataDir    = "testdata"
)

var (
	proseGitDiffFlags = []string{"-U0", "--no-color", "--no-ext-diff", "--no-renames", "--src-prefix=" + diffSrcPrefix, "--dst-prefix=" + diffDstPrefix}
	proseGatePaths    = []string{"skills/", "agents/", "docs/", "README.md", "DESIGN.md"}
	diffHunk          = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
)

func runGit(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-c", "core.quotePath=false", "-C", root}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func diffChangedLines(diff string) map[string]map[int]bool {
	changed := map[string]map[int]bool{}
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		if target, ok := strings.CutPrefix(line, diffNewFile); ok {
			target = strings.TrimSuffix(target, diffNameEnd)
			file, _ = strings.CutPrefix(target, diffDstPrefix)
			if file == target {
				file = ""
			}
			continue
		}
		m := diffHunk.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for n := start; n < start+count; n++ {
			if changed[file] == nil {
				changed[file] = map[int]bool{}
			}
			changed[file][n] = true
		}
	}
	return changed
}

func isProseFile(path string) bool {
	return strings.HasSuffix(path, markdownSuffix) && !slices.Contains(strings.Split(path, "/"), testdataDir)
}

func changedProse(root, base string) ([]string, error) {
	mergeBase, err := runGit(root, "merge-base", base, "HEAD")
	if err != nil {
		return nil, err
	}
	diffArgs := append([]string{"diff"}, proseGitDiffFlags...)
	diffArgs = append(append(diffArgs, strings.TrimSpace(mergeBase), "--"), proseGatePaths...)
	diff, err := runGit(root, diffArgs...)
	if err != nil {
		return nil, err
	}
	changed := diffChangedLines(diff)
	untracked, err := runGit(root, append([]string{"ls-files", "-z", "--others", "--exclude-standard", "--"}, proseGatePaths...)...)
	if err != nil {
		return nil, err
	}
	whole := map[string]bool{}
	for _, path := range strings.Split(untracked, "\x00") {
		if path != "" {
			whole[path] = true
			changed[path] = nil
		}
	}
	files := make([]string, 0, len(changed))
	for path := range changed {
		if isProseFile(path) {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	var report []string
	for _, path := range files {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}
		lines := changed[path]
		isChanged := func(n int) bool { return whole[path] || lines[n] }
		for _, p := range domain.CheckProse(strings.Split(string(raw), "\n"), isChanged) {
			report = append(report, fmt.Sprintf("%s:%d: %s: %s", path, p.Line, p.Rule, p.Message))
		}
	}
	return report, nil
}

func TestChangedProse(t *testing.T) {
	base := os.Getenv(proseBaseEnv)
	if base == "" {
		return
	}
	problems, err := changedProse(repository.Root(t), base)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

func TestProseGateConfig(t *testing.T) {
	var config struct {
		Gates []struct {
			ID  string `yaml:"id"`
			Run string `yaml:"run"`
		} `yaml:"gates"`
	}
	if err := yaml.Unmarshal([]byte(repository.Read(t, ".harness.yaml")), &config); err != nil {
		t.Fatal(err)
	}
	for _, gate := range config.Gates {
		if gate.ID != proseGateID {
			continue
		}
		if !strings.HasPrefix(gate.Run, proseBaseEnv+"="+gitMainBranch+" ") {
			t.Errorf("gate %s run %q must set %s to %s", proseGateID, gate.Run, proseBaseEnv, gitMainBranch)
		}
		if !strings.Contains(gate.Run, proseGateTest) {
			t.Errorf("gate %s run %q must contain %q", proseGateID, gate.Run, proseGateTest)
		}
		return
	}
	t.Fatalf(".harness.yaml has no gate with id %s", proseGateID)
}

func TestDiffChangedLines(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want map[string]map[int]bool
	}{
		{"long form", "+++ b/docs/a.md\n@@ -3,2 +5,3 @@ head\n+x\n", map[string]map[int]bool{"docs/a.md": {5: true, 6: true, 7: true}}},
		{"short form", "+++ b/README.md\n@@ -4 +4 @@\n+x\n", map[string]map[int]bool{"README.md": {4: true}}},
		{"zero count", "+++ b/README.md\n@@ -4,2 +3,0 @@\n-x\n-y\n", map[string]map[int]bool{}},
		{"two files", "+++ b/a.md\n@@ -1 +1,2 @@\n+++ b/b.md\n@@ -9,0 +10 @@\n", map[string]map[int]bool{"a.md": {1: true, 2: true}, "b.md": {10: true}}},
		{"name with a space ends with a tab", "+++ b/docs/new page.md\t\n@@ -1 +1 @@\n", map[string]map[int]bool{"docs/new page.md": {1: true}}},
		{"dev null", "+++ /dev/null\n@@ -1,3 +0,0 @@\n", map[string]map[int]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := diffChangedLines(tt.diff); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

const (
	oldSentence = "The harness keeps every run in a folder so that a person can read the plan, the findings and the evidence of each gate without any special tool at all."

	reasonDesignSentence  = "A reviewer agent reads the diff and writes one finding for each fault, and the verifier then tries to prove each finding before the lead sees it."
	reasonDesignTable     = "| `reviewer` | A reviewer on a second model reads the diff and writes one finding for each fault that the first model did not see in the diff alone. |"
	reasonReadmeLong      = "A reviewer agent reads the diff and writes one finding for each fault, and the verifier agent then tries to prove each finding before the lead sees it."
	reasonReadmeParagraph = "The run starts with a plan. You approve the plan. The worker writes the tests. The worker writes the code. The gates run the checks. The reviewer reads the diff. The lead ships the change."

	designBase = "# Design\n\n" + oldSentence + "\n\n"
	readmeBase = "# harness\n\nStatus: pre-alpha.\n\n" + oldSentence + "\n"
	tableHead  = "| Role | Rule |\n| --- | --- |\n"
)

func initTempRepo(t *testing.T, base map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", gitMainBranch},
		{"config", "user.email", "prose@example.test"},
		{"config", "user.name", "Prose Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		if _, err := runGit(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	writeFiles(t, root, base)
	commitAll(t, root)
	if _, err := runGit(root, "checkout", "-b", "feature"); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, text := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func commitAll(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "change"}} {
		if _, err := runGit(root, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func assertProse(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d problems, want %d: %v", len(got), len(want), got)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(got[i], prefix) {
			t.Errorf("problem %d is %q, want prefix %q", i, got[i], prefix)
		}
	}
}

func prosePrefix(file string, line int, rule domain.ProseRule) string {
	return fmt.Sprintf("%s:%d: %s:", file, line, rule)
}

func TestChangedProseInRepository(t *testing.T) {
	root := initTempRepo(t, map[string]string{
		"docs/a.md":          "# A\n\n" + oldSentence + "\n",
		"docs/testdata/x.md": "# X\n",
		"internal/x.md":      "# X\n",
	})
	writeFiles(t, root, map[string]string{
		"docs/a.md":          "# A\n\n" + oldSentence + "\n\n" + reasonReadmeLong + "\n",
		"docs/testdata/x.md": "# X\n\n" + reasonReadmeLong + "\n",
		"internal/x.md":      "# X\n\n" + reasonReadmeLong + "\n",
	})
	commitAll(t, root)
	writeFiles(t, root, map[string]string{
		"agents/new.md":    "# New\n\nThe agent stops here " + repository.EmDash + " and waits.\n",
		"docs/new page.md": "# Page\n\n" + reasonReadmeLong + "\n",
	})

	got, err := changedProse(root, gitMainBranch)
	if err != nil {
		t.Fatal(err)
	}
	assertProse(t, got,
		prosePrefix("agents/new.md", 3, domain.ProseEmDash),
		prosePrefix("docs/a.md", 5, domain.ProseSentenceLength),
		prosePrefix("docs/new page.md", 3, domain.ProseSentenceLength))

	if _, err := changedProse(root, "missing-base"); err == nil {
		t.Error("a missing base must give an error")
	}
}

func TestReworkReasonsFailTheProseGate(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		base    string
		changed string
		line    int
		rule    domain.ProseRule
	}{
		{"151 long sentence in the design", "DESIGN.md", designBase, designBase + reasonDesignSentence + "\n", 5, domain.ProseSentenceLength},
		{"206 long cell in the review models table", "DESIGN.md", designBase + tableHead, designBase + tableHead + reasonDesignTable + "\n", 7, domain.ProseSentenceLength},
		{"334 long sentence in the readme", "README.md", readmeBase, strings.Replace(readmeBase, "Status: pre-alpha.", reasonReadmeLong, 1), 3, domain.ProseSentenceLength},
		{"339 long paragraph in the readme", "README.md", readmeBase, strings.Replace(readmeBase, "Status: pre-alpha.", reasonReadmeParagraph, 1), 3, domain.ProseParagraphLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initTempRepo(t, map[string]string{tt.file: tt.base})
			writeFiles(t, root, map[string]string{tt.file: tt.changed})
			got, err := changedProse(root, gitMainBranch)
			if err != nil {
				t.Fatal(err)
			}
			assertProse(t, got, prosePrefix(tt.file, tt.line, tt.rule))
		})
	}
}
