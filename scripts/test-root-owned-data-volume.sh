#!/bin/sh
set -eu

IMAGE="${1:-}"
if [ -z "$IMAGE" ]; then
  echo "usage: $0 <image>" >&2
  exit 2
fi

TMPDIR="$(mktemp -d)"
NAME="cpa-root-owned-smoke-$$"
PARENT_DIR="$(dirname "$TMPDIR")"
BASE_NAME="$(basename "$TMPDIR")"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker run --rm -v "$PARENT_DIR:/parent" alpine sh -lc "rm -rf /parent/$BASE_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run --rm -v "$TMPDIR:/mnt" alpine sh -lc "chown 0:0 /mnt && chmod 755 /mnt" >/dev/null

echo "data_dir=$(stat -c %u:%g:%a "$TMPDIR")"

docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" -v "$TMPDIR:/data" "$IMAGE" >/dev/null
sleep 4

STATUS="$(docker inspect -f '{{.State.Status}}' "$NAME")"
echo "container_status=$STATUS"

if [ "$STATUS" != "running" ]; then
  echo "container failed to stay running" >&2
  docker logs "$NAME" 2>&1 || true
  exit 1
fi

if [ ! -f "$TMPDIR/config.yaml" ]; then
  echo "config file was not created under root-owned data volume" >&2
  docker logs "$NAME" 2>&1 || true
  exit 1
fi

echo "config_mode=$(stat -c %a "$TMPDIR/config.yaml")"
docker logs "$NAME" 2>&1 | tail -n 20
