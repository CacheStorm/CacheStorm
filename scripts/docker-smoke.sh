#!/usr/bin/env bash
# Builds the root Docker image and checks that a container started from it
# runs as the unprivileged user, reports healthy, enforces requirepass from
# the environment, serves commands and reports the stamped version.
#
#   scripts/docker-smoke.sh            # builds cachestorm:smoke
#   IMAGE=ghcr.io/... SKIP_BUILD=1 scripts/docker-smoke.sh
set -euo pipefail

IMAGE="${IMAGE:-cachestorm:smoke}"
VERSION="${VERSION:-smoke-test}"
PASSWORD="smoke-$RANDOM-$RANDOM"
NAME="cachestorm-smoke-$$"

cd "$(dirname "$0")/.."

if [ -z "${SKIP_BUILD:-}" ]; then
	docker build --build-arg VERSION="$VERSION" -t "$IMAGE" .
fi

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		echo "--- container logs"
		docker logs "$NAME" 2>&1 | tail -50 || true
	fi
	docker rm -f "$NAME" >/dev/null 2>&1 || true
	exit "$status"
}
trap cleanup EXIT

docker run -d --name "$NAME" \
	-e CACHESTORM_REQUIREPASS="$PASSWORD" \
	-p 127.0.0.1::6380 -p 127.0.0.1::8080 \
	"$IMAGE" >/dev/null

port="$(docker port "$NAME" 6380/tcp | head -1 | sed 's/.*://')"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# resp sends one command per argument list and prints the raw replies.
resp() {
	local payload=""
	local cmd
	for cmd in "$@"; do
		# shellcheck disable=SC2206
		local words=($cmd)
		payload+="*${#words[@]}\r\n"
		local w
		for w in "${words[@]}"; do
			payload+="\$${#w}\r\n${w}\r\n"
		done
	done
	exec 3<>"/dev/tcp/127.0.0.1/$port"
	printf '%b' "$payload" >&3
	timeout 2 cat <&3 || true
	exec 3<&-
}

for _ in $(seq 1 60); do
	health="$(docker inspect --format '{{.State.Health.Status}}' "$NAME")"
	[ "$health" = "healthy" ] && break
	[ "$(docker inspect --format '{{.State.Running}}' "$NAME")" = "true" ] || fail "container exited"
	sleep 1
done
[ "$health" = "healthy" ] || fail "container not healthy after 60s (status: $health)"
echo "ok: healthy"

uid="$(docker exec "$NAME" id -u)"
[ "$uid" != "0" ] || fail "container runs as root"
echo "ok: runs as uid $uid"

out="$(resp "PING")"
case "$out" in
	-NOAUTH*) echo "ok: unauthenticated PING refused" ;;
	*) fail "PING without AUTH replied: $out" ;;
esac

out="$(resp "AUTH wrong-password" "PING")"
case "$out" in
	-*) echo "ok: wrong password refused" ;;
	*) fail "wrong password accepted: $out" ;;
esac

out="$(resp "AUTH $PASSWORD" "SET smoke:key value" "GET smoke:key" "INFO server")"
case "$out" in
	+OK*+OK*value*cachestorm_version:"$VERSION"*) echo "ok: authenticated SET/GET and version $VERSION" ;;
	*) fail "authenticated session replied: $out" ;;
esac

echo "docker smoke test passed"
