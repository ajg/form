#!/bin/bash -eu

# TODO: Only colorize messages given a suitable terminal.

printf "\e[30m=== PRE-COMMIT STARTING ===\e[m\n"
git stash save --quiet --keep-index --include-untracked

if go test; then
  result=$?
  printf "\e[32m=== PRE-COMMIT SUCCEEDED ===\e[m\n"
else
  result=$?
  printf "\e[31m=== PRE-COMMIT FAILED ===\e[m\n"
fi

git stash pop --quiet
exit $result
