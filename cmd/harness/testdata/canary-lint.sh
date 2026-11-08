#!/bin/sh
cat >/dev/null
printf '{"verdict":"allow","findings":['
grep -rn 'violates: ' src | {
	sep=''
	while IFS=: read -r file line text; do
		printf '%s{"rule":"%s","severity":"error","file":"%s","line":%s,"message":"The canary breaks the rule."}' "$sep" "${text##*violates: }" "$file" "$line"
		sep=','
	done
}
printf ']}\n'
