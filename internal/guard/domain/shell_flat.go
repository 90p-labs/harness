package domain

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const (
	opNone       = ""
	opSemi       = ";"
	opNewline    = "\n"
	opAnd        = "&&"
	opOr         = "||"
	opPipe       = "|"
	opBackground = "&"

	subOpen  = "$("
	assignOp = "="
	resSep   = " "
	cmdDot   = "."

	quoteChars         = "'\"\\"
	doubleQuoteEscapes = "\"\\$`"

	splitOrGlob = " \t\n*?["
	wordEnd     = " \t\n;|&<>"
	nonFlat     = "(){}`"

	delimiterUnsafe = "$\n"

	specialParams = "?$!#@*-0123456789"
)

var (
	nonFlatWords = []string{"if", "case", "for", "while", "until", "repeat", "coproc", "select", "function"}

	formCommands = []string{"trap", "unset", "builtin", "command", "exec", "alias", "mapfile", "readarray"}

	stateWords = []string{
		"set", "setopt", "unsetopt", "shopt", "emulate", "let", "read", "declare", "typeset", "local",
		"export", "readonly", "eval", "source", "chdir", "pushd", "popd",
	}

	formVariables = []string{"IFS", "CDPATH", "cdpath", "HOME", "PWD", "OLDPWD", "BASH_ENV", "ENV", "_"}

	literalName        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\+?$`)
	formAssignment     = regexp.MustCompile(`^(` + strings.Join(formVariables, "|") + `)\+?=`)
	compoundAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\+?=\(|\[[^\]]*\]\+?=)`)
	nameToken          = regexp.MustCompile(`[A-Za-z0-9_]+`)
	flagLead           = regexp.MustCompile(`^-([A-Za-z0-9_]+)`)
)

type safeUse struct {
	at       int
	text     string
	resolved bool
}

type wordBuf struct {
	strings.Builder
	safe []safeUse
}

func (b *wordBuf) text() string {
	return b.splice(func(u safeUse) string { return u.text })
}

func (b *wordBuf) splice(fill func(safeUse) string) string {
	raw, last := b.String(), 0
	var out strings.Builder
	for _, u := range b.safe {
		out.WriteString(raw[last:u.at])
		out.WriteString(fill(u))
		last = u.at
	}
	out.WriteString(raw[last:])
	return out.String()
}

type flatRedirect int

const (
	flatRedirectNone flatRedirect = iota
	flatRedirectWrite
	flatRedirectInput
	flatRedirectIgnore
)

type flatHeredoc struct {
	delimiter string
	stripTabs bool
	command   int
}

type flatParser struct {
	src        []rune
	pos        int
	out        []Command
	cur        Command
	curHeredoc bool
	mode       flatRedirect
	waiting    []flatHeredoc
	vars       map[string]string
	scoped     []string
	listed     []string
	prevOp     string
	literal    bool
	beyondForm bool
	unsafe     map[string]bool
	argRes     []string
	argPlain   []bool
	argDollar  []bool
	residual   string
	res        strings.Builder
}

func flatCommands(line string) ([]Command, bool) {
	first := parseFlat(line, true, nil)
	hasControlRune := strings.ContainsFunc(line, func(r rune) bool {
		return (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f
	})
	if first.beyondForm || hasControlRune || outsideSimpleForm(first.out) {
		return nil, false
	}
	return expandStable(line, first.unsafeNames())
}

func expandStable(line string, unsafe map[string]bool) ([]Command, bool) {
	for {
		p := parseFlat(line, false, unsafe)
		if p.beyondForm || outsideSimpleForm(p.out) {
			return nil, false
		}
		grown := false
		for name := range p.unsafeNames() {
			grown = grown || !unsafe[name]
			unsafe[name] = true
		}
		if !grown {
			return p.out, true
		}
	}
}

func parseFlat(line string, literal bool, unsafe map[string]bool) *flatParser {
	p := &flatParser{src: []rune(line), vars: map[string]string{}, literal: literal, unsafe: unsafe}
	p.run()
	p.flush(opNone)
	return p
}

func (p *flatParser) unsafeNames() map[string]bool {
	names := map[string]bool{}
	for _, word := range strings.Fields(p.res.String()) {
		tokens := nameToken.FindAllString(word, -1)
		if lead := flagLead.FindStringSubmatch(word); lead != nil {
			run := lead[1]
			for k := 1; k < len(run) && isNameRune(rune(run[k-1]), false); k++ {
				names[run[k:]] = true
			}
			if word == flagMark+run {
				tokens = tokens[1:]
			}
		}
		for _, t := range tokens {
			names[t] = true
		}
	}
	return names
}

func (p *flatParser) run() {
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == ' ' || c == '\t':
			p.pos++
		case c == '\\' && p.peek(1) == '\n':
			p.pos += 2
		case c == '\n':
			p.flush(opNewline)
			p.pos++
			p.readHeredocBodies()
		case c == ';':
			p.flush(opSemi)
			p.pos++
		case c == '|':
			p.pos++
			p.flushDoubled('|', opPipe, opOr)
		case c == '&' && p.peek(1) == '>':
			p.pos++
			p.redirect()
		case c == '&':
			p.pos++
			p.flushDoubled('&', opBackground, opAnd)
		case c == '>':
			p.redirect()
		case c == '<':
			p.input()
		default:
			p.word()
		}
	}
}

func (p *flatParser) peek(offset int) rune {
	if p.pos+offset < len(p.src) {
		return p.src[p.pos+offset]
	}
	return 0
}

func (p *flatParser) skipIf(r rune) {
	if p.peek(0) == r {
		p.pos++
	}
}

func (p *flatParser) flushDoubled(r rune, single, double string) {
	op := single
	if p.peek(0) == r {
		p.pos++
		op = double
	}
	p.flush(op)
}

func (p *flatParser) flush(op string) {
	if len(p.cur.Args) > 0 || len(p.cur.Redirects) > 0 || p.curHeredoc {
		p.cur.Op = op
		p.track(p.cur)
		p.out = append(p.out, p.cur)
		p.prevOp = op
	}
	for _, w := range p.argRes {
		p.res.WriteString(w + resSep)
	}
	p.argRes, p.argPlain, p.argDollar = nil, nil, nil
	p.cur = Command{}
	p.curHeredoc = false
	p.mode = flatRedirectNone
}

func (p *flatParser) track(c Command) {
	if len(c.Args) > 0 && !slices.ContainsFunc(c.Args, func(a string) bool { return !envAssignment.MatchString(a) }) {
		p.assign(c)
	}
	if c.Op != opAnd && c.Op != opPipe {
		p.forget(p.scoped)
		p.scoped = nil
	}
	switch c.Op {
	case opBackground:
		p.forget(p.listed)
		p.listed = nil
	case opSemi, opNewline:
		p.listed = nil
	}
}

func (p *flatParser) forget(names []string) {
	for _, name := range names {
		delete(p.vars, name)
	}
}

func (p *flatParser) assign(c Command) {
	args := c.Args
	sequential := isSequential(p.prevOp)
	redirected := len(c.Redirects) > 0 || len(c.Inputs) > 0 || p.curHeredoc
	leftToRight := len(args) > 1 && slices.Contains(p.argDollar, true)
	surely := !redirected && !leftToRight &&
		c.Op != opPipe && c.Op != opBackground && (sequential || p.prevOp == opAnd)
	for i, a := range args {
		name, value, _ := strings.Cut(a, assignOp)
		if !surely || !p.argPlain[i] || value == "" || strings.ContainsAny(value, splitOrGlob) {
			delete(p.vars, name)
			continue
		}
		p.argRes[i] = strings.TrimPrefix(p.argRes[i], name+assignOp)
		p.vars[name] = value
		p.listed = append(p.listed, name)
		if !sequential {
			p.scoped = append(p.scoped, name)
		}
	}
}

func (p *flatParser) redirect() {
	p.pos++
	p.skipIf('>')
	p.mode = flatRedirectWrite
	if p.peek(0) == '&' {
		p.pos++
		p.mode = flatRedirectIgnore
	}
}

func (p *flatParser) input() {
	switch {
	case p.peek(1) == '<' && p.peek(2) == '<':
		p.pos += 3
	case p.peek(1) == '<':
		p.heredoc()
	default:
		p.pos++
		p.mode = flatRedirectInput
	}
}

func (p *flatParser) heredoc() {
	w := p.heredocStart()
	w.command = len(p.out)
	p.waiting = append(p.waiting, w)
	p.curHeredoc = true
}

func (p *flatParser) heredocStart() flatHeredoc {
	p.pos += 2
	stripTabs := p.peek(0) == '-'
	if stripTabs {
		p.pos++
	}
	for p.peek(0) == ' ' || p.peek(0) == '\t' {
		p.pos++
	}
	start, wasLiteral := p.pos, p.literal
	p.literal = true
	delimiter := p.scan()
	p.literal = wasLiteral
	raw := string(p.src[start:p.pos])
	p.beyondForm = p.beyondForm || !strings.ContainsAny(raw, quoteChars) || strings.ContainsAny(raw, delimiterUnsafe)
	return flatHeredoc{delimiter: delimiter, stripTabs: stripTabs}
}

func (p *flatParser) fill(u safeUse) string {
	if !u.resolved {
		return resSep
	}
	return u.text
}

func (p *flatParser) readHeredocBodies() {
	for _, w := range p.waiting {
		p.out[w.command].Heredoc = p.heredocBody(w)
	}
	p.waiting = nil
}

func (p *flatParser) heredocBody(w flatHeredoc) string {
	var lines []string
	for p.pos < len(p.src) {
		end := p.pos
		for end < len(p.src) && p.src[end] != '\n' {
			end++
		}
		line := string(p.src[p.pos:end])
		p.pos = min(end+1, len(p.src))
		terminator := line
		if w.stripTabs {
			terminator = strings.TrimLeft(line, "\t")
		}
		if terminator == w.delimiter {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (p *flatParser) word() {
	start := p.pos
	text := p.scan()
	if p.peek(0) == '>' && isDigits(text) {
		return
	}
	raw := string(p.src[start:p.pos])
	p.beyondForm = p.beyondForm || slices.Contains(nonFlatWords, raw)
	namePart, _, _ := strings.Cut(raw, assignOp)
	plain := literalName.MatchString(namePart)
	p.beyondForm = p.beyondForm || (p.mode == flatRedirectNone && p.breaksWord(text))
	switch p.mode {
	case flatRedirectWrite:
		p.cur.Redirects = append(p.cur.Redirects, text)
	case flatRedirectInput:
		p.cur.Inputs = append(p.cur.Inputs, text)
	case flatRedirectIgnore:
	default:
		p.cur.Args = append(p.cur.Args, text)
		p.argRes = append(p.argRes, p.residual)
		p.argPlain = append(p.argPlain, plain)
		p.argDollar = append(p.argDollar, strings.ContainsRune(raw, '$'))
	}
	if p.mode != flatRedirectNone {
		p.res.WriteString(p.residual + resSep)
	}
	p.mode = flatRedirectNone
}

func (p *flatParser) breaksWord(text string) bool {
	first := true
	for i, a := range p.cur.Args {
		first = first && p.argPlain[i] && envAssignment.MatchString(a)
	}
	cdLike := filepath.Base(text) == cmdCd && (!first || text != cmdCd)
	return slices.Contains(stateWords, text) || (first && text == cmdDot) || cdLike
}

func (p *flatParser) scan() string {
	var b wordBuf
	defer func() {
		// An open quote at the end of a line moves pos past the end.
		p.pos = min(p.pos, len(p.src))
		p.residual = b.splice(p.fill)
	}()
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\'':
			p.pos++
			for p.pos < len(p.src) && p.src[p.pos] != '\'' {
				b.WriteRune(p.src[p.pos])
				p.pos++
			}
			p.pos++
		case c == '"':
			p.pos++
			p.scanDoubleQuoted(&b)
		case c == '\\':
			p.pos++
			if p.peek(0) != '\n' && p.pos < len(p.src) {
				b.WriteRune(p.src[p.pos])
			}
			p.pos++
		case strings.ContainsRune(wordEnd, c):
			return b.text()
		case c == '$' && p.peek(1) == '(':
			p.beyondForm = true
			b.WriteString(subOpen)
			p.pos += 2
		case c == '$':
			p.beyondForm = p.beyondForm || p.peek(1) == '"'
			p.expand(&b)
		default:
			p.beyondForm = p.beyondForm || strings.ContainsRune(nonFlat, c) || (c == '#' && b.Len() == 0)
			b.WriteRune(c)
			p.pos++
		}
	}
	return b.text()
}

func (p *flatParser) expand(b *wordBuf) {
	name, width := p.variableAt()
	inertDollar := strings.ContainsRune(wordEnd+`"`+specialParams, p.peek(1)) || p.pos+1 >= len(p.src)
	if !inertDollar && (name == "" || p.peek(width) == '[') {
		p.beyondForm = true
	}
	if name == "" || p.peek(width) == ':' || p.peek(width) == '[' {
		b.WriteRune('$')
		p.pos++
		return
	}
	text := string(p.src[p.pos : p.pos+width])
	value, known := p.vars[name]
	resolved := known && !p.unsafe[name] && !p.literal
	if resolved {
		text = value
	}
	b.safe = append(b.safe, safeUse{at: b.Len(), text: text, resolved: resolved})
	p.pos += width
}

func (p *flatParser) variableAt() (string, int) {
	start, braced := p.pos+1, p.peek(1) == '{'
	if braced {
		start++
	}
	end := start
	for end < len(p.src) && isNameRune(p.src[end], end > start) {
		end++
	}
	width := end - p.pos
	switch {
	case braced && (end >= len(p.src) || p.src[end] != '}'):
		return "", 0
	case braced:
		width++
	}
	return string(p.src[start:end]), width
}

func isNameRune(r rune, allowDigit bool) bool {
	return r == '_' || (r < unicode.MaxASCII && unicode.IsLetter(r)) || (allowDigit && unicode.IsDigit(r) && r < unicode.MaxASCII)
}

func (p *flatParser) scanDoubleQuoted(b *wordBuf) {
	for p.pos < len(p.src) && p.src[p.pos] != '"' {
		c := p.src[p.pos]
		p.beyondForm = p.beyondForm || c == '`'
		if c == '$' {
			p.expand(b)
			continue
		}
		if c == '\\' && strings.ContainsRune(doubleQuoteEscapes, p.peek(1)) {
			p.pos++
			c = p.src[p.pos]
		}
		b.WriteRune(c)
		p.pos++
	}
	p.pos++
}

func outsideSimpleForm(commands []Command) bool {
	return slices.ContainsFunc(commands, func(c Command) bool {
		return slices.ContainsFunc(c.Args, isFormAssignment) || breaksForm(c.Args) || breaksForm(programArgs(c.Args))
	})
}

func isFormAssignment(arg string) bool {
	return formAssignment.MatchString(arg) || compoundAssignment.MatchString(arg)
}

func breaksForm(head []string) bool {
	if len(head) == 0 {
		return false
	}
	return slices.Contains(formCommands, head[0])
}
