import { Command } from './types';

export class ReplyError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ReplyError';
  }
}

export class ProtocolError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ProtocolError';
  }
}

export type RespValue = string | Buffer | number | null | ReplyError | RespValue[];

export interface RespDecoderOptions {
  returnBuffers?: boolean;
}

const CRLF = Buffer.from('\r\n');
const INCOMPLETE = Symbol('incomplete');

interface Parsed {
  value: RespValue;
  pos: number;
}

export function encodeCommand(command: Command): Buffer {
  const parts: Buffer[] = [Buffer.from(`*${command.length}\r\n`)];
  for (const arg of command) {
    const payload = Buffer.isBuffer(arg) ? arg : Buffer.from(String(arg), 'utf8');
    parts.push(Buffer.from(`$${payload.length}\r\n`), payload, CRLF);
  }
  return Buffer.concat(parts);
}

export class RespDecoder {
  private buffer: Buffer = Buffer.alloc(0);
  private chunks: Buffer[] = [];
  private chunkBytes = 0;
  private needed = 0;
  private readonly returnBuffers: boolean;

  constructor(options: RespDecoderOptions = {}) {
    this.returnBuffers = options.returnBuffers ?? false;
  }

  reset(): void {
    this.buffer = Buffer.alloc(0);
    this.chunks = [];
    this.chunkBytes = 0;
    this.needed = 0;
  }

  feed(data: Buffer): RespValue[] {
    this.chunks.push(data);
    this.chunkBytes += data.length;
    if (this.buffer.length + this.chunkBytes < this.needed) {
      return [];
    }
    this.buffer = Buffer.concat([this.buffer, ...this.chunks]);
    this.chunks = [];
    this.chunkBytes = 0;
    this.needed = 0;

    const replies: RespValue[] = [];
    let pos = 0;
    while (pos < this.buffer.length) {
      const parsed = this.parse(pos);
      if (parsed === INCOMPLETE) break;
      replies.push(parsed.value);
      pos = parsed.pos;
    }
    this.buffer = pos >= this.buffer.length ? Buffer.alloc(0) : this.buffer.subarray(pos);
    if (this.needed > 0) this.needed -= pos;
    return replies;
  }

  private readLine(pos: number): { line: string; pos: number } | typeof INCOMPLETE {
    const end = this.buffer.indexOf(CRLF, pos);
    if (end === -1) return INCOMPLETE;
    return { line: this.buffer.toString('utf8', pos, end), pos: end + 2 };
  }

  private parseLength(text: string): number {
    if (!/^-?\d+$/.test(text)) {
      throw new ProtocolError(`Invalid length: ${JSON.stringify(text)}`);
    }
    return Number(text);
  }

  private parse(start: number): Parsed | typeof INCOMPLETE {
    if (start >= this.buffer.length) return INCOMPLETE;
    const type = String.fromCharCode(this.buffer[start]);
    const header = this.readLine(start + 1);
    if (header === INCOMPLETE) return INCOMPLETE;
    const { line, pos } = header;

    switch (type) {
      case '+':
        return { value: line, pos };
      case '-':
        return { value: new ReplyError(line), pos };
      case ':': {
        if (!/^[-+]?\d+$/.test(line)) {
          throw new ProtocolError(`Invalid integer: ${JSON.stringify(line)}`);
        }
        return { value: Number(line), pos };
      }
      case '$': {
        const length = this.parseLength(line);
        if (length < 0) return { value: null, pos };
        const end = pos + length;
        if (this.buffer.length < end + 2) {
          this.needed = end + 2;
          return INCOMPLETE;
        }
        if (this.buffer[end] !== 0x0d || this.buffer[end + 1] !== 0x0a) {
          throw new ProtocolError('Bulk string is not terminated by CRLF');
        }
        const value = this.returnBuffers
          ? Buffer.from(this.buffer.subarray(pos, end))
          : this.buffer.toString('utf8', pos, end);
        return { value, pos: end + 2 };
      }
      case '*': {
        const count = this.parseLength(line);
        if (count < 0) return { value: null, pos };
        const items: RespValue[] = [];
        let cursor = pos;
        for (let i = 0; i < count; i++) {
          const item = this.parse(cursor);
          if (item === INCOMPLETE) return INCOMPLETE;
          items.push(item.value);
          cursor = item.pos;
        }
        return { value: items, pos: cursor };
      }
      default:
        throw new ProtocolError(`Unknown RESP type byte: ${JSON.stringify(type)}`);
    }
  }
}

export function buffersToStrings(value: RespValue): RespValue {
  if (Buffer.isBuffer(value)) return value.toString('utf8');
  if (Array.isArray(value)) return value.map(buffersToStrings);
  return value;
}
