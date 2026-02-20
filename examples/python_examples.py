#!/usr/bin/env python3
"""
CacheStorm Python Examples
Uses redis-py library which is fully compatible with CacheStorm
"""

import redis

def main():
    # Connect to CacheStorm
    r = redis.Redis(host='localhost', port=6380, decode_responses=True)
    
    print("=== String Operations ===")
    # Basic set/get
    r.set('mykey', 'Hello World')
    print(f"GET mykey: {r.get('mykey')}")
    
    # Increment
    r.set('counter', 0)
    r.incr('counter')
    r.incr('counter')
    print(f"Counter: {r.get('counter')}")
    
    # MSET/MGET
    r.mset({'key1': 'value1', 'key2': 'value2', 'key3': 'value3'})
    values = r.mget('key1', 'key2', 'key3')
    print(f"MGET: {values}")
    
    # Expiry
    r.set('session', 'data', ex=3600)  # Expires in 1 hour
    ttl = r.ttl('session')
    print(f"Session TTL: {ttl} seconds")
    
    print("\n=== Hash Operations ===")
    # Hash operations
    r.hset('user:1', mapping={'name': 'John Doe', 'email': 'john@example.com', 'age': '30'})
    print(f"User name: {r.hget('user:1', 'name')}")
    print(f"All user data: {r.hgetall('user:1')}")
    
    # Increment hash field
    r.hincrby('user:1', 'age', 1)
    print(f"New age: {r.hget('user:1', 'age')}")
    
    print("\n=== List Operations ===")
    # List operations
    r.delete('mylist')  # Clear first
    r.lpush('mylist', 'world')
    r.lpush('mylist', 'hello')
    r.rpush('mylist', '!')
    print(f"List contents: {r.lrange('mylist', 0, -1)}")
    print(f"List length: {r.llen('mylist')}")
    print(f"LPOP: {r.lpop('mylist')}")
    print(f"RPOP: {r.rpop('mylist')}")
    
    print("\n=== Set Operations ===")
    # Set operations
    r.sadd('myset', 'member1', 'member2', 'member3')
    print(f"Set members: {r.smembers('myset')}")
    print(f"Is member1 in set? {r.sismember('myset', 'member1')}")
    print(f"Set cardinality: {r.scard('myset')}")
    
    print("\n=== Sorted Set Operations ===")
    # Sorted set operations
    r.zadd('leaderboard', {'player1': 100, 'player2': 200, 'player3': 150})
    print(f"Leaderboard: {r.zrange('leaderboard', 0, -1, withscores=True)}")
    print(f"Reverse leaderboard: {r.zrevrange('leaderboard', 0, -1, withscores=True)}")
    print(f"Player1 score: {r.zscore('leaderboard', 'player1')}")
    print(f"Player1 rank: {r.zrank('leaderboard', 'player1')}")
    
    print("\n=== Lua Scripting ===")
    # Lua script for atomic counter with limit
    script = """
    local current = redis.call('GET', KEYS[1])
    if not current then
        current = 0
    else
        current = tonumber(current)
    end
    
    local limit = tonumber(ARGV[1])
    if current >= limit then
        return {err = "LIMIT_EXCEEDED", current = current}
    end
    
    redis.call('INCR', KEYS[1])
    return {ok = "OK", current = current + 1}
    """
    
    # Register and run script
    r.set('limited_counter', 0)
    
    # Run script multiple times
    for i in range(12):
        try:
            result = r.eval(script, 1, 'limited_counter', 10)
            print(f"Increment {i+1}: {result}")
        except redis.ResponseError as e:
            print(f"Increment {i+1}: Error - {e}")
    
    print("\n=== Pipeline (Batch Operations) ===")
    # Pipeline for batch operations
    pipe = r.pipeline()
    pipe.set('batch1', 'value1')
    pipe.set('batch2', 'value2')
    pipe.set('batch3', 'value3')
    pipe.get('batch1')
    pipe.get('batch2')
    pipe.get('batch3')
    results = pipe.execute()
    print(f"Pipeline results: {results}")
    
    print("\n=== Transaction ===")
    # Transaction with WATCH
    r.set('account:1', 100)
    r.set('account:2', 50)
    
    def transfer(from_key, to_key, amount):
        with r.pipeline() as pipe:
            while True:
                try:
                    pipe.watch(from_key)
                    balance = int(pipe.get(from_key) or 0)
                    
                    if balance < amount:
                        pipe.unwatch()
                        return False, "Insufficient funds"
                    
                    pipe.multi()
                    pipe.decrby(from_key, amount)
                    pipe.incrby(to_key, amount)
                    pipe.execute()
                    return True, "Transfer successful"
                    
                except redis.WatchError:
                    continue
    
    success, msg = transfer('account:1', 'account:2', 30)
    print(f"Transfer: {msg}")
    print(f"Account 1: {r.get('account:1')}")
    print(f"Account 2: {r.get('account:2')}")
    
    print("\n=== Pub/Sub ===")
    # Pub/Sub example (using separate connection for subscriber)
    import threading
    import time
    
    def subscriber():
        pubsub = r.pubsub()
        pubsub.subscribe('notifications')
        for message in pubsub.listen():
            if message['type'] == 'message':
                print(f"Received: {message['data']}")
                if message['data'] == 'STOP':
                    break
    
    # Start subscriber in background
    sub_thread = threading.Thread(target=subscriber)
    sub_thread.start()
    
    time.sleep(0.5)  # Wait for subscriber to connect
    
    # Publish messages
    r.publish('notifications', 'Hello subscribers!')
    r.publish('notifications', 'Another message')
    r.publish('notifications', 'STOP')
    
    sub_thread.join()
    
    print("\n=== Cleanup ===")
    r.flushdb()
    print("Database flushed!")
    
    print("\n=== Connection Info ===")
    print(f"Server info: {r.info()}")

if __name__ == '__main__':
    main()
