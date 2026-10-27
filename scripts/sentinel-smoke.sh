#!/usr/bin/env bash
# Runs three CacheStorm processes (a master and two replicas) under three real
# Redis Sentinels and checks automatic failover end to end: the sentinels
# discover the replicas and each other, elect a leader when the master is
# killed, promote a replica, repoint the other one, and demote the old master
# to a replica of the new one when it comes back. No acknowledged write may be
# lost.
#
#   scripts/sentinel-smoke.sh
#   BIN=./cachestorm SKIP_BUILD=1 scripts/sentinel-smoke.sh
#
# Needs docker for the redis-sentinel image (REDIS_IMAGE, default
# redis:7.4-alpine).
set -euo pipefail

cd "$(dirname "$0")/.."

WORK="$(mktemp -d)"
chmod 777 "$WORK"
BIN="${BIN:-$WORK/cachestorm}"
IMAGE="${REDIS_IMAGE:-redis:7.4-alpine}"
BASE="${BASE_PORT:-$((21000 + RANDOM % 18000))}"
PORTS=("$BASE" "$((BASE + 1))" "$((BASE + 2))")
SENTINEL_PORTS=("$((BASE + 10))" "$((BASE + 11))" "$((BASE + 12))")
PASSWORD="smoke-$RANDOM"
NAME="smoke$RANDOM"
declare -A pid_of
containers=()

cleanup() {
	status=$?
	for pid in "${pid_of[@]}"; do
		kill "$pid" 2>/dev/null || true
		wait "$pid" 2>/dev/null || true
	done
	if [ "$status" -ne 0 ]; then
		for log in "$WORK"/*.log; do
			echo "--- $log"
			tail -30 "$log" || true
		done
		for c in "${containers[@]}"; do
			echo "--- sentinel $c"
			docker logs "$c" 2>&1 | tail -30 || true
		done
	fi
	for c in "${containers[@]}"; do
		docker rm -f "$c" >/dev/null 2>&1 || true
	done
	rm -rf "$WORK"
	exit "$status"
}
trap cleanup EXIT
trap 'exit 143' INT TERM

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

if [ -z "${SKIP_BUILD:-}" ]; then
	go build -o "$BIN" ./cmd/cachestorm
fi
docker pull -q "$IMAGE" >/dev/null

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
	timeout 5 cat <&3 | tr -d '\r' | tail -n +2 | sed '$d' || true
	exec 3<&-
}

# sentinel <port> <words...> sends one command to a sentinel and prints the
# reply's values, one per line, without RESP array and length headers.
sentinel() {
	local port=$1
	shift
	local payload="*$#\r\n"
	local w
	for w in "$@"; do
		payload+="\$${#w}\r\n${w}\r\n"
	done
	payload+="*1\r\n\$4\r\nQUIT\r\n"
	if ! { exec 4<>"/dev/tcp/127.0.0.1/$port"; } 2>/dev/null; then
		return 0
	fi
	printf '%b' "$payload" >&4
	timeout 5 cat <&4 | tr -d '\r' | sed '$d' | grep -v -E '^[*$]' || true
	exec 4<&-
}

wait_for() {
	local what=$1 check=$2 tries=${3:-100}
	for _ in $(seq 1 "$tries"); do
		if eval "$check"; then
			return 0
		fi
		sleep 0.2
	done
	fail "timed out waiting for $what"
}

role_of() { cmd "$1" ROLE | grep -m1 -E '^(master|slave)$'; }

master_addr() { sentinel "${SENTINEL_PORTS[0]}" SENTINEL get-master-addr-by-name "$NAME" | tr '\n' ' '; }

write_config() {
	local port=$1 role=$2
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
		echo "persistence:"
		echo "  enabled: true"
		echo "  aof_sync: always"
		echo "  snapshot_interval: \"0\""
		echo "  data_dir: $WORK/data-$port"
		echo "replication:"
		echo "  role: $role"
		echo "  master_auth: $PASSWORD"
		if [ "$role" = replica ]; then
			echo "  master_host: 127.0.0.1"
			echo "  master_port: ${PORTS[0]}"
		fi
		echo "logging:"
		echo "  level: info"
	} >"$WORK/node-$port.yaml"
}

start_node() {
	local port=$1
	"$BIN" --config "$WORK/node-$port.yaml" >>"$WORK/node-$port.log" 2>&1 &
	pid_of[$port]=$!
}

write_config "${PORTS[0]}" master
write_config "${PORTS[1]}" replica
write_config "${PORTS[2]}" replica
for port in "${PORTS[@]}"; do
	start_node "$port"
done
for port in "${PORTS[@]}"; do
	wait_for "node $port" "cmd $port PING | grep -q PONG"
done
for port in "${PORTS[@]:1}"; do
	wait_for "replica $port link" "cmd $port INFO replication | grep -q master_link_status:up"
done

for i in 0 1 2; do
	conf="$WORK/sentinel-$i.conf"
	cat >"$conf" <<EOF
port ${SENTINEL_PORTS[$i]}
bind 127.0.0.1
sentinel monitor $NAME 127.0.0.1 ${PORTS[0]} 2
sentinel auth-pass $NAME $PASSWORD
sentinel down-after-milliseconds $NAME 2000
sentinel failover-timeout $NAME 10000
sentinel parallel-syncs $NAME 1
EOF
	chmod 666 "$conf"
	c="$NAME-s$i"
	docker run -d --name "$c" --network host --user "$(id -u):$(id -g)" -v "$WORK:/conf" "$IMAGE" \
		redis-sentinel "/conf/sentinel-$i.conf" >/dev/null
	containers+=("$c")
done

wait_for "sentinels to discover both replicas" \
	"sentinel ${SENTINEL_PORTS[0]} SENTINEL master $NAME | grep -A1 '^num-slaves$' | tail -1 | grep -q '^2$'" 150
wait_for "sentinels to discover each other" \
	"sentinel ${SENTINEL_PORTS[0]} SENTINEL master $NAME | grep -A1 '^num-other-sentinels$' | tail -1 | grep -q '^2$'" 150

for i in $(seq 1 50); do
	cmd "${PORTS[0]}" SET "key:$i" "$i" >/dev/null
done
[ "$(cmd "${PORTS[0]}" WAIT 2 5000 | tr -d ':')" = "2" ] || fail "WAIT did not reach both replicas"

echo "killing master ${PORTS[0]}"
kill -9 "${pid_of[${PORTS[0]}]}"
wait "${pid_of[${PORTS[0]}]}" 2>/dev/null || true
unset "pid_of[${PORTS[0]}]"

wait_for "a new master" "! master_addr | grep -q ' ${PORTS[0]} '" 150
new_port="$(master_addr | awk '{print $2}')"
echo "sentinels promoted $new_port"
[ "$new_port" = "${PORTS[1]}" ] || [ "$new_port" = "${PORTS[2]}" ] || fail "unexpected new master: $(master_addr)"
other_port="${PORTS[1]}"
[ "$new_port" = "${PORTS[1]}" ] && other_port="${PORTS[2]}"

wait_for "promoted node reports master" "[ \"\$(role_of $new_port)\" = master ]" 150
wait_for "other replica follows the new master" \
	"cmd $other_port INFO replication | grep -q 'master_port:$new_port' && cmd $other_port INFO replication | grep -q master_link_status:up" 150

[ "$(cmd "$new_port" GET key:50 | tail -1)" = "50" ] || fail "acknowledged write lost on the new master"
cmd "$new_port" SET after-failover yes | grep -q OK || fail "write on the new master"
wait_for "write reaches the other replica" "[ \"\$(cmd $other_port GET after-failover | tail -1)\" = yes ]"

echo "restarting old master ${PORTS[0]}"
start_node "${PORTS[0]}"
wait_for "old master demoted to a replica of $new_port" \
	"cmd ${PORTS[0]} INFO replication | grep -q 'master_port:$new_port' && cmd ${PORTS[0]} INFO replication | grep -q master_link_status:up" 200
[ "$(cmd "${PORTS[0]}" GET after-failover | tail -1)" = "yes" ] || fail "old master did not resync from the new master"
cmd "${PORTS[0]}" SET refused 1 | grep -q READONLY || fail "demoted old master accepts writes"

echo "sentinel smoke test passed: failover to $new_port, old master rejoined as a replica"
