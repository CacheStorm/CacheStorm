import { test } from 'node:test';
import * as assert from 'node:assert/strict';
import { RespDecoder, ReplyError, ProtocolError, encodeCommand } from '../src/resp';

function feedAll(parts: (string | Buffer)[]) {
  const decoder = new RespDecoder();
  const out: unknown[] = [];
  for (const part of parts) out.push(...decoder.feed(Buffer.isBuffer(part) ? part : Buffer.from(part)));
  return out;
}

test('decodes simple strings, integers and bulk strings', () => {
  assert.deepEqual(feedAll(['+OK\r\n', ':42\r\n', ':-7\r\n', '$5\r\nhello\r\n', '$0\r\n\r\n']), ['OK', 42, -7, 'hello', '']);
});

test('decodes null bulk strings and null arrays', () => {
  assert.deepEqual(feedAll(['$-1\r\n*-1\r\n*0\r\n']), [null, null, []]);
});

test('decodes error replies as ReplyError values', () => {
  const [err] = feedAll(['-ERR rejected\r\n']);
  assert.ok(err instanceof ReplyError);
  assert.equal((err as ReplyError).message, 'ERR rejected');
});

test('reassembles a reply split across chunks at every byte boundary', () => {
  const frame = Buffer.from('*3\r\n$5\r\nhello\r\n:12\r\n*2\r\n+a\r\n$-1\r\n');
  for (let cut = 1; cut < frame.length; cut++) {
    assert.deepEqual(feedAll([frame.subarray(0, cut), frame.subarray(cut)]), [['hello', 12, ['a', null]]], `cut=${cut}`);
  }
  const bytes = Array.from(frame, b => Buffer.from([b]));
  assert.deepEqual(feedAll(bytes), [['hello', 12, ['a', null]]]);
});

test('returns several coalesced replies from one chunk in order', () => {
  assert.deepEqual(feedAll(['+OK\r\n$5\r\nvalue\r\n:1\r\n-ERR x\r\n$-1\r\n']).map(v => v instanceof ReplyError ? `E:${v.message}` : v), ['OK', 'value', 1, 'E:ERR x', null]);
});

test('keeps a trailing partial reply buffered after complete ones', () => {
  const decoder = new RespDecoder();
  assert.deepEqual(decoder.feed(Buffer.from('+OK\r\n$5\r\nva')), ['OK']);
  assert.deepEqual(decoder.feed(Buffer.from('l')), []);
  assert.deepEqual(decoder.feed(Buffer.from('ue\r\n:3\r')), ['value']);
  assert.deepEqual(decoder.feed(Buffer.from('\n')), [3]);
});

test('decodes nested arrays with embedded errors', () => {
  const [value] = feedAll(['*2\r\n*2\r\n:1\r\n*1\r\n+deep\r\n-WRONGTYPE bad\r\n']) as unknown[][];
  assert.deepEqual(value[0], [1, ['deep']]);
  assert.ok(value[1] instanceof ReplyError);
});

test('uses byte lengths for multi-byte UTF-8 bulk strings', () => {
  const text = 'şğü€😀';
  const payload = Buffer.from(text, 'utf8');
  const frame = Buffer.concat([Buffer.from(`$${payload.length}\r\n`), payload, Buffer.from('\r\n')]);
  assert.deepEqual(feedAll([frame]), [text]);
  assert.deepEqual(feedAll([frame.subarray(0, 6), frame.subarray(6)]), [text]);
});

test('bulk strings may contain CRLF bytes', () => {
  assert.deepEqual(feedAll(['$4\r\na\r\nb\r\n']), ['a\r\nb']);
});

test('large bulk string delivered in many chunks', () => {
  const payload = Buffer.alloc(200000, 'x');
  const frame = Buffer.concat([Buffer.from(`$${payload.length}\r\n`), payload, Buffer.from('\r\n+next\r\n')]);
  const parts: Buffer[] = [];
  for (let i = 0; i < frame.length; i += 1000) parts.push(frame.subarray(i, i + 1000));
  const out = feedAll(parts);
  assert.equal(out.length, 2);
  assert.equal((out[0] as string).length, payload.length);
  assert.equal(out[1], 'next');
});

test('rejects unknown type bytes and malformed lengths', () => {
  assert.throws(() => feedAll(['?x\r\n']), ProtocolError);
  assert.throws(() => feedAll(['$abc\r\n']), ProtocolError);
  assert.throws(() => feedAll([':1.5\r\n']), ProtocolError);
});

test('encodes commands using byte lengths and raw Buffer arguments', () => {
  assert.deepEqual(encodeCommand(['SET', 'key', 'ş']), Buffer.from('*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$2\r\nş\r\n'));
  assert.deepEqual(encodeCommand(['EXPIRE', 'k', 10]), Buffer.from('*3\r\n$6\r\nEXPIRE\r\n$1\r\nk\r\n$2\r\n10\r\n'));
  const binary = Buffer.from([0x00, 0xff, 0x0d, 0x0a]);
  assert.deepEqual(encodeCommand(['SET', 'b', binary]), Buffer.concat([Buffer.from('*3\r\n$3\r\nSET\r\n$1\r\nb\r\n$4\r\n'), binary, Buffer.from('\r\n')]));
});
