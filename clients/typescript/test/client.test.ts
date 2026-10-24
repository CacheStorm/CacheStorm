import { test } from 'node:test';
import * as assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import * as net from 'node:net';
import { AddressInfo } from 'node:net';
import { CacheStormClient } from '../src/client';
import { Pipeline, PipelineError } from '../src/pipeline';
import { ReplyError, RespDecoder } from '../src/resp';

class FakeSocket extends EventEmitter {
  writes: Buffer[] = [];
  destroyed = false;
  write(data: Buffer): boolean {
    this.writes.push(Buffer.from(data));
    return true;
  }
  end(): void {}
  destroy(): void {
    this.destroyed = true;
  }
  reply(text: string | Buffer): void {
    this.emit('data', Buffer.isBuffer(text) ? text : Buffer.from(text));
  }
  sentCommands(): unknown[] {
    return new RespDecoder().feed(Buffer.concat(this.writes));
  }
}

function fakeClient(options: { commandTimeout?: number } = {}) {
  const client = new CacheStormClient(options);
  const socket = new FakeSocket();
  const internal = client as any;
  internal.socket = socket;
  internal.connected = true;
  socket.on('data', (data: Buffer) => internal.handleResponse(data));
  return { client, socket, internal };
}

test('matches pipelined replies to requests in FIFO order', async () => {
  const { client, socket } = fakeClient();
  const a = client.get('a');
  const b = client.del('b');
  const c = client.get('c');
  socket.reply('$1\r\n1\r\n:5\r\n$-');
  socket.reply('1\r\n');
  assert.deepEqual(await Promise.all([a, b, c]), ['1', 5, null]);
});

test('rejects only the request whose reply is an error', async () => {
  const { client, socket } = fakeClient();
  const a = client.get('a');
  const b = client.lpush('a', 'x');
  const c = client.get('c');
  socket.reply('$1\r\nv\r\n-WRONGTYPE Operation against a key holding the wrong kind of value\r\n+OK\r\n');
  assert.equal(await a, 'v');
  await assert.rejects(b, (err: unknown) => err instanceof ReplyError && /^WRONGTYPE/.test(err.message));
  assert.equal(await c, 'OK');
});

test('encodes UTF-8 arguments by byte length on the wire', async () => {
  const { client, socket } = fakeClient();
  const pending = client.set('k', 'çağ€');
  assert.deepEqual(socket.sentCommands(), [['SET', 'k', 'çağ€']]);
  socket.reply('+OK\r\n');
  assert.equal(await pending, 'OK');
});

test('pipeline writes all commands in one write and returns real replies', async () => {
  const { client, socket } = fakeClient();
  const pending = new Pipeline(client).set('k', 'v').get('k').del('k').get('missing').exec();
  assert.equal(socket.writes.length, 1);
  assert.deepEqual(socket.sentCommands(), [['SET', 'k', 'v'], ['GET', 'k'], ['DEL', 'k'], ['GET', 'missing']]);
  socket.reply('+OK\r\n$1\r');
  socket.reply('\nv\r\n:1\r\n$-1\r\n');
  assert.deepEqual(await pending, ['OK', 'v', 1, null]);
});

test('pipeline is cleared after exec and an empty pipeline resolves to []', async () => {
  const { client, socket } = fakeClient();
  const pipeline = client.pipeline().get('a');
  const first = pipeline.exec();
  socket.reply('$-1\r\n');
  assert.deepEqual(await first, [null]);
  assert.deepEqual(await pipeline.exec(), []);
  assert.equal(socket.writes.length, 1);
});

test('pipeline rejects with PipelineError carrying every result when a reply is an error', async () => {
  const { client, socket } = fakeClient();
  const pending = client.pipeline().set('k', 'v').lpush('k', 'x').get('k').exec();
  socket.reply('+OK\r\n-WRONGTYPE bad\r\n$1\r\nv\r\n');
  const err = await pending.then(() => assert.fail('expected rejection'), (e: unknown) => e);
  assert.ok(err instanceof PipelineError);
  assert.ok(err.cause instanceof ReplyError);
  assert.equal(err.results[0], 'OK');
  assert.ok(err.results[1] instanceof ReplyError);
  assert.equal(err.results[2], 'v');
});

test('pipeline on a disconnected client rejects without writing', async () => {
  const client = new CacheStormClient();
  await assert.rejects(client.pipeline().get('a').exec(), /Not connected/);
  await assert.rejects(client.get('a'), /Not connected/);
});

test('socket close rejects all pending requests', async () => {
  const { client, socket } = fakeClient();
  const a = client.get('a');
  const b = client.get('b');
  (client as any).failPending(new Error('Connection closed'));
  await assert.rejects(a, /Connection closed/);
  await assert.rejects(b, /Connection closed/);
  assert.equal((client as any).pending.length, 0);
  socket.reply('+late\r\n');
});

test('timed-out request keeps its slot so later replies stay aligned', async () => {
  const { client, socket } = fakeClient({ commandTimeout: 20 });
  const slow = client.get('slow');
  await assert.rejects(slow, /Command timeout/);
  const next = client.get('next');
  socket.reply('$4\r\nslow\r\n$4\r\nnext\r\n');
  assert.equal(await next, 'next');
});

test('protocol error rejects pending requests and destroys the socket', async () => {
  const { client, socket } = fakeClient();
  const a = client.get('a');
  socket.reply('?garbage\r\n');
  await assert.rejects(a, /Unknown RESP type/);
  assert.equal(socket.destroyed, true);
});

test('round trip over a real TCP socket with fragmented, coalesced replies', async () => {
  const received: unknown[] = [];
  const server = net.createServer(conn => {
    const decoder = new RespDecoder();
    conn.on('data', data => {
      received.push(...decoder.feed(data));
      if (received.length === 3) {
        const reply = Buffer.from('+OK\r\n$5\r\nçağ\r\n*2\r\n:1\r\n$-1\r\n');
        conn.write(reply.subarray(0, 9));
        setTimeout(() => conn.write(reply.subarray(9)), 10);
      }
    });
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const client = new CacheStormClient({ host: '127.0.0.1', port: (server.address() as AddressInfo).port });
  try {
    await client.connect();
    const results = await client.pipeline().set('k', 'çağ').get('k').lpush('l', 'x').exec();
    assert.deepEqual(received, [['SET', 'k', 'çağ'], ['GET', 'k'], ['LPUSH', 'l', 'x']]);
    assert.deepEqual(results, ['OK', 'çağ', [1, null]]);
    const pending = client.get('never');
    await new Promise(resolve => setTimeout(resolve, 10));
    (client as any).socket.destroy();
    await assert.rejects(pending, /Connection closed/);
  } finally {
    server.close();
  }
});
