#!/bin/bash
# CacheStorm Basic Usage Examples
# Run these commands after starting CacheStorm server

# Start CacheStorm
# ./cachestorm

# Or use redis-cli to connect
# redis-cli -p 6380

echo "=== String Commands ==="
redis-cli -p 6380 SET mykey "Hello World"
redis-cli -p 6380 GET mykey
redis-cli -p 6380 INCR counter
redis-cli -p 6380 INCR counter
redis-cli -p 6380 GET counter
redis-cli -p 6380 MSET key1 "value1" key2 "value2" key3 "value3"
redis-cli -p 6380 MGET key1 key2 key3

echo ""
echo "=== Hash Commands ==="
redis-cli -p 6380 HSET user:1 name "John Doe" email "john@example.com" age "30"
redis-cli -p 6380 HGET user:1 name
redis-cli -p 6380 HGETALL user:1
redis-cli -p 6380 HINCRBY user:1 age 1
redis-cli -p 6380 HGET user:1 age

echo ""
echo "=== List Commands ==="
redis-cli -p 6380 LPUSH mylist "world"
redis-cli -p 6380 LPUSH mylist "hello"
redis-cli -p 6380 RPUSH mylist "!"
redis-cli -p 6380 LRANGE mylist 0 -1
redis-cli -p 6380 LLEN mylist
redis-cli -p 6380 LPOP mylist
redis-cli -p 6380 RPOP mylist

echo ""
echo "=== Set Commands ==="
redis-cli -p 6380 SADD myset "member1" "member2" "member3"
redis-cli -p 6380 SMEMBERS myset
redis-cli -p 6380 SISMEMBER myset "member1"
redis-cli -p 6380 SCARD myset

echo ""
echo "=== Sorted Set Commands ==="
redis-cli -p 6380 ZADD leaderboard 100 "player1"
redis-cli -p 6380 ZADD leaderboard 200 "player2" 150 "player3"
redis-cli -p 6380 ZRANGE leaderboard 0 -1 WITHSCORES
redis-cli -p 6380 ZREVRANGE leaderboard 0 -1 WITHSCORES
redis-cli -p 6380 ZSCORE leaderboard "player1"
redis-cli -p 6380 ZRANK leaderboard "player1"

echo ""
echo "=== TTL Commands ==="
redis-cli -p 6380 SET session:abc "user_data" EX 3600
redis-cli -p 6380 TTL session:abc
redis-cli -p 6380 EXPIRE session:abc 7200
redis-cli -p 6380 TTL session:abc

echo ""
echo "=== Server Commands ==="
redis-cli -p 6380 PING
redis-cli -p 6380 DBSIZE
redis-cli -p 6380 INFO
redis-cli -p 6380 TIME

echo ""
echo "=== Cleanup ==="
redis-cli -p 6380 FLUSHDB
redis-cli -p 6380 DBSIZE

echo ""
echo "All examples completed!"
