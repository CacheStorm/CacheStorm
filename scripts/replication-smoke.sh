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

enable_replica_aof() {
	cat >>"$WORK/replica.yaml" <<EOF
persistence:
  enabled: true
  aof: true
  aof_sync: always
  snapshot_interval: "0"
  data_dir: $WORK/replica-data
EOF
}
enable_replica_aof

"$BIN" --config "$WORK/master.yaml" >"$WORK/master.log" 2>&1 &
pids+=($!)
wait_for "master to listen" "cmd $MASTER_PORT PING | grep -q PONG"

cmd "$MASTER_PORT" SET before-sync 1 >/dev/null
cmd "$MASTER_PORT" RPUSH list a b c >/dev/null
cmd "$MASTER_PORT" SET with-ttl x EX 1000 >/dev/null
function_code=$'#!lua name=smokelib\nredis.register_function{function_name="smoke_read", callback=function(keys, args) return redis.call("GET", keys[1]) end, flags={"no-writes"}}'
cmd "$MASTER_PORT" FUNCTION LOAD "$function_code" | grep -q '^smokelib$' || fail "master FUNCTION LOAD"

"$BIN" --config "$WORK/replica.yaml" >"$WORK/replica.log" 2>&1 &
replica_pid=$!
pids+=("$replica_pid")
wait_for "replica link" "cmd $REPLICA_PORT INFO replication | grep -q master_link_status:up"

[ "$(cmd "$REPLICA_PORT" GET before-sync | tail -1)" = "1" ] || fail "initial sync: before-sync missing"
cmd "$REPLICA_PORT" LRANGE list 0 -1 | grep -q '^c$' || fail "initial sync: list missing"
ttl="$(cmd "$REPLICA_PORT" TTL with-ttl | tr -d ':')"
[ "$ttl" -gt 0 ] || fail "initial sync: TTL lost ($ttl)"
[ "$(cmd "$REPLICA_PORT" FCALL_RO smoke_read 1 before-sync | tail -1)" = "1" ] || fail "initial sync: function library missing"

live_code=$'#!lua name=livelib\nredis.register_function{function_name="live_read", callback=function(keys, args) return args[1] end, flags={"no-writes"}}'
cmd "$MASTER_PORT" FUNCTION LOAD "$live_code" | grep -q '^livelib$' || fail "live FUNCTION LOAD"
wait_for "live function" "[ \"\$(cmd $REPLICA_PORT FCALL_RO live_read 0 propagated | tail -1)\" = propagated ]"
cmd "$MASTER_PORT" FUNCTION DELETE livelib | grep -q '^+OK' || fail "live FUNCTION DELETE"
wait_for "function deletion" "cmd $REPLICA_PORT FCALL_RO live_read 0 | grep -q 'Function not found'"

cmd "$MASTER_PORT" SET live 42 >/dev/null
cmd "$MASTER_PORT" INCR live >/dev/null
wait_for "live write" "[ \"\$(cmd $REPLICA_PORT GET live | tail -1)\" = 43 ]"

cmd "$REPLICA_PORT" SET local 1 | grep -q '^-READONLY' || fail "replica accepted a write"
[ "$(cmd "$MASTER_PORT" WAIT 1 2000)" = ":1" ] || fail "WAIT did not count the replica"
cmd "$MASTER_PORT" INFO replication | grep -q 'connected_slaves:1' || fail "master INFO: no replica"
cmd "$REPLICA_PORT" INFO server | grep -q "tcp_port:$REPLICA_PORT" || fail "INFO tcp_port is not the real port"

full_before="$(cmd "$MASTER_PORT" INFO stats | awk -F: '/^sync_full:/{print $2}')"
kill -TERM "$replica_pid"
wait "$replica_pid"
pids=("${pids[0]}")
[ -f "$WORK/replica-data/replication.state" ] || fail "clean shutdown did not write a checkpoint"
cp "$WORK/replica-data/replication.state" "$WORK/saved.state"
cmd "$MASTER_PORT" INCR live >/dev/null
"$BIN" --config "$WORK/replica.yaml" >"$WORK/partial-restart.log" 2>&1 &
replica_pid=$!
pids+=("$replica_pid")
wait_for "checkpoint partial restart" "cmd $REPLICA_PORT INFO replication | grep -q master_link_status:up"
wait_for "checkpoint live data" "[ \"\$(cmd $REPLICA_PORT GET live | tail -1)\" = 44 ]"
[ ! -e "$WORK/replica-data/replication.state" ] || fail "startup did not consume checkpoint"
[ "$(cmd "$MASTER_PORT" INFO stats | awk -F: '/^sync_full:/{print $2}')" = "$full_before" ] || fail "matching checkpoint unnecessarily required full sync"
partial="$(cmd "$MASTER_PORT" INFO stats | awk -F: '/^sync_partial_ok:/{print $2}')"
[ "$partial" -gt 0 ] || fail "clean restart did not use partial sync"

kill -KILL "$replica_pid"
wait "$replica_pid" 2>/dev/null || true
pids=("${pids[0]}")
cp "$WORK/saved.state" "$WORK/replica-data/replication.state"
cmd "$MASTER_PORT" INCR live >/dev/null
"$BIN" --config "$WORK/replica.yaml" >"$WORK/stale-restart.log" 2>&1 &
replica_pid=$!
pids+=("$replica_pid")
wait_for "stale checkpoint restart" "cmd $REPLICA_PORT INFO replication | grep -q master_link_status:up"
wait_for "stale checkpoint repaired data" "[ \"\$(cmd $REPLICA_PORT GET live | tail -1)\" = 45 ]"
full_after="$(cmd "$MASTER_PORT" INFO stats | awk -F: '/^sync_full:/{print $2}')"
[ "$full_after" -gt "$full_before" ] || fail "stale checkpoint did not require full sync"
[ "$(cmd "$MASTER_PORT" INFO stats | awk -F: '/^sync_partial_ok:/{print $2}')" = "$partial" ] || fail "stale checkpoint resumed from the wrong offset"

cmd "$REPLICA_PORT" REPLICAOF NO ONE | grep -q '^+OK' || fail "REPLICAOF NO ONE"
cmd "$REPLICA_PORT" SET local 1 | grep -q '^+OK' || fail "promoted replica refused a write"
[ "$(cmd "$REPLICA_PORT" GET live | tail -1)" = "45" ] || fail "promotion lost data"

kill -KILL "$replica_pid"
wait "$replica_pid" 2>/dev/null || true
pids=("${pids[0]}")
write_config "$WORK/replica.yaml" "$REPLICA_PORT" "role: master"
enable_replica_aof
"$BIN" --config "$WORK/replica.yaml" >"$WORK/restarted.log" 2>&1 &
pids+=($!)
wait_for "restarted replica" "cmd $REPLICA_PORT PING | grep -q PONG"
[ "$(cmd "$REPLICA_PORT" FCALL_RO smoke_read 1 before-sync | tail -1)" = "1" ] || fail "AOF restart lost full-sync function library"
cmd "$REPLICA_PORT" FCALL_RO live_read 0 | grep -q 'Function not found' || fail "AOF restart resurrected deleted function"
[ "$(cmd "$REPLICA_PORT" GET live | tail -1)" = "45" ] || fail "AOF restart lost live data"

echo "replication smoke test passed (full/partial sync, stale checkpoint rejection, live functions, promotion and AOF crash recovery; master :$MASTER_PORT, replica :$REPLICA_PORT)"
