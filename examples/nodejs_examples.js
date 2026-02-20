/**
 * CacheStorm Node.js Examples
 * Uses ioredis library which is fully compatible with CacheStorm
 * 
 * Install: npm install ioredis
 * Run: node nodejs_examples.js
 */

const Redis = require('ioredis');

async function main() {
    const redis = new Redis({
        host: 'localhost',
        port: 6380,
    });

    console.log('=== String Operations ===');
    
    // Basic set/get
    await redis.set('mykey', 'Hello World');
    const value = await redis.get('mykey');
    console.log(`GET mykey: ${value}`);
    
    // Increment
    await redis.set('counter', 0);
    await redis.incr('counter');
    await redis.incr('counter');
    const counter = await redis.get('counter');
    console.log(`Counter: ${counter}`);
    
    // MSET/MGET
    await redis.mset('key1', 'value1', 'key2', 'value2', 'key3', 'value3');
    const values = await redis.mget('key1', 'key2', 'key3');
    console.log(`MGET: ${values}`);
    
    // Expiry
    await redis.set('session', 'data', 'EX', 3600);
    const ttl = await redis.ttl('session');
    console.log(`Session TTL: ${ttl} seconds`);
    
    console.log('\n=== Hash Operations ===');
    
    // Hash operations
    await redis.hset('user:1', 'name', 'John Doe');
    await redis.hset('user:1', 'email', 'john@example.com');
    await redis.hset('user:1', 'age', '30');
    
    const name = await redis.hget('user:1', 'name');
    console.log(`User name: ${name}`);
    
    const allFields = await redis.hgetall('user:1');
    console.log(`All user data:`, allFields);
    
    // Increment hash field
    await redis.hincrby('user:1', 'age', 1);
    const newAge = await redis.hget('user:1', 'age');
    console.log(`New age: ${newAge}`);
    
    console.log('\n=== List Operations ===');
    
    // List operations
    await redis.del('mylist');
    await redis.lpush('mylist', 'world');
    await redis.lpush('mylist', 'hello');
    await redis.rpush('mylist', '!');
    
    const listContents = await redis.lrange('mylist', 0, -1);
    console.log(`List contents: ${listContents}`);
    
    const listLength = await redis.llen('mylist');
    console.log(`List length: ${listLength}`);
    
    const lpopValue = await redis.lpop('mylist');
    console.log(`LPOP: ${lpopValue}`);
    
    const rpopValue = await redis.rpop('mylist');
    console.log(`RPOP: ${rpopValue}`);
    
    console.log('\n=== Set Operations ===');
    
    // Set operations
    await redis.sadd('myset', 'member1', 'member2', 'member3');
    
    const members = await redis.smembers('myset');
    console.log(`Set members: ${members}`);
    
    const isMember = await redis.sismember('myset', 'member1');
    console.log(`Is member1 in set? ${isMember}`);
    
    const cardinality = await redis.scard('myset');
    console.log(`Set cardinality: ${cardinality}`);
    
    console.log('\n=== Sorted Set Operations ===');
    
    // Sorted set operations
    await redis.zadd('leaderboard', 100, 'player1');
    await redis.zadd('leaderboard', 200, 'player2', 150, 'player3');
    
    const leaderboard = await redis.zrange('leaderboard', 0, -1, 'WITHSCORES');
    console.log(`Leaderboard: ${leaderboard}`);
    
    const revLeaderboard = await redis.zrevrange('leaderboard', 0, -1, 'WITHSCORES');
    console.log(`Reverse leaderboard: ${revLeaderboard}`);
    
    const player1Score = await redis.zscore('leaderboard', 'player1');
    console.log(`Player1 score: ${player1Score}`);
    
    const player1Rank = await redis.zrank('leaderboard', 'player1');
    console.log(`Player1 rank: ${player1Rank}`);
    
    console.log('\n=== Lua Scripting ===');
    
    // Lua script for atomic counter with limit
    const script = `
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
    `;
    
    await redis.set('limited_counter', 0);
    
    // Run script multiple times
    for (let i = 0; i < 12; i++) {
        try {
            const result = await redis.eval(script, 1, 'limited_counter', 10);
            console.log(`Increment ${i + 1}:`, result);
        } catch (e) {
            console.log(`Increment ${i + 1}: Error - ${e.message}`);
        }
    }
    
    console.log('\n=== Pipeline (Batch Operations) ===');
    
    // Pipeline for batch operations
    const pipeline = redis.pipeline();
    pipeline.set('batch1', 'value1');
    pipeline.set('batch2', 'value2');
    pipeline.set('batch3', 'value3');
    pipeline.get('batch1');
    pipeline.get('batch2');
    pipeline.get('batch3');
    
    const pipelineResults = await pipeline.exec();
    console.log('Pipeline results:', pipelineResults);
    
    console.log('\n=== Transaction (MULTI/EXEC) ===');
    
    // Transaction
    await redis.set('account:1', 100);
    await redis.set('account:2', 50);
    
    const transfer = async (fromKey, toKey, amount) => {
        const balance = parseInt(await redis.get(fromKey) || '0');
        
        if (balance < amount) {
            return { success: false, message: 'Insufficient funds' };
        }
        
        const multi = redis.multi();
        multi.decrby(fromKey, amount);
        multi.incrby(toKey, amount);
        await multi.exec();
        
        return { success: true, message: 'Transfer successful' };
    };
    
    const transferResult = await transfer('account:1', 'account:2', 30);
    console.log(`Transfer: ${transferResult.message}`);
    console.log(`Account 1: ${await redis.get('account:1')}`);
    console.log(`Account 2: ${await redis.get('account:2')}`);
    
    console.log('\n=== Pub/Sub ===');
    
    // Pub/Sub example
    const subscriber = new Redis({ host: 'localhost', port: 6380 });
    const publisher = new Redis({ host: 'localhost', port: 6380 });
    
    await subscriber.subscribe('notifications');
    
    subscriber.on('message', (channel, message) => {
        console.log(`Received on ${channel}: ${message}`);
    });
    
    // Wait a bit for subscription to be ready
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Publish messages
    await publisher.publish('notifications', 'Hello subscribers!');
    await publisher.publish('notifications', 'Another message');
    
    // Wait for messages to be received
    await new Promise(resolve => setTimeout(resolve, 100));
    
    await subscriber.unsubscribe('notifications');
    subscriber.disconnect();
    publisher.disconnect();
    
    console.log('\n=== Cleanup ===');
    await redis.flushdb();
    console.log('Database flushed!');
    
    console.log('\n=== Server Info ===');
    const info = await redis.info();
    console.log('Server info:', info.split('\r\n').slice(0, 10).join('\n'));
    
    // Disconnect
    redis.disconnect();
}

main().catch(console.error);
