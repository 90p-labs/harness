package domain

import (
	"path"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	FolderMigrations = "migrations/"
	SuffixSQL        = ".sql"
	fullFolderCount  = 3
	folderHooks      = "hooks"
	folderPluginHook = "pluginhooks"
)

var hookFolders = []string{folderHooks, folderPluginHook}

func Classify(doc Document) shareddomain.Track {
	declared := shareddomain.Track(doc.Track)
	if touchesManyFolders(doc.CodeTree) || touchesFullFolder(doc.CodeTree) {
		return shareddomain.Stricter(declared, shareddomain.TrackFull)
	}
	return shareddomain.Stricter(declared, shareddomain.TrackLight)
}

func NeedsAdvisor(doc Document) bool {
	return Classify(doc) == shareddomain.TrackFull || hasThreatFolder(doc.CodeTree) || hasHookFolder(doc.CodeTree)
}

// LightReview reports whether a run may skip the full review.
// Both the plan and the changed files must classify light and need no advisor.
func LightReview(doc Document, changed []string) bool {
	if len(changed) == 0 || !isLight(doc) {
		return false
	}
	actual := doc
	actual.CodeTree = changedTree(changed)
	return isLight(actual)
}

// isLight needs a declared track that is empty or exactly light.
// An unknown value such as "ful" ranks like an empty one in Stricter, so it must not count.
func isLight(doc Document) bool {
	if doc.Track != "" && shareddomain.Track(doc.Track) != shareddomain.TrackLight {
		return false
	}
	return Classify(doc) == shareddomain.TrackLight && !NeedsAdvisor(doc)
}

// changedTree builds a tree with one entry for each folder and file of the changed paths.
func changedTree(changed []string) Tree {
	var tree Tree
	seen := map[string]bool{}
	for _, p := range changed {
		segments := strings.Split(strings.Trim(p, "/"), "/")
		for i := range segments[:len(segments)-1] {
			dir := strings.Join(segments[:i+1], "/") + "/"
			if !seen[dir] {
				seen[dir] = true
				tree.Entries = append(tree.Entries, TreeEntry{Path: dir, Depth: i, IsDir: true})
			}
		}
		file := strings.Join(segments, "/")
		if !seen[file] {
			seen[file] = true
			tree.Entries = append(tree.Entries, TreeEntry{Path: file, Depth: len(segments) - 1})
		}
	}
	return tree
}

func hasHookFolder(tree Tree) bool {
	return slices.ContainsFunc(tree.Entries, func(e TreeEntry) bool {
		if e.IsDir {
			return false
		}
		return slices.ContainsFunc(strings.Split(path.Dir(e.Path), "/"), func(s string) bool { return containsFold(hookFolders, s) })
	})
}

func touchesManyFolders(tree Tree) bool {
	folders := map[string]bool{}
	for _, e := range tree.Entries {
		if !e.IsDir {
			folders[path.Dir(e.Path)] = true
		}
	}
	return len(folders) >= fullFolderCount
}

func touchesFullFolder(tree Tree) bool {
	for _, e := range tree.Entries {
		if e.IsDir {
			continue
		}
		if hasSuffixFold(e.Path, SuffixSQL) {
			return true
		}
		dirs := strings.Split(e.Path, "/")
		dirs = dirs[:len(dirs)-1]
		for _, f := range []string{FolderDomain, FolderMigrations, FolderCommand, FolderQuery} {
			if hasSegmentsFold(dirs, strings.Split(strings.Trim(f, "/"), "/")) {
				return true
			}
		}
	}
	return false
}

// containsFold reports whether list holds s, folding case by Unicode simple folding.
func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(item string) bool { return strings.EqualFold(item, s) })
}

func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// hasSegmentsFold reports whether want appears as consecutive segments of dirs, folding case.
func hasSegmentsFold(dirs, want []string) bool {
	for i := 0; i+len(want) <= len(dirs); i++ {
		if slices.EqualFunc(dirs[i:i+len(want)], want, strings.EqualFold) {
			return true
		}
	}
	return false
}
