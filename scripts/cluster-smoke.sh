#!/usr/bin/env bash
# Starts six CacheStorm nodes in cluster mode, joins them with
# `redis-cli --cluster create --cluster-replicas 1` (redis:7.4 in Docker),
# checks the result with `redis-cli --cluster check`, runs go-redis
# ClusterClient traffic, then kills a master and checks that its replica
# takes over: the cluster is OK again, the keys written before survive and
# the clients keep working.
set -euo pipefail
cd "$(dirname "$0")/.."

work="$(mktemp -d)"
pids=()
sub_container="cachestorm-cluster-smoke-sub-$$"
cleanup() {
	docker rm -f "$sub_container" >/dev/null 2>&1 || true
	for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done
	for p in "${pids[@]}"; do wait "$p" 2>/dev/null || true; done
	rm -rf "$work"
}
trap cleanup EXIT

base="${CLUSTER_SMOKE_PORT:-17400}"
ports=()
for i in 0 1 2 3 4 5; do ports+=("$((base + i))"); done
export REDISCLI_AUTH="cluster-smoke-$RANDOM"
export CACHESTORM_PASSWORD="$REDISCLI_AUTH"

go build -o "$work/cachestorm" ./cmd/cachestorm
declare -A pid_of
for p in "${ports[@]}"; do
	mkdir -p "$work/$p"
	cat > "$work/$p/cachestorm.yaml" <<YAML
server: {bind: "127.0.0.1", port: $p, requirepass: "$REDISCLI_AUTH"}
http: {enabled: false}
plugins: {metrics: {enabled: false}}
logging: {level: "warn"}
persistence: {data_dir: "$work/$p/data"}
replication: {master_auth: "$REDISCLI_AUTH"}
cluster: {enabled: true, node_timeout: 2000}
YAML
	"$work/cachestorm" --config "$work/$p/cachestorm.yaml" > "$work/$p/server.log" 2>&1 &
	pids+=($!)
	pid_of[$p]=$!
done
for p in "${ports[@]}"; do
	for _ in $(seq 1 50); do
		(exec 3<>"/dev/tcp/127.0.0.1/$p") 2>/dev/null && break
		sleep 0.1
	done
done

rcli() { docker run --rm -i --network host -e REDISCLI_AUTH redis:7.4 redis-cli "$@"; }

# check_ok runs redis-cli --cluster check against port $1 and reports
# whether it found the cluster healthy.
check_ok() {
	rcli --cluster check "127.0.0.1:$1" > "$work/check.out" 2>&1 || true
	grep -q "\[OK\] All nodes agree about slots configuration" "$work/check.out" &&
		grep -q "\[OK\] All 16384 slots covered" "$work/check.out" &&
		! grep -q "\[ERR\]\|\[WARNING\]" "$work/check.out"
}

wait_check_ok() {
	for _ in $(seq 1 60); do
		check_ok "$1" && { cat "$work/check.out"; return 0; }
		sleep 1
	done
	cat "$work/check.out"
	echo "redis-cli --cluster check reported problems" >&2
	return 1
}

addrs=()
for p in "${ports[@]}"; do addrs+=("127.0.0.1:$p"); done
echo "--- redis-cli --cluster create --cluster-replicas 1"
rcli --cluster create "${addrs[@]}" --cluster-replicas 1 --cluster-yes | tee "$work/create.out"
grep -q "\[OK\] All 16384 slots covered" "$work/create.out"

echo "--- redis-cli --cluster check"
wait_check_ok "${ports[0]}"
grep -q "keys in 3 masters" "$work/check.out"
[ "$(grep -c '^S: ' "$work/check.out")" = 3 ]

for p in "${ports[@]}"; do
	info="$(rcli -p "$p" cluster info)"
	grep -q "cluster_state:ok" <<<"$info"
	grep -q "cluster_known_nodes:6" <<<"$info"
	grep -q "cluster_size:3" <<<"$info"
done

echo "--- go-redis ClusterClient"
(cd tests/clients/goredis && CACHESTORM_CLUSTER_ADDR="127.0.0.1:${ports[1]}" go run .)

echo "--- PUBLISH across nodes"
sub="$work/sub.out"
docker run --rm --name "$sub_container" --network host -e REDISCLI_AUTH redis:7.4 \
	redis-cli -p "${ports[2]}" subscribe smoke-channel > "$sub" 2>&1 &
pids+=($!)
for _ in $(seq 1 50); do
	[ "$(rcli -p "${ports[2]}" pubsub numsub smoke-channel | tail -1)" = "1" ] && break
	sleep 0.2
done
rcli -p "${ports[0]}" publish smoke-channel hello >/dev/null
for _ in $(seq 1 50); do
	grep -q "^hello$" "$sub" && break
	sleep 0.1
done
grep -q "^hello$" "$sub"
docker rm -f "$sub_container" >/dev/null 2>&1 || true

echo "--- write keys, then kill a master"
nkeys=300
for i in $(seq 1 "$nkeys"); do echo "SET smoke:$i v$i"; done | rcli -c -p "${ports[0]}" > "$work/set.out"
[ "$(grep -c '^OK$' "$work/set.out")" = "$nkeys" ]
# Wait until the replicas have the keys.
sleep 2

victim=""
victim_id=""
while read -r id addr flags _; do
	port="${addr#*:}"
	port="${port%%@*}"
	if [[ "$flags" == *master* && "$port" != "${ports[0]}" ]]; then
		victim="$port"
		victim_id="$id"
		break
	fi
done < <(rcli -p "${ports[0]}" cluster nodes)
[ -n "$victim" ]
replica_port="$(rcli -p "${ports[0]}" cluster replicas "$victim_id" | awk '{print $2}' | sed 's/^[^:]*://; s/@.*//')"
echo "killing master 127.0.0.1:$victim ($victim_id), replica 127.0.0.1:$replica_port"
kill "${pid_of[$victim]}"
wait "${pid_of[$victim]}" 2>/dev/null || true

for _ in $(seq 1 60); do
	if rcli -p "$replica_port" role | head -1 | grep -q '^master$'; then
		break
	fi
	sleep 1
done
rcli -p "$replica_port" role | head -1 | grep -q '^master$'
echo "the replica on port $replica_port was promoted"

echo "--- redis-cli --cluster check after the failover"
wait_check_ok "${ports[0]}"
for p in "${ports[@]}"; do
	[ "$p" = "$victim" ] && continue
	rcli -p "$p" cluster info | grep -q "cluster_state:ok"
done

echo "--- the keys survived"
for i in $(seq 1 "$nkeys"); do echo "GET smoke:$i"; done | rcli -c -p "${ports[0]}" | grep -v '^-> Redirected' > "$work/get.out"
for i in $(seq 1 "$nkeys"); do echo "v$i"; done | diff - "$work/get.out"

echo "--- go-redis ClusterClient after the failover"
(cd tests/clients/goredis && CACHESTORM_CLUSTER_ADDR="127.0.0.1:${ports[0]}" go run .)

echo "cluster smoke test passed"
