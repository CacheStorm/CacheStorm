#!/usr/bin/env bash
# Starts three CacheStorm nodes in cluster mode, joins them with
# `redis-cli --cluster create` (redis:7.4 in Docker), checks the result with
# `redis-cli --cluster check`, and runs go-redis ClusterClient traffic.
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
ports=("$base" "$((base + 1))" "$((base + 2))")
export REDISCLI_AUTH="cluster-smoke-$RANDOM"
export CACHESTORM_PASSWORD="$REDISCLI_AUTH"

go build -o "$work/cachestorm" ./cmd/cachestorm
for p in "${ports[@]}"; do
	mkdir -p "$work/$p"
	cat > "$work/$p/cachestorm.yaml" <<YAML
server: {bind: "127.0.0.1", port: $p, requirepass: "$REDISCLI_AUTH"}
http: {enabled: false}
plugins: {metrics: {enabled: false}}
logging: {level: "warn"}
persistence: {data_dir: "$work/$p/data"}
cluster: {enabled: true, node_timeout: 2000}
YAML
	"$work/cachestorm" --config "$work/$p/cachestorm.yaml" > "$work/$p/server.log" 2>&1 &
	pids+=($!)
done
for p in "${ports[@]}"; do
	for _ in $(seq 1 50); do
		(exec 3<>"/dev/tcp/127.0.0.1/$p") 2>/dev/null && break
		sleep 0.1
	done
done

rcli() { docker run --rm --network host -e REDISCLI_AUTH redis:7.4 redis-cli "$@"; }

echo "--- redis-cli --cluster create"
rcli --cluster create "127.0.0.1:${ports[0]}" "127.0.0.1:${ports[1]}" "127.0.0.1:${ports[2]}" --cluster-yes | tee "$work/create.out"
grep -q "\[OK\] All 16384 slots covered" "$work/create.out"

echo "--- redis-cli --cluster check"
rcli --cluster check "127.0.0.1:${ports[0]}" | tee "$work/check.out"
grep -q "\[OK\] All nodes agree about slots configuration" "$work/check.out"
grep -q "\[OK\] All 16384 slots covered" "$work/check.out"
if grep -q "\[ERR\]\|\[WARNING\]" "$work/check.out"; then
	echo "redis-cli --cluster check reported problems" >&2
	exit 1
fi

for p in "${ports[@]}"; do
	info="$(rcli -p "$p" cluster info)"
	grep -q "cluster_state:ok" <<<"$info"
	grep -q "cluster_known_nodes:3" <<<"$info"
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

echo "cluster smoke test passed"
