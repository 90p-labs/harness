#!/bin/sh
cat >/dev/null
printf '{"verdict":"allow","findings":['
grep -rn 'TODO' src | {
	sep=''
	while IFS=: read -r file line _; do
		printf '%s{"rule":"team/no-todo","severity":"error","file":"%s","line":%s,"message":"A TODO is left in the code."}' "$sep" "$file" "$line"
		sep=','
	done
}
printf ']}\n'
