package domain

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	cmdTee  = "tee"
	cmdSed  = "sed"
	cmdPerl = "perl"
	cmdCp   = "cp"
	cmdMv   = "mv"

	flagSedInPlace      = "-i"
	flagSedInPlaceLong  = "--in-place"
	envFile             = ".env"
	envFilePrefix       = ".env."
	envExample          = ".env.example"
	sshKeyPrefix        = "id_rsa"
	goTestSuffix        = "_test.go"
	nextStepSecret      = "read .env.example, or ask the user for the value"
	nextStepFocus       = "remove the marker, or fix the test"
	nextStepEditTool    = "edit the file with the Edit or Write tool"
	reasonSecret        = "the path holds a secret"
	reasonFocusSkip     = "the test edit adds a focus or skip marker"
	reasonWriteSource   = "the shell command writes a source file"
	parentPrefix        = ".."
	pathSeparator       = "/"
	currentDir          = "."
	minCopyOperandCount = 2
)

var (
	printCommands  = []string{"echo", "printf"}
	searchCommands = []string{"grep", "egrep", "fgrep", "rg", "ag"}
	patternFlags   = []string{"-e", "--regexp"}

	secretSuffixes = []string{".pem", ".key"}
	testNameParts  = []string{".test.", ".spec."}
	stateDirs      = []string{".harness", ".kodify"}
	perlInPlace    = regexp.MustCompile(`^-[nplaw]*i`)

	focusSkipMarkers = []string{".only(", "fit(", "fdescribe(", ".skip(", "xit(", "t.Skip("}
	focusSkipPattern = markerPattern(focusSkipMarkers)

	sourceExtensions = []string{
		".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".css", ".scss", ".html",
		".md", ".json", ".yaml", ".yml", ".toml", ".sql", ".sh", ".py", ".mod",
	}
)

func markerPattern(markers []string) *regexp.Regexp {
	parts := make([]string, len(markers))
	for i, marker := range markers {
		parts[i] = regexp.QuoteMeta(marker)
		if first := marker[0]; first == '_' || first >= '0' && first <= '9' || first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z' {
			parts[i] = `\b` + parts[i]
		}
	}
	return regexp.MustCompile(strings.Join(parts, "|"))
}

func isSecretPath(path string) bool {
	base := filepath.Base(path)
	switch {
	case base == envExample:
		return false
	case base == envFile, strings.HasPrefix(base, envFilePrefix), strings.HasPrefix(base, sshKeyPrefix):
		return true
	}
	return slices.ContainsFunc(secretSuffixes, func(s string) bool { return strings.HasSuffix(base, s) })
}

func denySecretsRead(call ToolCall, _ RuleContext) (string, bool) {
	if call.Kind == shareddomain.ToolKindRead || call.Kind == shareddomain.ToolKindSearch {
		if slices.ContainsFunc(call.Paths, isSecretPath) {
			return deny(reasonSecret, nextStepSecret)
		}
	}
	for _, c := range invocations(call.Command) {
		if slices.ContainsFunc(slices.Concat(readOperands(c), c.Inputs, c.Redirects), isSecretPath) {
			return deny(reasonSecret, nextStepSecret)
		}
	}
	return "", false
}

func readOperands(c Command) []string {
	if len(c.Args) == 0 || slices.Contains(printCommands, c.Args[0]) {
		return nil
	}
	operands := nonFlags(c.Args[1:])
	if !slices.Contains(searchCommands, c.Args[0]) {
		return operands
	}
	patterns := flagValues(c, patternFlags...)
	if len(patterns) == 0 && len(operands) > 0 {
		return operands[1:]
	}
	return slices.DeleteFunc(operands, func(arg string) bool { return slices.Contains(patterns, arg) })
}

func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, goTestSuffix) ||
		slices.ContainsFunc(testNameParts, func(part string) bool { return strings.Contains(base, part) })
}

func denyFocusSkip(call ToolCall, _ RuleContext) (string, bool) {
	if slices.ContainsFunc(call.Paths, isTestFile) && focusSkipPattern.MatchString(call.NewText) {
		return deny(reasonFocusSkip, nextStepFocus)
	}
	return "", false
}

func isSourcePath(path string, rc RuleContext) bool {
	if rc.Worktree == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(rc.Cwd, path)
	}
	rel, err := filepath.Rel(rc.Worktree, filepath.Clean(path))
	if err != nil || rel == parentPrefix || strings.HasPrefix(rel, parentPrefix+string(filepath.Separator)) {
		return false
	}
	if slices.Contains(stateDirs, strings.SplitN(rel, string(filepath.Separator), 2)[0]) {
		return false
	}
	return slices.Contains(sourceExtensions, filepath.Ext(rel))
}

func writeTargets(c Command) []string {
	targets := slices.Clone(c.Redirects)
	if len(c.Args) == 0 {
		return targets
	}
	operands := nonFlags(c.Args[1:])
	switch c.Args[0] {
	case cmdTee:
		targets = append(targets, operands...)
	case cmdSed:
		if slices.ContainsFunc(c.Args, func(a string) bool {
			return strings.HasPrefix(a, flagSedInPlace) || strings.HasPrefix(a, flagSedInPlaceLong)
		}) {
			targets = append(targets, operands...)
		}
	case cmdPerl:
		if slices.ContainsFunc(c.Args, perlInPlace.MatchString) {
			targets = append(targets, operands...)
		}
	case cmdCp, cmdMv:
		if len(operands) >= minCopyOperandCount {
			targets = append(targets, copyTargets(operands)...)
		}
	}
	return targets
}

func copyTargets(operands []string) []string {
	dest := operands[len(operands)-1]
	name := filepath.Base(dest)
	isDir := strings.HasSuffix(dest, pathSeparator) || name == currentDir || name == parentPrefix || filepath.Ext(name) == ""
	if !isDir {
		return []string{dest}
	}
	targets := make([]string, 0, len(operands)-1)
	for _, source := range operands[:len(operands)-1] {
		targets = append(targets, filepath.Join(dest, filepath.Base(source)))
	}
	return targets
}

func denyShellWriteSource(call ToolCall, rc RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if slices.ContainsFunc(writeTargets(c), func(path string) bool { return isSourcePath(path, rc) }) {
			return deny(reasonWriteSource, nextStepEditTool)
		}
	}
	return "", false
}
