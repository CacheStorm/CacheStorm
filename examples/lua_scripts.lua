-- CacheStorm Lua Scripting Examples
-- Run with: redis-cli -p 6380 EVAL "<script>" <numkeys> [keys...] [args...]

-- ============================================
-- Example 1: Simple GET and SET
-- ============================================
-- EVAL "return redis.call('GET', KEYS[1])" 1 mykey
-- EVAL "return redis.call('SET', KEYS[1], ARGV[1])" 1 mykey myvalue

-- Simple atomic counter
local current = redis.call('GET', KEYS[1])
if not current then
    current = 0
else
    current = tonumber(current)
end
redis.call('SET', KEYS[1], current + 1)
return current + 1

-- ============================================
-- Example 2: Cache-Aside Pattern
-- ============================================
-- EVAL "<script>" 1 cache:key data "3600"
local function get_or_set()
    local cached = redis.call('GET', KEYS[1])
    if cached then
        return cached
    end
    
    -- Simulate expensive computation
    local value = ARGV[1]
    local ttl = tonumber(ARGV[2]) or 3600
    
    redis.call('SET', KEYS[1], value, 'EX', ttl)
    return value
end

return get_or_set()

-- ============================================
-- Example 3: Rate Limiter
-- ============================================
-- EVAL "<script>" 1 rate_limit:user123 10 60
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = redis.call('GET', key)
if not current then
    current = 0
end
current = tonumber(current)

if current >= limit then
    return 0  -- Rate limit exceeded
end

redis.call('INCR', key)
redis.call('EXPIRE', key, window)
return 1  -- Allowed

-- ============================================
-- Example 4: Distributed Lock
-- ============================================
-- EVAL "<script>" 1 lock:resource1 client123 30
local key = KEYS[1]
local token = ARGV[1]
local ttl = tonumber(ARGV[2])

local value = redis.call('GET', key)
if value then
    if value == token then
        -- Extend lock
        redis.call('EXPIRE', key, ttl)
        return 1
    end
    return 0  -- Lock held by another
end

-- Acquire lock
redis.call('SET', key, token, 'EX', ttl)
return 1

-- ============================================
-- Example 5: Atomic Transfer (Simple)
-- ============================================
-- EVAL "<script>" 2 from_account to_account 100
local from = KEYS[1]
local to = KEYS[2]
local amount = tonumber(ARGV[1])

local from_balance = tonumber(redis.call('GET', from) or 0)
local to_balance = tonumber(redis.call('GET', to) or 0)

if from_balance < amount then
    return {err = "INSUFFICIENT_FUNDS"}
end

redis.call('SET', from, from_balance - amount)
redis.call('SET', to, to_balance + amount)

return {ok = "TRANSFERRED", from = from_balance - amount, to = to_balance + amount}

-- ============================================
-- Example 6: List Processing
-- ============================================
-- EVAL "<script>" 1 mylist 5
local key = KEYS[1]
local count = tonumber(ARGV[1])

local results = {}
for i = 1, count do
    local val = redis.call('LPOP', key)
    if not val then
        break
    end
    table.insert(results, val)
end

return results

-- ============================================
-- Example 7: Hash Aggregation
-- ============================================
-- EVAL "<script>" 1 users:stats
local key = KEYS[1]
local data = redis.call('HGETALL', key)

local result = {
    total = 0,
    count = 0,
    average = 0
}

-- Process hash fields
for i = 1, #data, 2 do
    local field = data[i]
    local value = tonumber(data[i + 1]) or 0
    result.total = result.total + value
    result.count = result.count + 1
end

if result.count > 0 then
    result.average = result.total / result.count
end

return result

-- ============================================
-- Example 8: Sorted Set Leaderboard
-- ============================================
-- EVAL "<script>" 1 leaderboard player4 250
local key = KEYS[1]
local player = ARGV[1]
local score = tonumber(ARGV[2])

-- Add player
redis.call('ZADD', key, score, player)

-- Get rank (0-indexed)
local rank = redis.call('ZREVRANK', key, player)

-- Get top 10
local top10 = redis.call('ZREVRANGE', key, 0, 9, 'WITHSCORES')

return {
    player = player,
    score = score,
    rank = rank,
    top10 = top10
}

-- ============================================
-- Example 9: Set Operations
-- ============================================
-- EVAL "<script>" 3 set1 set2 set3
local set1 = KEYS[1]
local set2 = KEYS[2]
local result = KEYS[3]

-- Get members from set1
local members1 = redis.call('SMEMBERS', set1)

-- Add to result set
for _, member in ipairs(members1) do
    if redis.call('SISMEMBER', set2, member) == 1 then
        redis.call('SADD', result, member)
    end
end

return redis.call('SCARD', result)

-- ============================================
-- Example 10: Conditional Update
-- ============================================
-- EVAL "<script>" 1 document:1 "new_content" "expected_version"
local key = KEYS[1]
local new_content = ARGV[1]
local expected_version = ARGV[2]

-- Get current version
local current = redis.call('HGET', key, 'version')

if not current then
    -- Document doesn't exist, create it
    redis.call('HSET', key, 'content', new_content, 'version', 1)
    return {ok = "CREATED", version = 1}
end

if current ~= expected_version then
    return {err = "VERSION_MISMATCH", current = current}
end

-- Update document
redis.call('HSET', key, 'content', new_content)
redis.call('HINCRBY', key, 'version', 1)

return {ok = "UPDATED", version = tonumber(current) + 1}
