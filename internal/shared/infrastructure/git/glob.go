package git

import (
	"path"
	"strings"
)

const (
	recursiveSegment = "**"
	segmentSeparator = "/"
)

func MatchesAny(patterns []string, name string) bool {
	segments := strings.Split(name, segmentSeparator)
	for _, pattern := range patterns {
		if matchSegments(strings.Split(pattern, segmentSeparator), segments) {
			return true
		}
	}
	return false
}

func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == recursiveSegment {
		for skip := 0; skip <= len(name); skip++ {
			if matchSegments(pattern[1:], name[skip:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], name[0])
	return err == nil && ok && matchSegments(pattern[1:], name[1:])
}
