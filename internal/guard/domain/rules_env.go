package domain

import (
	"path/filepath"
	"slices"
	"strings"
)

const (
	cmdPrintenv = "printenv"
	cmdExport   = "export"
	cmdSet      = "set"
	cmdDeclare  = "declare"
	cmdTypeset  = "typeset"

	reasonEnvDump = "the command prints the process environment, which can hold a secret"

	procPrefix      = "/proc/"
	procEnvironName = "environ"

	flagHelp      = "--help"
	flagVersion   = "--version"
	wordBraceOpen = "{" // opens a brace group
	wordNegate    = "!" // negates the status of the pipeline
	funcDefSuffix = "()"
	// funcDefBraceSuffix ends the word of a header with no space before the brace.
	funcDefBraceSuffix = "(){"
	cmdTime            = "time"
	flagTimePosix      = "-p" // the POSIX output format of time

	flagLookup    byte = 'v'
	flagLookupAlt byte = 'V'
	flagPrint     byte = 'p'
	flagCommand   byte = 'c'
	flagFunc      byte = 'f'
	flagFuncNames byte = 'F'

	flagSplit     byte = 'S' // env -S splits its value into words and puts them in its place
	flagSplitLong      = "--split-string"
	splitSpace         = " \t\n\r\f\v" // the bytes that end a word of a split string

	flagOptionName byte = 'o' // -o takes a set option name
	flagShoptName  byte = 'O' // -O takes a shopt option name

	wordEndOptions   = "--" // ends the options of a shell
	longOptionPrefix = "--" // starts a long option
	prefixDash       = "-"  // starts a short option cluster that sets options
	prefixPlus       = "+"  // starts a short option cluster that unsets options

	// subshellPrev holds the bytes that may stand before a ( that opens a subshell.
	subshellPrev = " \t\r\n;&|("
	// wordStartPrev holds the bytes that may stand before a # that starts a comment.
	// It is the same set as subshellPrev. Neither has ), so a # after $(...) stays
	// in the word. A # after the ) of a subshell is read as text, not a comment.
	// A quote in that comment can then hide a later line, a known limit.
	wordStartPrev = subshellPrev
	// delimiterEnd holds the bytes that end a heredoc delimiter word.
	delimiterEnd = " \t\r\n;&|<>()"
	// quoteRunes holds the bytes that make a heredoc delimiter quoted.
	quoteRunes = "'\"\\"
	// hereString is the operator that reads a word, not a heredoc body.
	hereString = "<<<"
)

var (
	shellKeywords = []string{"if", "then", "elif", "else", "do", "while", "until"}
	shellPrograms = []string{"bash", "sh", "zsh", "dash"}
	// shellValuedLongOptions are the long shell options whose next word is a value.
	shellValuedLongOptions = []string{"--rcfile", "--init-file"}
	// envValuedOptions are the env options that take a value. The shared ones come from launchers.
	// The extras matter only to this rule. expandSplitStrings drops each but the split option.
	envValuedOptions = slices.Concat(launchers[cmdEnv], []string{"-P", "-a", "--argv0", "-S", flagSplitLong})
	// envLongOptions are all the long options of GNU env. A unique prefix names one.
	envLongOptions = []string{
		"--argv0", "--block-signal", "--chdir", "--debug", "--default-signal", "--help",
		"--ignore-environment", "--ignore-signal", "--list-signal-handling", "--null",
		flagSplitLong, "--unset", flagVersion,
	}
	// envValuedLetters are the letters of the short options in envValuedOptions.
	envValuedLetters = shortLetters(envValuedOptions)
	envSplitShort    = prefixDash + string(flagSplit)
)

func shortLetters(options []string) string {
	var letters strings.Builder
	for _, o := range options {
		if len(o) == 2 && o[0] == '-' && o[1] != '-' {
			letters.WriteByte(o[1])
		}
	}
	return letters.String()
}

func isProcEnviron(path string) bool {
	return strings.HasPrefix(path, procPrefix) && filepath.Base(path) == procEnvironName
}

// isEnvDump reports whether the raw words of one command print the process
// environment. It reads the words before programArgs, so a bare env is seen.
// The words come from expandSplitStrings, so no env has a split option left.
func isEnvDump(args []string) bool {
	for {
		args = skipPrefixWords(args)
		if len(args) == 0 {
			return false
		}
		name := filepath.Base(args[0])
		valued, launcher := launchers[name]
		if !launcher {
			return isDumpBuiltin(name, args[1:])
		}
		rest := skipOptions(args[1:], valued)
		options := args[1 : len(args)-len(rest)]
		switch {
		case name == cmdCommand && isLookup(options):
			return false
		case name == cmdEnv:
			rest = skipAssignments(rest)
			// skipOptions already drops a leading --, so this handles a -- after the assignments, as in env FOO=bar --.
			if len(rest) > 0 && rest[0] == wordEndOptions {
				rest = rest[1:]
			}
			if len(rest) == 0 {
				return !hasInfoFlag(options)
			}
		}
		args = rest
	}
}

// expandSplitStrings rewrites the words of one command. Each env launcher in
// the chain gets its split option replaced, in place, by the words of its value.
// It also drops the other valued env options. Each rewrite removes one split
// option and adds words that total no more bytes than its value, so it ends.
func expandSplitStrings(args []string) []string {
	p := 0
	for {
		p = len(args) - len(skipPrefixWords(args[p:]))
		if p >= len(args) {
			return args
		}
		name := filepath.Base(args[p])
		valued, launcher := launchers[name]
		switch {
		case !launcher:
			return args
		case name == cmdEnv:
			args, p = expandEnvOptions(args, p+1)
		default:
			p = len(args) - len(skipOptions(args[p+1:], valued))
		}
	}
}

// expandEnvOptions rewrites the options of the env whose options start at j.
// It returns the new words and the index of the first word after the options.
func expandEnvOptions(args []string, j int) ([]string, int) {
	for j < len(args) && strings.HasPrefix(args[j], prefixDash) && args[j] != wordEndOptions {
		width, words, reprocess := envOption(args, j)
		args = slices.Concat(args[:j:j], words, args[j+width:])
		if !reprocess {
			j += len(words)
		}
	}
	if j < len(args) && args[j] == wordEndOptions {
		j++
	}
	return args, j
}

// envOption reads the option at j. It returns how many words the option takes
// and the words that replace it. A split option gives the words of its value,
// and reprocess is true, since those words can hold options. A valued option gives none.
func envOption(args []string, j int) (width int, words []string, reprocess bool) {
	arg := args[j]
	name, value, attached := arg, "", false
	if strings.HasPrefix(arg, longOptionPrefix) {
		name, value, attached = strings.Cut(arg, "=")
		name = resolveEnvLong(name)
	} else if at := strings.IndexAny(arg[1:], envValuedLetters); at >= 0 {
		name, value = prefixDash+string(arg[1+at]), arg[at+2:]
		attached = value != ""
	}
	isSplit := name == envSplitShort || name == flagSplitLong
	if !isSplit && !slices.Contains(envValuedOptions, name) {
		return 1, []string{arg}, false
	}
	width = 1
	if !attached && j+1 < len(args) {
		value, width = args[j+1], 2
	}
	if isSplit {
		return width, splitWords(value), true
	}
	return width, nil, false
}

// resolveEnvLong returns the full GNU env long option that a word names.
// An exact name or a unique prefix resolves. An ambiguous or unknown word stays as it is.
func resolveEnvLong(name string) string {
	if slices.Contains(envLongOptions, name) {
		return name
	}
	var found []string
	for _, option := range envLongOptions {
		if strings.HasPrefix(option, name) {
			found = append(found, option)
		}
	}
	if len(found) == 1 {
		return found[0]
	}
	return name
}

// wordSplitter builds the words of a split string.
type wordSplitter struct {
	words  []string
	cur    strings.Builder
	inWord bool
	done   bool // a \c ended the string
}

func (w *wordSplitter) add(c byte) {
	w.cur.WriteByte(c)
	w.inWord = true
}

func (w *wordSplitter) flush() {
	if w.inWord {
		w.words = append(w.words, w.cur.String())
		w.cur.Reset()
		w.inWord = false
	}
}

// quoted copies the text after the quote at i, up to the closing quote.
// It returns the index of the closing quote. A backslash follows the escape rules.
func (w *wordSplitter) quoted(s string, i int, quote byte) int {
	w.inWord = true
	for i++; i < len(s) && s[i] != quote && !w.done; i++ {
		switch end := expansionEnd(s, i); {
		case s[i] == '\\' && i+1 < len(s):
			i = w.escape(s, i, quote)
		case quote == '"' && end >= 0:
			i = end
		default:
			w.add(s[i])
		}
	}
	return i
}

// expansionEnd returns the index of the closing brace of a ${NAME} at i, or -1.
// env -S expands such a name in its value. The rule models every name as unset,
// so the expansion gives nothing. That keeps a hidden word from passing as safe.
func expansionEnd(s string, i int) int {
	if !strings.HasPrefix(s[i:], "${") {
		return -1
	}
	for j := i + 2; j < len(s); j++ {
		c := s[j]
		switch {
		case c == '}' && j > i+2:
			return j
		case c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
		default:
			return -1
		}
	}
	return -1
}

// escape reads the backslash at i and returns the index of the last byte it used.
// In single quotes only a backslash and a quote are escapes. Elsewhere it follows GNU env.
func (w *wordSplitter) escape(s string, i int, quote byte) int {
	c := s[i+1]
	switch {
	case quote == '\'' && c != '\\' && c != '\'':
		w.add('\\')
		return i
	case quote == '\'':
		w.add(c)
	case c == 'c':
		w.done = true
	case c == '_' && quote == 0:
		w.flush()
	case c == '_':
		w.add(' ')
	default:
		w.add(escapeByte(c))
	}
	return i + 1
}

// escapeByte returns the byte that a backslash and c stand for.
func escapeByte(c byte) byte {
	if at := strings.IndexByte("ntrfv", c); at >= 0 {
		return "\n\t\r\f\v"[at]
	}
	return c
}

// splitWords splits a string into words as GNU env -S does: single quotes,
// double quotes and backslash escapes. A # that starts a word ends the string.
// A ${NAME} outside single quotes gives nothing, and a word left empty with no quotes is dropped.
func splitWords(s string) []string {
	w := &wordSplitter{}
	for i := 0; i < len(s) && !w.done; i++ {
		switch c := s[i]; {
		case strings.IndexByte(splitSpace, c) >= 0:
			w.flush()
		case expansionEnd(s, i) >= 0:
			i = expansionEnd(s, i)
		case c == '#' && !w.inWord:
			return w.words
		case c == '\'' || c == '"':
			i = w.quoted(s, i, c)
		case c == '\\' && i+1 < len(s):
			i = w.escape(s, i, 0)
		default:
			w.add(c)
		}
	}
	w.flush()
	return w.words
}

func isLookup(options []string) bool {
	return slices.ContainsFunc(options, func(o string) bool {
		return shortCluster(o, flagLookup) || shortCluster(o, flagLookupAlt)
	})
}

func hasInfoFlag(words []string) bool {
	return slices.Contains(words, flagHelp) || slices.Contains(words, flagVersion)
}

func skipAssignments(args []string) []string {
	for len(args) > 0 && envAssignment.MatchString(args[0]) {
		args = args[1:]
	}
	return args
}

// skipPrefixWords drops the words that come before the program name:
// assignments, block and negation words, function headers and keywords.
func skipPrefixWords(args []string) []string {
	for len(args) > 0 {
		word := args[0]
		switch {
		case envAssignment.MatchString(word), word == wordBraceOpen, word == wordNegate,
			strings.HasSuffix(word, funcDefSuffix), strings.HasSuffix(word, funcDefBraceSuffix),
			slices.Contains(shellKeywords, word):
			args = args[1:]
		case word == cmdTime:
			args = args[1:]
			if len(args) > 0 && args[0] == flagTimePosix {
				args = args[1:]
			}
		default:
			return args
		}
	}
	return args
}

func isDumpBuiltin(name string, rest []string) bool {
	switch name {
	case cmdPrintenv:
		return !(len(rest) == 1 && hasInfoFlag(rest))
	case cmdExport:
		return len(nonFlags(rest)) == 0
	case cmdSet:
		return len(rest) == 0
	case cmdDeclare, cmdTypeset:
		return isDeclareDump(rest)
	}
	return false
}

// declareOperands returns the names of a declare command. A plus option is no name.
func declareOperands(rest []string) []string {
	return slices.DeleteFunc(nonFlags(rest), func(a string) bool {
		return len(a) > 1 && strings.HasPrefix(a, prefixPlus)
	})
}

func isDeclareDump(rest []string) bool {
	if slices.ContainsFunc(rest, func(a string) bool { return shortCluster(a, flagPrint) }) {
		return true
	}
	listsFuncs := slices.ContainsFunc(rest, func(a string) bool {
		return shortCluster(a, flagFunc) || shortCluster(a, flagFuncNames)
	})
	return len(declareOperands(rest)) == 0 && !listsFuncs
}

// valueCount returns how many following words a shell option takes as values.
// Each o or O in a single-dash or plus cluster takes one. A valued long option takes one.
func valueCount(arg string) int {
	if strings.HasPrefix(arg, longOptionPrefix) {
		if slices.Contains(shellValuedLongOptions, arg) {
			return 1
		}
		return 0
	}
	if !isOptionWord(arg) {
		return 0
	}
	cluster := arg[1:]
	return strings.Count(cluster, string(flagOptionName)) + strings.Count(cluster, string(flagShoptName))
}

// isOptionWord reports whether a word starts with a dash or a plus.
func isOptionWord(arg string) bool {
	return strings.HasPrefix(arg, prefixDash) || strings.HasPrefix(arg, prefixPlus)
}

// shellScript returns the -c script of a shell command, read after programArgs.
// It reads options up to the first operand, as bash does, and notes a -c cluster.
// A -- ends the options. The first operand is the script only when a -c was seen.
func shellScript(args []string) (string, bool) {
	if len(args) == 0 || !slices.Contains(shellPrograms, args[0]) {
		return "", false
	}
	sawCommand := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == wordEndOptions {
			i++ // the word after -- is the first operand
			return operandScript(args, i, sawCommand)
		}
		if !isOptionWord(arg) || len(arg) < 2 {
			return operandScript(args, i, sawCommand)
		}
		sawCommand = sawCommand || shortCluster(arg, flagCommand)
		i += valueCount(arg) // each value word is no operand
	}
	return "", false
}

// operandScript returns the first operand as the script when a -c was seen.
func operandScript(args []string, i int, sawCommand bool) (string, bool) {
	if !sawCommand || i >= len(args) {
		return "", false
	}
	return args[i], true
}

// nestedScripts returns the body of each substitution, process substitution,
// backtick part and subshell in the line. It does not return bodies inside bodies.
func nestedScripts(line string) []string {
	s := &scriptScan{src: line}
	s.run(0, true)
	return s.bodies
}

// neutralANSIC rewrites each backslash quote in a $'...' string, at any depth.
// The quote byte becomes a second backslash, so Commands cannot end a quote early.
// The line keeps its length, so each body stays shorter than its line.
// A second call finds no backslash quote and changes nothing.
func neutralANSIC(line string) string {
	s := &scriptScan{src: line}
	s.run(0, false)
	if len(s.escaped) == 0 {
		return line
	}
	out := []byte(line)
	for _, i := range s.escaped {
		out[i] = '\\'
	}
	return string(out)
}

type heredocMark struct {
	delimiter string
	stripTabs bool
	quoted    bool
}

type scriptScan struct {
	src     string
	pos     int
	bodies  []string
	escaped []int // offset of the quote byte of each backslash quote in a $'...' string
	pending []heredocMark
}

// run walks the text up to the closer, or to the end when the closer is 0.
// It reports whether it stopped at the closer. Only a collecting walk keeps bodies.
func (s *scriptScan) run(closer byte, collect bool) bool {
	inDouble := false
	depth := 0
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\\':
			s.pos += 2
		case c == '\'' && !inDouble:
			s.skipSingle()
		case c == '$' && s.at(1) == '$' && !inDouble:
			s.pos += 2 // the pid, so a quote after it is no $' string
		case c == '$' && s.at(1) == '\'' && !inDouble:
			s.skipANSIC()
		case c == '"':
			inDouble = !inDouble
			s.pos++
		case c == '`' && closer == '`' && !inDouble:
			s.pos++
			return true
		case c == '`':
			s.body('`', 1, collect)
		case c == '$' && s.at(1) == '(' && s.at(2) == '(':
			s.skipArithmetic()
		case c == '$' && s.at(1) == '(':
			s.body(')', 2, collect)
		case inDouble:
			s.pos++
		case c == '#' && s.startsWord():
			s.skipComment()
		case (c == '<' || c == '>') && s.at(1) == '(':
			s.body(')', 2, collect)
		case strings.HasPrefix(s.src[s.pos:], hereString):
			s.pos += len(hereString)
		case c == '<' && s.at(1) == '<':
			s.readHeredocMark()
		case c == '\n' && len(s.pending) > 0:
			s.pos++
			s.skipHeredocBodies(collect)
		case c == '(' && s.opensSubshell():
			s.body(')', 1, collect)
		case c == '(':
			depth++
			s.pos++
		case c == ')' && depth > 0:
			depth--
			s.pos++
		case c == ')' && closer == ')':
			s.pos++
			return true
		default:
			s.pos++
		}
	}
	return false
}

func (s *scriptScan) at(offset int) byte {
	if s.pos+offset < len(s.src) {
		return s.src[s.pos+offset]
	}
	return 0
}

// startsWord reports whether the byte at pos begins a word.
func (s *scriptScan) startsWord() bool {
	return s.pos == 0 || strings.IndexByte(wordStartPrev, s.src[s.pos-1]) >= 0
}

// skipComment passes the text up to the end of the line. A quote in it is text.
// It keeps the newline, so a pending heredoc still starts there.
func (s *scriptScan) skipComment() {
	end := strings.IndexByte(s.src[s.pos:], '\n')
	if end < 0 {
		s.pos = len(s.src)
		return
	}
	s.pos += end
}

func (s *scriptScan) skipSingle() {
	end := strings.IndexByte(s.src[s.pos+1:], '\'')
	if end < 0 {
		s.pos = len(s.src)
		return
	}
	s.pos += end + 2
}

// skipANSIC passes a $'...' string. In it a backslash escapes the next byte.
// It records the offset of the quote byte of each backslash quote.
func (s *scriptScan) skipANSIC() {
	s.pos += 2
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '\\':
			if s.at(1) == '\'' {
				s.escaped = append(s.escaped, s.pos+1)
			}
			s.pos += 2
		case '\'':
			s.pos++
			return
		default:
			s.pos++
		}
	}
	s.pos = len(s.src)
}

func (s *scriptScan) opensSubshell() bool {
	return s.pos == 0 || strings.IndexByte(subshellPrev, s.src[s.pos-1]) >= 0
}

// body walks a nested part that opens with width bytes and keeps its text.
func (s *scriptScan) body(closer byte, width int, collect bool) {
	s.pos += width
	start := s.pos
	inner := &scriptScan{src: s.src, pos: start}
	closed := inner.run(closer, false)
	s.escaped = append(s.escaped, inner.escaped...)
	s.pos = min(inner.pos, len(s.src))
	end := s.pos
	if closed {
		end--
	}
	if collect && start <= end {
		s.bodies = append(s.bodies, s.src[start:end])
	}
}

func (s *scriptScan) skipArithmetic() {
	depth := 0
	for ; s.pos < len(s.src); s.pos++ {
		switch s.src[s.pos] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				s.pos++
				return
			}
		}
	}
}

func (s *scriptScan) readHeredocMark() {
	s.pos += 2
	mark := heredocMark{stripTabs: s.at(0) == '-'}
	if mark.stripTabs {
		s.pos++
	}
	for s.at(0) == ' ' || s.at(0) == '\t' {
		s.pos++
	}
	var word strings.Builder
	for s.pos < len(s.src) && strings.IndexByte(delimiterEnd, s.src[s.pos]) < 0 {
		if c := s.src[s.pos]; strings.IndexByte(quoteRunes, c) >= 0 {
			mark.quoted = true
		} else {
			word.WriteByte(c)
		}
		s.pos++
	}
	mark.delimiter = word.String()
	s.pending = append(s.pending, mark)
}

// skipHeredocBodies passes each pending heredoc body. A quoted body is text.
// An unquoted body keeps its substitutions and backtick parts, as bash runs them.
func (s *scriptScan) skipHeredocBodies(collect bool) {
	marks := s.pending
	s.pending = nil
	for _, mark := range marks {
		bodyEnd, next := s.heredocExtent(mark)
		if !mark.quoted {
			s.walkHeredocText(bodyEnd, collect)
		}
		s.pos = max(s.pos, next)
	}
}

// heredocExtent returns the offset where the body ends, which is the start of
// the delimiter line, and the offset after the delimiter line.
func (s *scriptScan) heredocExtent(mark heredocMark) (int, int) {
	pos := s.pos
	for pos < len(s.src) {
		lineStart := pos
		line := s.src[pos:]
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line = line[:end]
			pos += end + 1
		} else {
			pos = len(s.src)
		}
		if mark.stripTabs {
			line = strings.TrimLeft(line, "\t")
		}
		if line == mark.delimiter {
			return lineStart, pos
		}
	}
	return len(s.src), len(s.src)
}

// walkHeredocText collects the substitutions and backtick parts of an unquoted
// body. A quote in the body is text.
func (s *scriptScan) walkHeredocText(end int, collect bool) {
	for s.pos < end {
		switch c := s.src[s.pos]; {
		case c == '\\':
			s.pos += 2
		case c == '`':
			s.body('`', 1, collect)
		case c == '$' && s.at(1) == '(' && s.at(2) == '(':
			s.skipArithmetic()
		case c == '$' && s.at(1) == '(':
			s.body(')', 2, collect)
		default:
			s.pos++
		}
	}
}
