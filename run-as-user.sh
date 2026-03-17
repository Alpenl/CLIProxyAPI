#!/bin/sh
set -eu

cmd="${1:-}"
if [ -z "$cmd" ]; then
  echo "run-as-user.sh: missing command" >&2
  exit 2
fi

shift
exec "$cmd" "$@"
