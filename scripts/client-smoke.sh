#!/usr/bin/env bash
# Builds CacheStorm, starts it with a password and runs real-world clients
# against it: go-redis v9 (native), redis-py 5 and ioredis 5 (in Docker).
set -euo pipefail
cd "$(dirname "$0")/.."

work="$(mktemp -d)"
trap 'kill "$pid" 2>/dev/null || true; rm -rf "$work"' EXIT

port="${PORT:-17399}"
export CACHESTORM_ADDR="127.0.0.1:$port"
export CACHESTORM_PASSWORD="client-smoke-$RANDOM"

go build -o "$work/cachestorm" ./cmd/cachestorm
cat > "$work/cachestorm.yaml" <<YAML
server: {bind: "127.0.0.1", port: $port, requirepass: "$CACHESTORM_PASSWORD"}
http: {enabled: false}
plugins: {metrics: {enabled: false}}
logging: {level: "warn"}
YAML
"$work/cachestorm" --config "$work/cachestorm.yaml" > "$work/server.log" 2>&1 &
pid=$!
for _ in $(seq 1 50); do
	(exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null && break
	sleep 0.1
done

echo "--- go-redis"
(cd tests/clients/goredis && go run .)

echo "--- redis-py"
docker run --rm --network host -e CACHESTORM_ADDR -e CACHESTORM_PASSWORD \
	-v "$PWD/tests/clients/python:/app:ro" python:3.12-slim \
	sh -c "pip -q install --disable-pip-version-check redis==5.2.1 && python /app/smoke.py"

echo "--- ioredis"
cp tests/clients/node/smoke.js tests/clients/package.json "$work/"
docker run --rm --network host -e CACHESTORM_ADDR -e CACHESTORM_PASSWORD \
	--user "$(id -u):$(id -g)" -e HOME=/tmp \
	-v "$work:/app" -w /app node:22-slim \
	sh -c "npm install --silent --no-audit --no-fund && node smoke.js"

echo "client smoke test passed"
