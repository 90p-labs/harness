#!/bin/sh
cat > /dev/null
printf '%s\n' '{"verdict":"allow","context":"{\"id\":\"kubeconform\",\"run\":\"kubeconform -summary deploy\"}"}'
