# HTTP API

CacheStorm provides a RESTful HTTP API for integration and management.

## Base URL

```
http://localhost:8080/api
```

## Authentication

When HTTP password is configured, include authentication in requests:

### Bearer Token
```bash
curl -H "Authorization: Bearer your-password" http://localhost:8080/api/keys
```

### Query Parameter
```bash
curl "http://localhost:8080/api/keys?token=your-password"
```

### Cookie
```bash
curl -b "auth_token=your-password" http://localhost:8080/api/keys
```

## Endpoints

### Health Check

```http
GET /api/health
```

Response:
```json
{
  "status": "ok",
  "uptime": "2h30m15s"
}
```

### Server Info

```http
GET /api/info
```

Response:
```json
{
  "server": {
    "version": "1.0.0",
    "uptime": "2h30m15s",
    "keys": 1234,
    "memory": 52428800,
    "started_at": "2024-01-15T10:00:00Z"
  },
  "store": {
    "shards": 256,
    "keys": 1234,
    "mem_used": 52428800
  }
}
```

### Stats

```http
GET /api/stats
```

Response:
```json
{
  "keys": 1234,
  "memory": 52428800,
  "tags": 45,
  "namespaces": 3,
  "uptime": "2h30m15s",
  "shards": 256,
  "started_at": "2024-01-15T10:00:00Z"
}
```

### Prometheus Metrics

```http
GET /api/metrics
```

Response:
```
# HELP cachestorm_keys_total Total number of keys
# TYPE cachestorm_keys_total gauge
cachestorm_keys_total 1234
# HELP cachestorm_memory_bytes Memory usage in bytes
# TYPE cachestorm_memory_bytes gauge
cachestorm_memory_bytes 52428800
# HELP cachestorm_uptime_seconds Server uptime in seconds
# TYPE cachestorm_uptime_seconds gauge
cachestorm_uptime_seconds 9015
```

---

## Keys

### List Keys

```http
GET /api/keys?pattern=*
```

Query Parameters:
- `pattern` - Pattern to match keys (default: `*`)

Response:
```json
{
  "count": 2,
  "keys": [
    {
      "key": "user:1",
      "type": "string",
      "ttl": "-1",
      "size": 128,
      "tags": ["user", "session"]
    },
    {
      "key": "user:2",
      "type": "hash",
      "ttl": "1h30m",
      "size": 256,
      "tags": ["user"]
    }
  ]
}
```

### Get Key

```http
GET /api/key/{key}
```

Response:
```json
{
  "key": "user:1",
  "type": "string",
  "value": "{\"name\":\"John\",\"email\":\"john@example.com\"}",
  "ttl": "-1",
  "tags": ["user", "session"],
  "size": 128
}
```

### Create Key

```http
POST /api/keys
Content-Type: application/json

{
  "key": "user:3",
  "value": "{\"name\":\"Jane\"}",
  "type": "string",
  "ttl": "1h",
  "tags": ["user"]
}
```

Response:
```json
{
  "result": "OK",
  "key": "user:3"
}
```

### Delete Key

```http
DELETE /api/key/{key}
```

Response:
```json
{
  "deleted": true,
  "key": "user:3"
}
```

---

## Tags

### List Tags

```http
GET /api/tags
```

Response:
```json
{
  "count": 2,
  "tags": [
    {
      "name": "user",
      "count": 150
    },
    {
      "name": "session",
      "count": 89
    }
  ]
}
```

### Get Tag Keys

```http
GET /api/tag/{tag}
```

Response:
```json
{
  "tag": "user",
  "count": 150,
  "keys": [
    "user:1",
    "user:2",
    "user:3"
  ]
}
```

### Invalidate Tag

```http
POST /api/invalidate/{tag}
```

Response:
```json
{
  "tag": "user",
  "keys_deleted": 150,
  "keys": ["user:1", "user:2", "user:3"]
}
```

---

## Namespaces

### List Namespaces

```http
GET /api/namespaces
```

Response:
```json
{
  "count": 2,
  "namespaces": [
    {
      "name": "default",
      "keys": 1234,
      "memory": 52428800,
      "created_at": "2024-01-15T10:00:00Z"
    },
    {
      "name": "cache",
      "keys": 567,
      "memory": 20971520,
      "created_at": "2024-01-15T11:00:00Z"
    }
  ]
}
```

### Create Namespace

```http
POST /api/namespaces
Content-Type: application/json

{
  "name": "sessions"
}
```

Response:
```json
{
  "result": "OK",
  "namespace": "sessions"
}
```

### Get Namespace

```http
GET /api/namespace/{name}
```

Response:
```json
{
  "name": "cache",
  "keys": 567,
  "memory": 20971520,
  "created_at": "2024-01-15T11:00:00Z"
}
```

### Delete Namespace

```http
DELETE /api/namespace/{name}
```

Response:
```json
{
  "result": "OK",
  "namespace": "cache"
}
```

---

## Cluster

### Get Cluster Info

```http
GET /api/cluster
```

Response:
```json
{
  "state": "ok",
  "slots_assigned": 16384,
  "slots_ok": 16384,
  "known_nodes": 3,
  "size": 3,
  "current_epoch": 1,
  "nodes": [
    {
      "id": "node-1",
      "addr": "10.0.0.1:6380",
      "role": "master",
      "slots": "0-5461",
      "connected": true
    },
    {
      "id": "node-2",
      "addr": "10.0.0.2:6380",
      "role": "master",
      "slots": "5462-10922",
      "connected": true
    }
  ]
}
```

### Join Cluster

```http
POST /api/cluster/join
Content-Type: application/json

{
  "host": "10.0.0.1",
  "port": 7946
}
```

Response:
```json
{
  "result": "OK",
  "message": "Joining cluster at 10.0.0.1:7946"
}
```

---

## Console

### Execute Command

```http
POST /api/execute
Content-Type: application/json

{
  "command": "SET",
  "args": ["mykey", "myvalue"]
}
```

Response:
```json
{
  "result": "OK"
}
```

Examples:
```bash
# GET command
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command": "GET", "args": ["mykey"]}'

# KEYS command
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command": "KEYS", "args": ["*"]}'

# DBSIZE command
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command": "DBSIZE", "args": []}'
```

---

## Slow Log

### Get Slow Log

```http
GET /api/slowlog?count=10
```

Response:
```json
{
  "count": 2,
  "entries": [
    {
      "id": 1,
      "start_time": "2024-01-15T12:00:00Z",
      "duration": "25ms",
      "command": "KEYS *"
    },
    {
      "id": 2,
      "start_time": "2024-01-15T12:01:00Z",
      "duration": "15ms",
      "command": "SORT mylist"
    }
  ]
}
```

---

## Authentication

### Login

```http
POST /api/login
Content-Type: application/json

{
  "password": "your-password"
}
```

Response (success):
```json
{
  "success": true,
  "token": "your-password"
}
```

Response (failure):
```json
{
  "error": true,
  "status": 401,
  "reason": "invalid password"
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": true,
  "status": 400,
  "reason": "invalid JSON"
}
```

Common error codes:
- `400` - Bad Request
- `401` - Unauthorized
- `404` - Not Found
- `405` - Method Not Allowed
- `500` - Internal Server Error

## CORS

CORS is enabled for all API endpoints:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

## Rate Limiting

Currently not implemented. Coming in future versions.
