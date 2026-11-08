package domain

import (
	"regexp"
	"strings"
)

const (
	redacted     = "<redacted>"
	homeMark     = "~"
	gitSSHPrefix = "git@"
	pathSlash    = "/"
	encodedChar  = "-"
	secretWord   = `(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|private[_-]?key)`
	quotedValue  = `"[^"\n]*"|'[^'\n]*'`
)

type rule struct {
	pattern     *regexp.Regexp
	replacement string
}

var rules = []rule{
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), redacted},
	{regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s:@]*:[^/\s@]+@`), "${1}" + redacted + "@"},
	{regexp.MustCompile(`(?i)(authorization["']?[ \t]*:[ \t]*["']?(?:bearer|basic|token))[ \t]+[^\s"']+`), "${1} " + redacted},
	{regexp.MustCompile(`(?i)\b(bearer)[ \t]+[A-Za-z0-9._~+/=-]{8,}`), "${1} " + redacted},
	{regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})`), redacted},
	{regexp.MustCompile(`\b(xox[abprs]-[A-Za-z0-9-]{10,}|glpat-[A-Za-z0-9_-]{20,}|npm_[A-Za-z0-9]{36})`), redacted},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`), redacted},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), redacted},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`), redacted},
	{
		regexp.MustCompile(`(?i)\b([a-z0-9_]*` + secretWord + `(?:_[a-z0-9_]*)?)(["']?[ \t]*[:=][ \t]*)(?:` + quotedValue + `|["']?[^\s"',=][^\s"',]*["']?)`),
		"${1}${2}" + redacted,
	},
	{
		regexp.MustCompile(`(?i)(^|\s)(--?[a-z0-9-]*` + secretWord + `(?:-[a-z0-9-]*)?)[ \t]+(?:` + quotedValue + `|["']?[^\s"'-][^\s"']*["']?)`),
		"${1}${2} " + redacted,
	},
	{regexp.MustCompile(`(/Users|/home)/[A-Za-z0-9._-]+`), homeMark},
}

var (
	email         = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	scpRemotePath = regexp.MustCompile(`^:\S`)
	pathEncoded   = regexp.MustCompile(`[^A-Za-z0-9]`)
)

// Redact replaces secrets with <redacted>, and the home folder, also in its dash-encoded form, with ~.
func Redact(text, home string) string {
	home = strings.TrimRight(home, pathSlash)
	if home != "" {
		text = replaceName(text, home,
			namePart,
			func(rest string) bool { return rest[0] == '-' || extendsName(rest) })
		text = replaceName(text, pathEncoded.ReplaceAllString(home, encodedChar),
			func(b byte) bool { return namePart(b) || b == '-' },
			extendsName)
	}
	for _, r := range rules {
		text = r.pattern.ReplaceAllString(text, r.replacement)
	}
	// The email pass runs after the rules, because it would eat the user:pass@host of a URL credential.
	var out strings.Builder
	last := 0
	for _, loc := range email.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		out.WriteString(text[last:start])
		if strings.HasPrefix(text[start:end], gitSSHPrefix) && scpRemotePath.MatchString(text[end:]) {
			out.WriteString(text[start:end])
		} else {
			out.WriteString(redacted)
		}
		last = end
	}
	out.WriteString(text[last:])
	return out.String()
}

func replaceName(text, name string, extendsBefore func(byte) bool, extendsAfter func(rest string) bool) string {
	var out strings.Builder
	last, from := 0, 0
	for {
		i := strings.Index(text[from:], name)
		if i < 0 {
			break
		}
		start, end := from+i, from+i+len(name)
		if (start == 0 || !extendsBefore(text[start-1])) && (end == len(text) || !extendsAfter(text[end:])) {
			out.WriteString(text[last:start])
			out.WriteString(homeMark)
			last = end
		}
		from = end
	}
	out.WriteString(text[last:])
	return out.String()
}

func extendsName(rest string) bool {
	if rest[0] == '.' {
		return len(rest) > 1 && namePart(rest[1])
	}
	return namePart(rest[0])
}

func namePart(b byte) bool {
	return b == '.' || b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9'
}
