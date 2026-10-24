#!/usr/bin/env bash
# Builds cachestorm, starts a master and a replica as two processes and checks
# that replication works end to end: the initial full sync, live writes, the
# replica's read-only refusal, INFO replication and promotion.
#
#   scripts/replication-smoke.sh
#   BIN=./cachestorm SKIP_BUILD=1 scripts/replication-smoke.sh
set -euo pipefail

cd "$(dirname "$0")/.."

WORK="$(mktemp -d)"
BIN="${BIN:-$WORK/cachestorm}"
MASTER_PORT="${MASTER_PORT:-$((20000 + RANDOM % 20000))}"
REPLICA_PORT="${REPLICA_PORT:-$((MASTER_PORT + 1))}"
PASSWORD="smoke-$RANDOM"
pids=()

cleanup() {
	status=$?
	for pid in "${pids[@]}"; do
		kill "$pid" 2>/dev/null || true
		wait "$pid" 2>/dev/null || true
	done
	if [ "$status" -ne 0 ]; then
		for log in "$WORK"/*.log; do
			echo "--- $log"
			tail -30 "$log" || true
		done
	fi
	rm -rf "$WORK"
	exit "$status"
}
trap cleanup EXIT

if [ -z "${SKIP_BUILD:-}" ]; then
	go build -o "$BIN" ./cmd/cachestorm
fi

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# write_config <file> <port> [replication lines...]
write_config() {
	local file=$1 port=$2
	shift 2
	{
		echo "server:"
		echo "  bind: 127.0.0.1"
		echo "  port: $port"
		echo "  requirepass: $PASSWORD"
		echo "http:"
		echo "  enabled: false"
		echo "plugins:"
		echo "  metrics:"
		echo "    enabled: false"
		echo "logging:"
		echo "  level: warn"
		echo "replication:"
		for line in "$@"; do
			echo "  $line"
		done
	} >"$file"
}

# cmd <port> <words...> sends one authenticated command and prints the reply.
cmd() {
	local port=$1
	shift
	local payload="*2\r\n\$4\r\nAUTH\r\n\$${#PASSWORD}\r\n${PASSWORD}\r\n*$#\r\n"
	local w
	for w in "$@"; do
		payload+="\$${#w}\r\n${w}\r\n"
	done
	payload+="*1\r\n\$4\r\nQUIT\r\n"
	if ! { exec 3<>"/dev/tcp/127.0.0.1/$port"; } 2>/dev/null; then
		return 0
	fi
	printf '%b' "$payload" >&3
	# The AUTH reply, the command's reply, then QUIT's +OK.
	timeout 5 cat <&3 | tr -d '\r' | tail -n +2 | sed '$d' || true
	exec 3<&-
}

wait_for() {
	local what=$1 check=$2
	for _ in $(seq 1 100); do
		if eval "$check"; then
			return 0
		fi
		sleep 0.1
	done
	fail "timed out waiting for $what"
}

write_config "$WORK/master.yaml" "$MASTER_PORT" "role: master"
write_config "$WORK/replica.yaml" "$REPLICA_PORT" \
	"role: replica" "master_host: 127.0.0.1" "master_port: $MASTER_PORT" "master_auth: $PASSWORD"

"$BIN" --config "$WORK/master.yaml" >"$WORK/master.log" 2>&1 &
pids+=($!)
wait_for "master to listen" "cmd $MASTER_PORT PING | grep -q PONG"

cmd "$MASTER_PORT" SET before-sync 1 >/dev/null
cmd "$MASTER_PORT" RPUSH list a b c >/dev/null
cmd "$MASTER_PORT" SET with-ttl x EX 1000 >/dev/null

"$BIN" --config "$WORK/replica.yaml" >"$WORK/replica.log" 2>&1 &
pids+=($!)
wait_for "replica link" "cmd $REPLICA_PORT INFO replication | grep -q master_link_status:up"

[ "$(cmd "$REPLICA_PORT" GET before-sync | tail -1)" = "1" ] || fail "initial sync: before-sync missing"
cmd "$REPLICA_PORT" LRANGE list 0 -1 | grep -q '^c$' || fail "initial sync: list missing"
ttl="$(cmd "$REPLICA_PORT" TTL with-ttl | tr -d ':')"
[ "$ttl" -gt 0 ] || fail "initial sync: TTL lost ($ttl)"

cmd "$MASTER_PORT" SET live 42 >/dev/null
cmd "$MASTER_PORT" INCR live >/dev/null
wait_for "live write" "[ \"\$(cmd $REPLICA_PORT GET live | tail -1)\" = 43 ]"

cmd "$REPLICA_PORT" SET local 1 | grep -q '^-READONLY' || fail "replica accepted a write"
[ "$(cmd "$MASTER_PORT" WAIT 1 2000)" = ":1" ] || fail "WAIT did not count the replica"
cmd "$MASTER_PORT" INFO replication | grep -q 'connected_slaves:1' || fail "master INFO: no replica"
cmd "$REPLICA_PORT" INFO server | grep -q "tcp_port:$REPLICA_PORT" || fail "INFO tcp_port is not the real port"

cmd "$REPLICA_PORT" REPLICAOF NO ONE | grep -q '^+OK' || fail "REPLICAOF NO ONE"
cmd "$REPLICA_PORT" SET local 1 | grep -q '^+OK' || fail "promoted replica refused a write"
[ "$(cmd "$REPLICA_PORT" GET live | tail -1)" = "43" ] || fail "promotion lost data"

echo "replication smoke test passed (master :$MASTER_PORT, replica :$REPLICA_PORT)"
