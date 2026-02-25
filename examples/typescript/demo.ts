/**
 * CacheStorm TypeScript SDK Demo
 *
 * This demo shows all major features of the CacheStorm TypeScript client.
 * Run this after starting CacheStorm server.
 */

import { CacheStormClient } from '../../clients/typescript/src';

function printHeader(title: string): void {
  console.log(`\n→ ${title}`);
}

function printSuccess(msg: string): void {
  console.log(`  ✓ ${msg}`);
}

function printError(msg: string): void {
  console.log(`  ✗ ${msg}`);
}

async function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function main(): Promise<void> {
  console.log('='.repeat(64));
  console.log('  CacheStorm TypeScript SDK Demo');
  console.log('='.repeat(64));

  // Create client
  const client = new CacheStormClient({
    host: 'localhost',
    port: 6379,
  });

  try {
    // Test connection
    printHeader('Testing connection...');
    try {
      await client.connect();
      printSuccess('Connected to CacheStorm');
    } catch (error) {
      printError(`Failed to connect: ${error}`);
      console.log('\n  Make sure CacheStorm is running:');
      console.log('    docker run -d -p 6379:6379 cachestorm/cachestorm:latest');
      return;
    }

    // String operations
    printHeader('String Operations:');

    // SET
    await client.set('demo:key', 'Hello from TypeScript!');
    printSuccess("SET demo:key 'Hello from TypeScript!'");

    // GET
    const value = await client.get('demo:key');
    printSuccess(`GET demo:key = ${value}`);

    // SET with expiration
    await client.set('demo:temp', 'Expires soon', { ex: 10 });
    printSuccess('SET demo:temp with 10s expiration');

    // INCR
    await client.del('demo:counter');
    const newVal = await client.incr('demo:counter');
    printSuccess(`INCR demo:counter = ${newVal}`);

    // Hash operations
    printHeader('Hash Operations:');

    // HSET
    const hsetResult = await client.hset('demo:user:1', 'name', 'TypeScript User');
    printSuccess(`HSET demo:user:1 name = ${hsetResult}`);

    await client.hset('demo:user:1', 'email', 'ts@example.com');
    await client.hset('demo:user:1', 'framework', 'Node.js');

    // HGET
    const name = await client.hget('demo:user:1', 'name');
    printSuccess(`HGET demo:user:1 name = ${name}`);

    // HGETALL
    const user = await client.hgetall('demo:user:1');
    printSuccess(`HGETALL demo:user:1 = ${JSON.stringify(user)}`);

    // List operations
    printHeader('List Operations:');

    // LPUSH
    const lpushResult = await client.lpush('demo:queue', 'task1', 'task2', 'task3');
    printSuccess(`LPUSH demo:queue (3 items) = ${lpushResult}`);

    // LRANGE
    const tasks = await client.lrange('demo:queue', 0, -1);
    printSuccess(`LRANGE demo:queue = ${JSON.stringify(tasks)}`);

    // LPOP
    const task = await client.lpop('demo:queue');
    printSuccess(`LPOP demo:queue = ${task}`);

    // Set operations
    printHeader('Set Operations:');

    // SADD
    const saddResult = await client.sadd('demo:tags', 'typescript', 'redis', 'cache');
    printSuccess(`SADD demo:tags (3 items) = ${saddResult}`);

    // SMEMBERS
    const tags = await client.smembers('demo:tags');
    printSuccess(`SMEMBERS demo:tags = ${JSON.stringify(tags)}`);

    // SISMEMBER
    const isMember = await client.sismember('demo:tags', 'typescript');
    printSuccess(`SISMEMBER demo:tags typescript = ${isMember}`);

    // Sorted Set operations
    printHeader('Sorted Set Operations:');

    // ZADD
    await client.zadd('demo:leaderboard', { alice: 100, bob: 200, charlie: 150 });
    printSuccess('ZADD demo:leaderboard (3 players)');

    // ZRANGE
    const leaders = await client.zrange('demo:leaderboard', 0, -1);
    printSuccess(`ZRANGE demo:leaderboard = ${JSON.stringify(leaders)}`);

    // CacheStorm-specific: Tags
    printHeader('CacheStorm Tags (Unique Feature):');

    // Set with tags
    await client.setWithTags('demo:product:1', 'Product Data', ['products', 'featured']);
    printSuccess('SET demo:product:1 with tags [products, featured]');

    await client.setWithTags('demo:product:2', 'Another Product', ['products']);
    await client.setWithTags('demo:article:1', 'Article Content', ['articles', 'featured']);

    // Get keys by tag
    const productKeys = await client.tagKeys('products');
    printSuccess(`TAGKEYS products = ${JSON.stringify(productKeys)}`);

    // Get tags of a key
    const keyTags = await client.tags('demo:product:1');
    printSuccess(`TAGS demo:product:1 = ${JSON.stringify(keyTags)}`);

    // Invalidate by tag
    const invalidated = await client.invalidate('featured');
    printSuccess(`INVALIDATE featured = ${invalidated} keys removed`);

    // Pipeline demo
    printHeader('Pipeline (Batch Operations):');

    const pipeline = client.pipeline();
    pipeline.set('demo:pipe:1', 'value1');
    pipeline.set('demo:pipe:2', 'value2');
    pipeline.set('demo:pipe:3', 'value3');
    pipeline.get('demo:pipe:1');

    const pipeResults = await pipeline.execute();
    printSuccess(`Pipeline executed ${pipeResults.length} commands`);

    // Pub/Sub demo
    printHeader('Pub/Sub (brief demo):');
    console.log('  Starting pub/sub demo...');

    // Create subscriber
    const subscriber = client.duplicate();
    await subscriber.connect();

    const pubsub = subscriber.pubsub();

    // Subscribe with message handler
    let receivedMessage: string | null = null;
    pubsub.subscribe('demo:channel', (message) => {
      receivedMessage = message.data as string;
      console.log(`  ✓ Received: ${message.data}`);
    });

    await sleep(500); // Let subscriber connect

    // Publish message
    const published = await client.publish('demo:channel', 'Hello from pub/sub!');
    printSuccess(`Published message to demo:channel (${published} subscribers)`);

    await sleep(500); // Let message be received

    pubsub.unsubscribe();
    await subscriber.disconnect();

    // Cleanup
    printHeader('Cleanup:');
    const deleted = await client.del(
      'demo:key', 'demo:temp', 'demo:counter',
      'demo:user:1', 'demo:queue', 'demo:tags',
      'demo:leaderboard', 'demo:product:1', 'demo:product:2',
      'demo:article:1', 'demo:pipe:1', 'demo:pipe:2', 'demo:pipe:3'
    );
    printSuccess(`Deleted ${deleted} demo keys`);

    console.log('\n' + '='.repeat(64));
    console.log('  Demo completed successfully!');
    console.log('='.repeat(64));

  } finally {
    await client.disconnect();
  }
}

// Run the demo
main().catch((error) => {
  console.error('Demo failed:', error);
  process.exit(1);
});
