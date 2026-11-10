package domain

import (
	"strings"
	"unicode"
)

type Command struct {
	Args      []string
	Redirects []string
	Inputs    []string
	Heredoc   string
	Op        string
}

type redirectMode int

const (
	redirectNone redirectMode = iota
	redirectWrite
	redirectInput
	redirectIgnore
)

type heredocWait struct {
	delimiter string
	stripTabs bool
	command   int
}

type shellParser struct {
	src        []rune
	pos        int
	out        []Command
	cur        Command
	curHeredoc bool
	mode       redirectMode
	waiting    []heredocWait
}

func Commands(line string) []Command {
	p := &shellParser{src: []rune(line)}
	p.run()
	p.flush()
	return p.out
}

func (p *shellParser) run() {
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == ' ' || c == '\t' || c == '\r':
			p.pos++
		case c == '\\' && p.peek(1) == '\n':
			p.pos += 2
		case c == '\n':
			p.flush()
			p.pos++
			p.readHeredocBodies()
		case c == ';':
			p.flush()
			p.pos++
		case c == '|':
			p.flush()
			p.pos++
			p.skipIf('|')
		case c == '&' && p.peek(1) == '>':
			p.pos++
			p.redirect()
		case c == '&':
			p.flush()
			p.pos++
			p.skipIf('&')
		case c == '>':
			p.redirect()
		case c == '<':
			p.input()
		default:
			p.word()
		}
	}
}

func (p *shellParser) peek(offset int) rune {
	if p.pos+offset < len(p.src) {
		return p.src[p.pos+offset]
	}
	return 0
}

func (p *shellParser) skipIf(r rune) {
	if p.peek(0) == r {
		p.pos++
	}
}

func (p *shellParser) flush() {
	if len(p.cur.Args) > 0 || len(p.cur.Redirects) > 0 || p.curHeredoc {
		p.out = append(p.out, p.cur)
	}
	p.cur = Command{}
	p.curHeredoc = false
	p.mode = redirectNone
}

func (p *shellParser) redirect() {
	p.pos++
	p.skipIf('>')
	p.mode = redirectWrite
	if p.peek(0) == '&' {
		p.pos++
		p.mode = redirectIgnore
	}
}

func (p *shellParser) input() {
	switch {
	case p.peek(1) == '<' && p.peek(2) == '<':
		p.pos += 3
	case p.peek(1) == '<':
		p.heredoc()
	default:
		p.pos++
		p.mode = redirectInput
	}
}

func (p *shellParser) heredoc() {
	w := p.heredocStart()
	w.command = len(p.out)
	p.waiting = append(p.waiting, w)
	p.curHeredoc = true
}

func (p *shellParser) heredocStart() heredocWait {
	p.pos += 2
	stripTabs := p.peek(0) == '-'
	if stripTabs {
		p.pos++
	}
	for p.peek(0) == ' ' || p.peek(0) == '\t' {
		p.pos++
	}
	return heredocWait{delimiter: p.scan(), stripTabs: stripTabs}
}

func (p *shellParser) readHeredocBodies() {
	for _, w := range p.waiting {
		p.out[w.command].Heredoc, _ = p.heredocBody(w)
	}
	p.waiting = nil
}

func (p *shellParser) heredocBody(w heredocWait) (string, bool) {
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
			return strings.Join(lines, "\n"), true
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), false
}

func (p *shellParser) word() {
	text := p.scan()
	if p.peek(0) == '>' && isDigits(text) {
		return
	}
	switch p.mode {
	case redirectWrite:
		p.cur.Redirects = append(p.cur.Redirects, text)
	case redirectInput:
		p.cur.Inputs = append(p.cur.Inputs, text)
	case redirectIgnore:
	default:
		p.cur.Args = append(p.cur.Args, text)
	}
	p.mode = redirectNone
}

func isDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}

func (p *shellParser) scan() string {
	var b strings.Builder
	// An open quote at the end of a line moves pos past the end.
	defer func() { p.pos = min(p.pos, len(p.src)) }()
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
		case strings.ContainsRune(" \t\r\n;|&<>", c):
			return b.String()
		default:
			b.WriteRune(c)
			p.pos++
		}
	}
	return b.String()
}

func (p *shellParser) scanDoubleQuoted(b *strings.Builder) {
	for p.pos < len(p.src) && p.src[p.pos] != '"' {
		c := p.src[p.pos]
		if c == '$' && p.peek(1) == '(' {
			p.copySubstitution(b)
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

func (p *shellParser) copySubstitution(b *strings.Builder) {
	depth := 0
	var pending []heredocWait
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\'' || c == '"':
			p.copyQuoted(b, c)
			continue
		case c == '<' && p.peek(1) == '<' && p.peek(2) != '<':
			start := p.pos
			pending = append(pending, p.heredocStart())
			b.WriteString(string(p.src[start:p.pos]))
			continue
		case c == '\n' && len(pending) > 0:
			b.WriteRune(c)
			p.pos++
			for _, w := range pending {
				body, terminated := p.heredocBody(w)
				b.WriteString(body)
				if terminated {
					b.WriteString("\n" + w.delimiter + "\n")
				}
			}
			pending = nil
			continue
		case c == '(':
			depth++
		case c == ')':
			depth--
		}
		b.WriteRune(c)
		p.pos++
		if depth == 0 && c == ')' {
			return
		}
	}
}

func (p *shellParser) copyQuoted(b *strings.Builder, quote rune) {
	b.WriteRune(quote)
	p.pos++
	for p.pos < len(p.src) && p.src[p.pos] != quote {
		if quote == '"' && p.src[p.pos] == '\\' && p.pos+1 < len(p.src) {
			b.WriteRune(p.src[p.pos])
			p.pos++
		}
		b.WriteRune(p.src[p.pos])
		p.pos++
	}
	if p.pos < len(p.src) {
		b.WriteRune(quote)
		p.pos++
	}
}
