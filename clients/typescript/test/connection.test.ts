import { test } from 'node:test';
import * as assert from 'node:assert/strict';
import { CacheStormClient } from '../src/client';
import { ReplyError, ProtocolError } from '../src/resp';
import { Message } from '../src/types';
import { startFakeServer } from './fake-server';

async function waitFor(condition: () => boolean, timeoutMs = 2000): Promise<void> {
  const start = Date.now();
  while (!condition()) {
    if (Date.now() - start > timeoutMs) throw new Error('waitFor timed out');
    await new Promise(resolve => setTimeout(resolve, 2));
  }
}

test('connect sends AUTH and SELECT before resolving', async () => {
  const server = await startFakeServer({ password: 's3cret' });
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port, password: 's3cret', database: 3 });
  try {
    await client.connect();
    assert.deepEqual(server.commands, [['AUTH', 's3cret'], ['SELECT', '3']]);
    assert.equal(await client.set('k', 'v'), 'OK');
  } finally {
    await client.quit();
    await server.close();
  }
});

test('connect rejects when AUTH fails and leaves the client disconnected', async () => {
  const server = await startFakeServer({ password: 's3cret' });
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port, password: 'wrong' });
  try {
    await assert.rejects(client.connect(), (err: unknown) => err instanceof ReplyError && /^WRONGPASS/.test(err.message));
    await assert.rejects(client.get('k'), /Not connected/);
  } finally {
    await server.close();
  }
});

test('no AUTH or SELECT is sent without password or database', async () => {
  const server = await startFakeServer();
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port });
  try {
    await client.connect();
    assert.deepEqual(server.commands, []);
  } finally {
    await client.quit();
    await server.close();
  }
});

test('getBuffer returns binary values unchanged', async () => {
  const binary = Buffer.from([0x00, 0xff, 0xfe, 0x0d, 0x0a, 0xc3]);
  const server = await startFakeServer({ binary });
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port });
  try {
    await client.connect();
    const value = await client.getBuffer('bin');
    assert.ok(Buffer.isBuffer(value));
    assert.deepEqual(value, binary);
    assert.equal(await client.getBuffer('missing'), null);
    assert.equal(typeof (await client.get('bin')), 'string');
  } finally {
    await client.quit();
    await server.close();
  }
});

test('unimplemented options are rejected instead of silently ignored', async () => {
  assert.throws(() => new CacheStormClient({ pool: { max: 2 } } as any), /"pool" is not implemented/);
  assert.throws(() => new CacheStormClient({ retry: { maxRetries: 1 } } as any), /"retry" is not implemented/);
  const client = new CacheStormClient();
  await assert.rejects(client.set('k', 'v', { namespace: 'x' } as any), /"namespace" is not implemented/);
});

test('pub/sub delivers message and pmessage on a dedicated connection', async () => {
  const server = await startFakeServer();
  const publisher = new CacheStormClient({ host: '127.0.0.1', port: server.port });
  await publisher.connect();
  const received: Message[] = [];
  const sub = publisher.subscribe(['news', 'sport'], message => received.push(message));
  try {
    await sub.subscribe();
    await sub.psubscribe('wea*');
    assert.equal(sub.subscribed, true);
    assert.equal(await publisher.publish('news', 'héllo'), 1);
    assert.equal(await publisher.publish('weather', 'rain'), 1);
    assert.equal(await publisher.publish('other', 'x'), 0);
    await waitFor(() => received.length >= 2);
    assert.deepEqual(received, [
      { channel: 'news', payload: 'héllo' },
      { pattern: 'wea*', channel: 'weather', payload: 'rain' },
    ]);
    assert.equal(await publisher.get('missing'), null);

    await sub.unsubscribe('news');
    assert.deepEqual(sub.getChannels(), ['sport']);
    assert.equal(await publisher.publish('news', 'gone'), 0);
    assert.equal(await publisher.publish('sport', 'goal'), 1);
    await waitFor(() => received.length >= 3);
    assert.deepEqual(received[2], { channel: 'sport', payload: 'goal' });
  } finally {
    await sub.quit();
    assert.equal(sub.subscribed, false);
    assert.equal(await publisher.publish('sport', 'after'), 0);
    await publisher.quit();
    await server.close();
  }
  const subscribeCommands = server.commands.filter(c => /SUBSCRIBE/.test(c[0]));
  assert.deepEqual(subscribeCommands, [
    ['SUBSCRIBE', 'news', 'sport'],
    ['PSUBSCRIBE', 'wea*'],
    ['UNSUBSCRIBE', 'news'],
    ['UNSUBSCRIBE', 'sport'],
    ['PUNSUBSCRIBE', 'wea*'],
  ]);
});

test('pub/sub subscribe rejects when the server returns an error', async () => {
  const server = await startFakeServer({ password: 'pw' });
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port, password: 'pw' });
  await client.connect();
  const sub = client.subscribe('c');
  const internal = client as any;
  internal.options.password = '';
  try {
    await assert.rejects(sub.subscribe(), /NOAUTH/);
  } finally {
    await sub.quit();
    await client.quit();
    await server.close();
  }
});

test('a reply with no pending request emits a ProtocolError and drops the connection', async () => {
  const client = new CacheStormClient();
  const internal = client as any;
  let destroyed = false;
  internal.socket = { write() {}, destroy() { destroyed = true; } };
  internal.connected = true;
  const errors: unknown[] = [];
  client.on('error', err => errors.push(err));
  internal.handleResponse(Buffer.from('+unsolicited\r\n'));
  assert.equal(errors.length, 1);
  assert.ok(errors[0] instanceof ProtocolError);
  assert.equal(destroyed, true);
});

test('hgetall handles null and rejects malformed replies', async () => {
  const client = new CacheStormClient();
  const internal = client as any;
  internal.socket = { write() {}, destroy() {} };
  internal.connected = true;
  const empty = client.hgetall('missing');
  internal.handleResponse(Buffer.from('*-1\r\n'));
  assert.deepEqual(await empty, {});
  const pairs = client.hgetall('h');
  internal.handleResponse(Buffer.from('*4\r\n$1\r\na\r\n$1\r\n1\r\n$1\r\nb\r\n$1\r\n2\r\n'));
  assert.deepEqual(await pairs, { a: '1', b: '2' });
  const wrong = client.hgetall('h');
  internal.handleResponse(Buffer.from(':5\r\n'));
  await assert.rejects(wrong, ProtocolError);
  const odd = client.hgetall('h');
  internal.handleResponse(Buffer.from('*1\r\n$1\r\na\r\n'));
  await assert.rejects(odd, ProtocolError);
});

test('a command after the connection dropped reconnects and replays AUTH and SELECT', async () => {
  const server = await startFakeServer({ password: 's3cret' });
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port, password: 's3cret', database: 2 });
  let reconnects = 0;
  client.on('reconnect', () => reconnects++);
  try {
    await client.connect();
    let closed = false;
    client.once('close', () => { closed = true; });
    server.dropClients();
    await waitFor(() => closed);
    server.commands.length = 0;
    assert.equal(await client.set('k', 'v'), 'OK');
    assert.deepEqual(server.commands, [['AUTH', 's3cret'], ['SELECT', '2'], ['SET', 'k', 'v']]);
    assert.equal(reconnects, 1);
  } finally {
    await client.quit();
    await server.close();
  }
});

test('no reconnect after quit, with reconnect off, or when the server stays down', async () => {
  const server = await startFakeServer();
  const client = new CacheStormClient({ host: '127.0.0.1', port: server.port });
  await client.connect();
  await client.quit();
  await assert.rejects(client.set('k', 'v'), /Not connected/);

  const manual = new CacheStormClient({ host: '127.0.0.1', port: server.port, reconnect: false });
  await manual.connect();
  let closed = false;
  manual.once('close', () => { closed = true; });
  server.dropClients();
  await waitFor(() => closed);
  await assert.rejects(manual.set('k', 'v'), /Not connected/);

  const doomed = new CacheStormClient({ host: '127.0.0.1', port: server.port, reconnectAttempts: 2 });
  doomed.on('error', () => {});
  await doomed.connect();
  closed = false;
  doomed.once('close', () => { closed = true; });
  await server.close();
  await waitFor(() => closed);
  await assert.rejects(doomed.set('k', 'v'), /ECONNREFUSED/);
});
