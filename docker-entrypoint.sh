#!/bin/sh
set -eu

DATA_DIR="${WRITABLE_PATH:-/data}"
DEFAULT_CONFIG_PATH="${CONFIG_PATH:-$DATA_DIR/config.yaml}"
TARGET_UID=1000
TARGET_GID=1000
TARGET_USER=app

can_write_as_target() {
  su "$TARGET_USER" -s /bin/sh -c 'touch "$1/.write-check" && rm -f "$1/.write-check"' sh "$DATA_DIR"
}

run_as_target() {
  exec su "$TARGET_USER" /app/run-as-user.sh -- "$@"
}

run_as_current() {
  exec "$@"
}

if [ "$#" -eq 0 ]; then
  set -- /app/CLIProxyAPI -config "$DEFAULT_CONFIG_PATH"
elif [ "${1#-}" != "$1" ]; then
  set -- /app/CLIProxyAPI "$@"
fi

mkdir -p "$DATA_DIR"

if [ "$(id -u)" -ne 0 ]; then
  run_as_current "$@"
fi

chown -R "${TARGET_UID}:${TARGET_GID}" "$DATA_DIR" 2>/dev/null || true

if can_write_as_target; then
  run_as_target "$@"
fi

echo "warning: $DATA_DIR is not writable by ${TARGET_UID}:${TARGET_GID}; continuing as root" >&2
run_as_current "$@"
