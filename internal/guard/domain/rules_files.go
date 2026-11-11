package domain

import (
	"fmt"
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
	reasonWriteSource   = "the shell command writes the source file `%s`"
	parentPrefix        = ".."
	pathSeparator       = "/"
	currentDir          = "."
	minCopyOperandCount = 2

	homePrefix             = "~"
	unresolvedChars        = "$`"
	globChars              = "*?[{"
	reasonUnresolvedTarget = "the guard cannot resolve the target `%s`"
	reasonNotFlat          = "the command is not a flat line"
	nextStepAbsolute       = "use a literal absolute path"
)

const longFlagMark = "--"

var (
	writeOptions = map[string][]string{
		cmdTee:  {"-a", "-i", "-p", "--append", "--ignore-interrupts"},
		cmdSed:  {flagSedInPlace, "-E", "-n", "-r", "-s", flagSedInPlaceLong, "--regexp-extended", "--quiet", "--silent"},
		cmdPerl: {"-i", "-n", "-p"},
		cmdCp:   {"-a", "-f", "-i", "-n", "-p", "-r", "-R", "-v", "--force", "--no-clobber", "--recursive", "--verbose"},
		cmdMv:   {"-f", "-i", "-n", "-v", "--force", "--no-clobber", "--verbose"},
	}

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

func sourceFile(path string, rc RuleContext) (rel string, isSource bool) {
	if rc.Worktree == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(rc.Cwd, path)
	}
	rel, err := filepath.Rel(rc.Worktree, filepath.Clean(path))
	if err != nil || rel == parentPrefix || strings.HasPrefix(rel, parentPrefix+string(filepath.Separator)) {
		return "", false
	}
	if slices.Contains(stateDirs, strings.SplitN(rel, string(filepath.Separator), 2)[0]) {
		return "", false
	}
	return rel, slices.Contains(sourceExtensions, filepath.Ext(rel))
}

func writeOptionsAllowed(args []string) bool {
	if len(args) == 0 {
		return true
	}
	allowed, writes := writeOptions[args[0]]
	return !writes || !slices.ContainsFunc(args[1:], func(a string) bool {
		return unresolvable(a) || strings.HasPrefix(a, flagMark) && !optionAllowed(a, allowed)
	})
}

func optionAllowed(option string, allowed []string) bool {
	if strings.HasPrefix(option, longFlagMark) {
		return slices.Contains(allowed, option)
	}
	return option != flagMark && !slices.ContainsFunc([]rune(option[1:]), func(r rune) bool {
		return !slices.Contains(allowed, flagMark+string(r))
	})
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
		if sedInPlace(c) {
			_, files := sedTargets(c.Args[1:])
			targets = append(targets, files...)
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

func sedInPlace(c Command) bool {
	return len(c.Args) > 0 && c.Args[0] == cmdSed && slices.ContainsFunc(c.Args, func(a string) bool {
		return strings.HasPrefix(a, flagSedInPlace) || strings.HasPrefix(a, flagSedInPlaceLong)
	})
}

// sedTargets splits the arguments of an in-place sed into the script word and
// the file operands. The BSD form passes an empty suffix word after -i,
// which is not an operand.
func sedTargets(args []string) (script string, files []string) {
	kept := make([]string, 0, len(args))
	for i, a := range args {
		if a == "" && i > 0 && args[i-1] == flagSedInPlace {
			continue
		}
		kept = append(kept, a)
	}
	operands := nonFlags(kept)
	if len(operands) == 0 {
		return "", nil
	}
	return operands[0], operands[1:]
}

// sedScript returns the script word of an in-place sed.
func sedScript(c Command) (string, bool) {
	if !sedInPlace(c) {
		return "", false
	}
	script, _ := sedTargets(c.Args[1:])
	return script, script != ""
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

func isSourcePath(path string, rc RuleContext) bool {
	_, isSource := sourceFile(path, rc)
	return isSource
}

func mainShellWriteSource(call ToolCall, rc RuleContext) bool {
	for _, c := range invocations(call.Command) {
		script, hasScript := sedScript(c)
		if hasScript && isSourcePath(script, rc) ||
			slices.ContainsFunc(writeTargets(c), func(path string) bool { return isSourcePath(path, rc) }) {
			return true
		}
	}
	return false
}

func denyShellWriteSource(call ToolCall, rc RuleContext) (string, bool) {
	commands, flat := flatInvocations(call.Command)
	if !flat {
		if mainShellWriteSource(call, rc) {
			return deny(reasonNotFlat, nextStepAbsolute)
		}
		return "", false
	}
	var firstUnresolved *string
	for i, f := range walkFolders(commands) {
		if script, ok := sedScript(commands[i]); ok {
			// The script word is checked as a source file but never named as unresolved.
			if rel, isSource := sourceFile(f.join(script), rc); isSource {
				return deny(fmt.Sprintf(reasonWriteSource, rel), nextStepEditTool)
			}
		}
		for _, target := range flatWriteTargets(commands[i]) {
			if unresolvable(target) || !f.known && !filepath.IsAbs(target) {
				if firstUnresolved == nil {
					firstUnresolved = &target
				}
			} else if rel, isSource := sourceFile(f.join(target), rc); isSource {
				return deny(fmt.Sprintf(reasonWriteSource, rel), nextStepEditTool)
			}
		}
	}
	if firstUnresolved != nil && mainShellWriteSource(call, rc) {
		return deny(fmt.Sprintf(reasonUnresolvedTarget, *firstUnresolved), nextStepAbsolute)
	}
	return "", false
}

func flatWriteTargets(c Command) []string {
	targets := writeTargets(c)
	if len(c.Args) == 0 || c.Args[0] != cmdCp && c.Args[0] != cmdMv {
		return targets
	}
	operands := nonFlags(c.Args[1:])
	for _, source := range operands[:max(len(operands)-1, 0)] {
		targets = append(targets, filepath.Join(operands[len(operands)-1], filepath.Base(source)))
	}
	return targets
}

type folder struct {
	path  string
	known bool
}

func (f folder) join(path string) string {
	if filepath.IsAbs(path) || !f.known {
		return path
	}
	return filepath.Join(f.path, path)
}

func unresolvable(path string) bool {
	return strings.ContainsAny(path, unresolvedChars) || strings.HasPrefix(path, homePrefix)
}

func isSequential(op string) bool {
	return op == opNone || op == opSemi || op == opNewline
}

func walkFolders(commands []Command) []folder {
	walk := make([]folder, len(commands))
	cur := folder{known: true}
	afterChain := false
	for i, c := range commands {
		walk[i] = cur
		prev := opNone
		if i > 0 {
			prev = commands[i-1].Op
		}
		if len(c.Args) > 0 && c.Args[0] == cmdCd {
			inChain := (prev == opAnd || c.Op == opAnd) && isStepOp(prev) && isStepOp(c.Op)
			cur = cdFolder(c.Args, cur, inChain)
			afterChain = afterChain || inChain
		}
		if c.Op != opAnd && c.Op != opPipe && afterChain {
			cur, afterChain = folder{}, false
		}
	}
	return walk
}

func isStepOp(op string) bool {
	return isSequential(op) || op == opAnd
}

func cdFolder(head []string, cur folder, surely bool) folder {
	if !surely || len(head) != cdArgCount || unresolvable(head[1]) || strings.ContainsAny(head[1], globChars) || strings.HasPrefix(head[1], flagMark) {
		return folder{}
	}
	if filepath.IsAbs(head[1]) {
		return folder{path: head[1], known: true}
	}
	if !cur.known {
		return cur
	}
	return folder{path: filepath.Join(cur.path, head[1]), known: true}
}
