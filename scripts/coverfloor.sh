#!/bin/sh
# Fails when the total statement coverage of a Go coverage profile is below a floor.
# Reads the profile with awk and sums raw statement counts, so 84.96 never rounds up to 85.
#
# usage: coverfloor.sh <profile> <floor>
set -eu

usage() {
	echo "usage: coverfloor.sh <profile> <floor>" >&2
	exit 2
}

refuse() {
	echo "coverfloor: $*" >&2
	exit 2
}

[ $# -eq 2 ] || usage
profile=$1
floor=$2

# A case pattern matches the whole value, so an embedded newline cannot hide behind a valid line.
# The floor is digits with an optional dot and digits.
case $floor in
'' | *[!0-9.]* | .* | *. | *.*.*) refuse "the floor must be a number such as 85 or 85.5, got: $floor" ;;
esac
[ -f "$profile" ] && [ -r "$profile" ] || refuse "cannot read the profile: $profile"

# A block is "file:start.col,end.col numStmts count". The same block can repeat, so keep the
# highest count per block and count its statements once. The compare is cross multiplied so that
# the raw ratio decides, and the printed total is truncated to one decimal.
# The first line must be the mode line. Every non-empty line after it must have exactly three
# fields, and the last two must be unsigned integers.
result=$(awk -v floor="$floor" '
	NR == 1 && /^mode:/ { next }
	NR == 1 {
		print "nomode"
		bad = 1
		exit
	}
	NF == 0 { next }
	NF != 3 || !($2 ~ /^[0-9]+$/) || !($3 ~ /^[0-9]+$/) {
		print "malformed " NR
		bad = 1
		exit
	}
	{
		key = $1
		if (!(key in stmts)) {
			stmts[key] = $2
			hits[key] = $3
		} else if ($3 > hits[key]) {
			hits[key] = $3
		}
	}
	END {
		if (bad) {
			exit
		}
		all = 0
		covered = 0
		for (key in stmts) {
			all += stmts[key]
			if (hits[key] > 0) {
				covered += stmts[key]
			}
		}
		if (all == 0) {
			print "empty"
			exit
		}
		status = (covered * 100 < floor * all) ? "below" : "ok"
		tenths = int(covered * 1000 / all)
		printf "%d.%d %s\n", int(tenths / 10), tenths % 10, status
	}
' "$profile") || refuse "cannot parse the profile: $profile"

case $result in
nomode) refuse "the first line of the profile must be a mode line such as mode: set: $profile" ;;
malformed*) refuse "line ${result#malformed } of the profile is malformed, it needs exactly three fields with unsigned integer counts: $profile" ;;
esac
[ "$result" != empty ] || refuse "the profile has no statements: $profile"

total=${result% *}
status=${result#* }
echo "coverage: ${total}% (floor ${floor}%)"
if [ "$status" = below ]; then
	echo "coverfloor: coverage ${total}% is below the floor of ${floor}%" >&2
	exit 1
fi
