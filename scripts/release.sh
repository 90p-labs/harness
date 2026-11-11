#!/bin/sh
# Picks the next release version from conventional commits, writes the plugin manifests and builds the release notes.
#
# usage: release.sh next | notes <tag> | bump <tag> | check <tag>
set -eu

manifests=".claude-plugin/plugin.json .codex-plugin/plugin.json"
tag_pattern='^v[0-9]+\.[0-9]+\.[0-9]+$'

usage() {
	echo "usage: release.sh next | notes <tag> | bump <tag> | check <tag>" >&2
	exit 2
}

die() {
	echo "release: $*" >&2
	exit 1
}

valid_tag() {
	case $1 in *"
"*) return 1 ;; esac
	printf '%s\n' "$1" | grep -Eq "$tag_pattern"
}

need_tag() {
	[ $# -eq 1 ] || usage
	valid_tag "$1" || die "the tag must look like v1.2.3, got: $1"
}

# Same parse as read_version in bin/harness: the first line with "version".
read_version() {
	version=
	while IFS= read -r line; do
		case $line in
			*'"version"'*) version=${line#*\"version\"}; version=${version#*\"}; version=${version%%\"*}; break ;;
		esac
	done <"$1" || return 1
	[ -n "$version" ] || return 1
	printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || return 1
	printf '%s\n' "$version"
}

# The last valid version tag that is an ancestor of the commit, not counting the skipped tag.
previous_tag() {
	git tag --merged "$1" 2>/dev/null |
		grep -E "$tag_pattern" |
		grep -vxF "${2:-}" |
		sort -t. -k1.2,1n -k2,2n -k3,3n |
		tail -n 1 || true
}

# Prints breaking, feat, fix or other for one commit. A malformed subject is other, even with a footer.
classify() {
	subject=$(git log -1 --format=%s "$1")
	parsed=$(printf '%s\n' "$subject" | sed -n -E 's/^([a-z]+)(\([^()]+\))?(!?): .+$/\1 \3/p')
	[ -n "$parsed" ] || { echo other; return; }
	type=${parsed%% *}
	bang=${parsed#* }
	if [ -n "$bang" ] || git log -1 --format=%b "$1" | grep -Eq '^BREAKING[ -]CHANGE:'; then
		echo breaking
		return
	fi
	case $type in
		feat | fix) echo "$type" ;;
		*) echo other ;;
	esac
}

cmd_next() {
	base=$(previous_tag HEAD)
	if [ -n "$base" ]; then
		range="$base..HEAD"
	else
		current=$(read_version .claude-plugin/plugin.json) || die "no valid version in .claude-plugin/plugin.json"
		base=v$current
		range=HEAD
	fi
	level=0
	for sha in $(git log --no-merges --format=%H "$range" 2>/dev/null || true); do
		case $(classify "$sha") in
			breaking) level=3 ;;
			feat) [ "$level" -ge 2 ] || level=2 ;;
			fix) [ "$level" -ge 1 ] || level=1 ;;
		esac
	done
	[ "$level" -gt 0 ] || return 0
	numbers=${base#v}
	major=${numbers%%.*}
	rest=${numbers#*.}
	minor=${rest%%.*}
	patch=${rest#*.}
	if [ "$level" -eq 3 ] && [ "$major" -ge 1 ]; then
		echo "v$((major + 1)).0.0"
	elif [ "$level" -ge 2 ]; then
		echo "v$major.$((minor + 1)).0"
	else
		echo "v$major.$minor.$((patch + 1))"
	fi
}

cmd_notes() {
	need_tag "$@"
	tag=$1
	end=$(git rev-parse -q --verify "refs/tags/$tag^{commit}" || echo HEAD)
	previous=$(previous_tag "$end" "$tag")
	if [ -n "$previous" ]; then
		range="$previous..$end"
	else
		range=$end
	fi
	nl='
'
	breaking='' features='' fixes='' other=''
	shas=$(git log --no-merges --format=%H "$range") || die "cannot read the log of $range"
	for sha in $shas; do
		subject=$(git log -1 --format=%s "$sha")
		case $subject in "chore(release):"*) continue ;; esac
		line="- $subject ($(git rev-parse --short "$sha"))$nl"
		case $(classify "$sha") in
			breaking) breaking=$breaking$line ;;
			feat) features=$features$line ;;
			fix) fixes=$fixes$line ;;
			*) other=$other$line ;;
		esac
	done
	first=1
	for section in "Breaking changes:$breaking" "Features:$features" "Fixes:$fixes" "Other changes:$other"; do
		title=${section%%:*}
		body=${section#*:}
		[ -n "$body" ] || continue
		[ "$first" -eq 1 ] || printf '\n'
		first=0
		printf '## %s\n\n%s' "$title" "$body"
	done
}

cmd_bump() {
	need_tag "$@"
	version=${1#v}
	for file in $manifests; do
		[ -f "$file" ] || die "$file is missing"
		tmp=$(mktemp "$file.XXXXXX")
		awk -v v="$version" '
			!done && /"version"/ {
				sub(/"version"[ \t]*:[ \t]*"[^"]*"/, "\"version\": \"" v "\"")
				done = 1
			}
			{ print }
		' "$file" >"$tmp" || { rm -f "$tmp"; die "cannot write $file"; }
		if [ "$(read_version "$tmp" || true)" != "$version" ]; then
			rm -f "$tmp"
			die "$file has no version line to set"
		fi
		chmod 644 "$tmp"
		mv -f "$tmp" "$file"
	done
}

cmd_check() {
	need_tag "$@"
	want=${1#v}
	status=0
	for file in $manifests; do
		if [ ! -f "$file" ]; then
			echo "release: $file is missing" >&2
			status=1
			continue
		fi
		have=$(read_version "$file") || have=
		if [ "$have" != "$want" ]; then
			echo "release: $file has version ${have:-none}, want $want" >&2
			status=1
		fi
	done
	exit $status
}

[ $# -ge 1 ] || usage
command=$1
shift
case $command in
	next | notes | bump | check) ;;
	*) usage ;;
esac
top=$(git rev-parse --show-toplevel 2>/dev/null) || die "not in a git repository"
cd "$top"
case $command in
	next) [ $# -eq 0 ] || usage; cmd_next ;;
	notes) cmd_notes "$@" ;;
	bump) cmd_bump "$@" ;;
	check) cmd_check "$@" ;;
esac
