#!/usr/bin/env bash
# Starts six CacheStorm nodes in cluster mode, joins them with
# `redis-cli --cluster create --cluster-replicas 1` (redis:7.4 in Docker),
# checks the result with `redis-cli --cluster check`, runs go-redis
# ClusterClient traffic, then kills a master and checks that its replica
# takes over: the cluster is OK again, the keys written before survive and
# the clients keep working. It then restarts the killed master and, while a
# go-redis ClusterClient keeps writing and reading, runs redis-cli --cluster
# reshard, add-node (a master and a replica), rebalance
# --cluster-use-empty-masters, del-node of a master emptied by a reshard and
# fix of an open slot, and checks that no key was lost and the client saw no
# error.
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
# start_node starts (or restarts) the node on port $1, keeping its data
# directory, and waits for it to accept connections.
start_node() {
	local p="$1"
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
	"$work/cachestorm" --config "$work/$p/cachestorm.yaml" >> "$work/$p/server.log" 2>&1 &
	pids+=($!)
	pid_of[$p]=$!
}
wait_port() {
	for _ in $(seq 1 50); do
		(exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && return 0
		sleep 0.1
	done
	return 1
}
for p in "${ports[@]}"; do start_node "$p"; done
for p in "${ports[@]}"; do wait_port "$p"; done

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

echo "--- restart the killed master: it rejoins as a replica"
start_node "$victim"
wait_port "$victim"
wait_check_ok "${ports[0]}"
for _ in $(seq 1 60); do
	rcli -p "$victim" role | head -1 | grep -q '^slave$' && break
	sleep 1
done
rcli -p "$victim" role | head -1 | grep -q '^slave$'

# masters prints "id port slot-count" for each master that serves slots, as
# node $1 sees them.
masters() {
	rcli -p "$1" cluster nodes | awk '$3 ~ /master/ && $3 !~ /fail/ {
		n = 0
		for (i = 9; i <= NF; i++) {
			if ($i ~ /^\[/) continue
			split($i, r, "-")
			n += (r[2] == "" ? 1 : r[2] - r[1] + 1)
		}
		split($2, a, "[:@]")
		print $1, a[2], n
	}'
}
node_id() { rcli -p "$1" cluster myid; }

echo "--- background ClusterClient traffic"
stop="$work/load.stop"
(cd tests/clients/goredis && CACHESTORM_CLUSTER_LOAD="127.0.0.1:${ports[0]}" CACHESTORM_LOAD_STOP="$stop" go run .) > "$work/load.out" 2>&1 &
load_pid=$!
pids+=($load_pid)
sleep 3

mapfile -t ms < <(masters "${ports[0]}" | sort -k2n)
[ "${#ms[@]}" = 3 ]
read -r from_id from_port _ <<<"${ms[0]}"
read -r to_id to_port _ <<<"${ms[1]}"
echo "--- redis-cli --cluster reshard: 1000 slots from $from_port to $to_port"
rcli --cluster reshard "127.0.0.1:${ports[0]}" --cluster-from "$from_id" --cluster-to "$to_id" \
	--cluster-slots 1000 --cluster-yes > "$work/reshard.out" 2>&1 || { tail -30 "$work/reshard.out"; exit 1; }
grep -c "^Moving slot" "$work/reshard.out" | sed 's/^/slots moved: /'
wait_check_ok "${ports[0]}"

new_master="$((base + 6))"
new_replica="$((base + 7))"
start_node "$new_master"
start_node "$new_replica"
wait_port "$new_master"
wait_port "$new_replica"
echo "--- redis-cli --cluster add-node 127.0.0.1:$new_master (master)"
rcli --cluster add-node "127.0.0.1:$new_master" "127.0.0.1:${ports[0]}" > "$work/add.out" 2>&1 || { cat "$work/add.out"; exit 1; }
grep -q "\[OK\] New node added correctly" "$work/add.out"
wait_check_ok "${ports[0]}"
new_master_id="$(node_id "$new_master")"
echo "--- redis-cli --cluster add-node 127.0.0.1:$new_replica --cluster-slave --cluster-master-id $new_master_id"
rcli --cluster add-node "127.0.0.1:$new_replica" "127.0.0.1:${ports[0]}" --cluster-slave --cluster-master-id "$new_master_id" > "$work/add2.out" 2>&1 || { cat "$work/add2.out"; exit 1; }
grep -q "\[OK\] New node added correctly" "$work/add2.out"
wait_check_ok "${ports[0]}"

echo "--- redis-cli --cluster rebalance --cluster-use-empty-masters"
rcli --cluster rebalance "127.0.0.1:${ports[0]}" --cluster-use-empty-masters > "$work/rebalance.out" 2>&1 || { tail -30 "$work/rebalance.out"; exit 1; }
grep -E "^(>>>|Moving)" "$work/rebalance.out" | grep -v "^Moving slot" || true
wait_check_ok "${ports[0]}"
masters "${ports[0]}" | awk -v id="$new_master_id" '$1 == id && $3 > 3000 {found = 1} END {exit !found}'
for _ in $(seq 1 30); do
	rcli -p "$new_replica" role | head -1 | grep -q '^slave$' && break
	sleep 1
done
rcli -p "$new_replica" role | head -1 | grep -q '^slave$'

# Empty the master on the lowest port other than the entry node, then delete it.
mapfile -t ms < <(masters "${ports[0]}" | sort -k2n)
victim2_id=""
for m in "${ms[@]}"; do
	read -r id port count <<<"$m"
	if [ "$port" != "${ports[0]}" ] && [ "$id" != "$new_master_id" ]; then
		victim2_id="$id"; victim2_port="$port"; victim2_slots="$count"
		break
	fi
done
[ -n "$victim2_id" ]
echo "--- redis-cli --cluster reshard: all $victim2_slots slots away from $victim2_port"
rcli --cluster reshard "127.0.0.1:${ports[0]}" --cluster-from "$victim2_id" --cluster-to "$new_master_id" \
	--cluster-slots "$victim2_slots" --cluster-yes > "$work/reshard2.out" 2>&1 || { tail -30 "$work/reshard2.out"; exit 1; }
wait_check_ok "${ports[0]}"
[ "$(rcli -p "$victim2_port" dbsize)" = 0 ] || [ "$(rcli -p "$victim2_port" role | head -1)" = slave ]
echo "--- redis-cli --cluster del-node $victim2_port"
rcli --cluster del-node "127.0.0.1:${ports[0]}" "$victim2_id" > "$work/del.out" 2>&1 || { cat "$work/del.out"; exit 1; }
cat "$work/del.out"
kill "${pid_of[$victim2_port]}"
wait "${pid_of[$victim2_port]}" 2>/dev/null || true
wait_check_ok "${ports[0]}"
! grep -q "$victim2_id" <(rcli -p "${ports[0]}" cluster nodes)

echo "--- redis-cli --cluster fix of an open slot"
read -r fix_id fix_port _ <<<"$(masters "${ports[0]}" | sort -k2n | head -1)"
fix_slot="$(rcli -p "$fix_port" cluster nodes | awk '/myself/ {split($9, r, "-"); print r[1]}')"
rcli -p "$new_master" cluster setslot "$fix_slot" importing "$fix_id" >/dev/null
rcli -p "$fix_port" cluster setslot "$fix_slot" migrating "$new_master_id" >/dev/null
check_ok "${ports[0]}" && { echo "check did not report the open slot" >&2; exit 1; }
rcli --cluster fix "127.0.0.1:${ports[0]}" > "$work/fix.out" 2>&1 || { tail -30 "$work/fix.out"; exit 1; }
grep -E "open slot|Fixing" "$work/fix.out" || true
wait_check_ok "${ports[0]}"

echo "--- stop the background client"
touch "$stop"
if ! wait "$load_pid"; then
	cat "$work/load.out"
	echo "the background ClusterClient failed" >&2
	exit 1
fi
cat "$work/load.out"
grep -q " 0 missing or wrong, 0 errors$" "$work/load.out"

echo "--- the keys written before the resharding survived"
for i in $(seq 1 "$nkeys"); do echo "GET smoke:$i"; done | rcli -c -p "${ports[0]}" | grep -v '^-> Redirected' > "$work/get.out"
for i in $(seq 1 "$nkeys"); do echo "v$i"; done | diff - "$work/get.out"

echo "cluster smoke test passed"
