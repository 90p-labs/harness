package domain

import "strings"

func ToolBaseName(name string, known func(string) bool) string {
	if known(name) {
		return name
	}
	rest, ok := strings.CutPrefix(name, "mcp__")
	if !ok {
		return name
	}
	cut := strings.LastIndex(rest, "__")
	if cut <= 0 {
		return name
	}
	server, segment := rest[:cut], rest[cut+2:]
	if segment == "" || strings.Contains(server, "__") {
		return name
	}
	if known(segment) {
		return segment
	}
	word, base, _ := strings.Cut(segment, "_")
	if word != "" && isLowerASCII(word) && known(base) {
		return base
	}
	return name
}

func isLowerASCII(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
