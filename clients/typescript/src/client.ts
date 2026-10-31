import { EventEmitter } from 'events';
import * as net from 'net';
import {
  CacheStormClientOptions,
  SetOptions,
  Message,
  Command,
} from './types';
import { Pipeline } from './pipeline';
import { PubSub } from './pubsub';
import { RespDecoder, RespValue, ReplyError, ProtocolError, encodeCommand, buffersToStrings } from './resp';

export type PushHandler = (reply: RespValue) => void;

const PUSH_KINDS = new Set(['message', 'pmessage', 'subscribe', 'psubscribe', 'unsubscribe', 'punsubscribe']);

interface PendingRequest {
  command?: Command;
  resolve: Function;
  reject: Function;
  timer?: ReturnType<typeof setTimeout>;
  settled?: boolean;
  returnBuffers?: boolean;
}

export class CacheStormClient extends EventEmitter {
  private socket: net.Socket | null = null;
  private options: Required<CacheStormClientOptions>;
  private connected = false;
  private pending: PendingRequest[] = [];
  private decoder = new RespDecoder({ returnBuffers: true });
  private pushHandler: PushHandler | null = null;
  // wasConnected allows reconnecting; quit and a failed first connect leave
  // it false. reconnecting is the reconnect in progress.
  private wasConnected = false;
  private reconnecting: Promise<void> | null = null;

  private get currentCommand(): PendingRequest | null {
    return this.pending[0] ?? null;
  }

  private set currentCommand(request: PendingRequest | null) {
    if (request) this.pending.push(request);
  }

  constructor(options: CacheStormClientOptions = {}) {
    super();
    for (const unsupported of ['pool', 'retry']) {
      if ((options as Record<string, unknown>)[unsupported] !== undefined) {
        throw new Error(`CacheStormClient option "${unsupported}" is not implemented`);
      }
    }
    this.options = {
      host: options.host ?? 'localhost',
      port: options.port ?? 6379,
      password: options.password ?? '',
      database: options.database ?? 0,
      connectTimeout: options.connectTimeout ?? 10000,
      commandTimeout: options.commandTimeout ?? 5000,
      reconnect: options.reconnect ?? true,
      reconnectAttempts: options.reconnectAttempts ?? 3,
    };
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      let settled = false;
      const timeout = setTimeout(() => {
        if (settled) return;
        settled = true;
        socket.destroy();
        reject(new Error('Connection timeout'));
      }, this.options.connectTimeout);

      this.decoder.reset();
      const socket = net.createConnection({
        host: this.options.host,
        port: this.options.port,
      });
      this.socket = socket;
      socket.setNoDelay(true);

      socket.on('connect', () => {
        if (settled) return;
        this.connected = true;
        this.handshake().then(
          () => {
            clearTimeout(timeout);
            if (settled) return;
            settled = true;
            this.wasConnected = true;
            this.emit('connect');
            resolve();
          },
          (err: Error) => {
            clearTimeout(timeout);
            if (settled) return;
            settled = true;
            this.connected = false;
            socket.destroy();
            reject(err);
          }
        );
      });

      socket.on('error', (err) => {
        clearTimeout(timeout);
        this.failPending(err);
        if (this.listenerCount('error') > 0) this.emit('error', err);
        if (!settled) {
          settled = true;
          reject(err);
        }
      });

      socket.on('data', (data) => {
        this.handleResponse(data);
      });

      socket.on('close', () => {
        clearTimeout(timeout);
        this.connected = false;
        if (this.socket === socket) this.socket = null;
        this.decoder.reset();
        this.failPending(new Error('Connection closed'));
        this.emit('close');
        if (!settled) {
          settled = true;
          reject(new Error('Connection closed'));
        }
      });
    });
  }

  private async handshake(): Promise<void> {
    if (this.options.password) {
      await this.sendCommand(['AUTH', this.options.password]);
    }
    if (this.options.database) {
      await this.sendCommand(['SELECT', this.options.database]);
    }
  }

  get commandTimeout(): number {
    return this.options.commandTimeout;
  }

  async quit(): Promise<void> {
    this.wasConnected = false;
    if (!this.connected) return;
    const socket = this.socket;
    try {
      await this.sendCommand(['QUIT']);
    } catch {
    } finally {
      this.connected = false;
      socket?.end();
    }
  }

  async ping(message?: string): Promise<string> {
    const cmd = message ? ['PING', message] : ['PING'];
    return this.sendCommand(cmd) as Promise<string>;
  }

  async set(key: string, value: string | number | Buffer, options?: SetOptions): Promise<string> {
    const args: Command = ['SET', key, value];

    if (options?.EX) args.push('EX', options.EX);
    if (options?.PX) args.push('PX', options.PX);
    if (options?.NX) args.push('NX');
    if (options?.XX) args.push('XX');
    if ((options as Record<string, unknown> | undefined)?.namespace !== undefined) {
      throw new Error('SET option "namespace" is not implemented');
    }
    if (options?.TAGS) {
      args.push('TAGS');
      args.push(...options.TAGS);
    }

    return this.sendCommand(args) as Promise<string>;
  }

  async setWithTags(key: string, value: string | number | Buffer, tags: string[]): Promise<string> {
    return this.set(key, value, { TAGS: tags });
  }

  async get(key: string): Promise<string | null> {
    return this.sendCommand(['GET', key]) as Promise<string | null>;
  }

  async getBuffer(key: string): Promise<Buffer | null> {
    return this.sendCommands([['GET', key]], { returnBuffers: true })[0] as Promise<Buffer | null>;
  }

  async del(...keys: string[]): Promise<number> {
    return this.sendCommand(['DEL', ...keys]) as Promise<number>;
  }

  async exists(...keys: string[]): Promise<number> {
    return this.sendCommand(['EXISTS', ...keys]) as Promise<number>;
  }

  async expire(key: string, seconds: number): Promise<number> {
    return this.sendCommand(['EXPIRE', key, seconds]) as Promise<number>;
  }

  async ttl(key: string): Promise<number> {
    return this.sendCommand(['TTL', key]) as Promise<number>;
  }

  // Hash commands
  async hset(key: string, field: string, value: string | number | Buffer): Promise<number>;
  async hset(key: string, fields: Record<string, string | number | Buffer>): Promise<number>;
  async hset(
    key: string,
    fieldOrFields: string | Record<string, string | number | Buffer>,
    value?: string | number | Buffer
  ): Promise<number> {
    const args: Command = ['HSET', key];

    if (typeof fieldOrFields === 'string') {
      args.push(fieldOrFields, value!);
    } else {
      for (const [field, val] of Object.entries(fieldOrFields)) {
        args.push(field, val);
      }
    }

    return this.sendCommand(args) as Promise<number>;
  }

  async hget(key: string, field: string): Promise<string | null> {
    return this.sendCommand(['HGET', key, field]) as Promise<string | null>;
  }

  async hgetall(key: string): Promise<Record<string, string>> {
    const result = await this.sendCommand(['HGETALL', key]);
    const obj: Record<string, string> = {};
    if (result === null) return obj;
    if (!Array.isArray(result) || result.length % 2 !== 0) {
      throw new ProtocolError('HGETALL expected an array of field/value pairs');
    }
    for (let i = 0; i < result.length; i += 2) {
      obj[result[i]] = result[i + 1];
    }
    return obj;
  }

  async hdel(key: string, ...fields: string[]): Promise<number> {
    return this.sendCommand(['HDEL', key, ...fields]) as Promise<number>;
  }

  // List commands
  async lpush(key: string, ...values: (string | number | Buffer)[]): Promise<number> {
    return this.sendCommand(['LPUSH', key, ...values]) as Promise<number>;
  }

  async rpush(key: string, ...values: (string | number | Buffer)[]): Promise<number> {
    return this.sendCommand(['RPUSH', key, ...values]) as Promise<number>;
  }

  async lpop(key: string): Promise<string | null> {
    return this.sendCommand(['LPOP', key]) as Promise<string | null>;
  }

  async rpop(key: string): Promise<string | null> {
    return this.sendCommand(['RPOP', key]) as Promise<string | null>;
  }

  async lrange(key: string, start: number, stop: number): Promise<string[]> {
    return this.sendCommand(['LRANGE', key, start, stop]) as Promise<string[]>;
  }

  // Set commands
  async sadd(key: string, ...members: (string | number | Buffer)[]): Promise<number> {
    return this.sendCommand(['SADD', key, ...members]) as Promise<number>;
  }

  async srem(key: string, ...members: (string | number | Buffer)[]): Promise<number> {
    return this.sendCommand(['SREM', key, ...members]) as Promise<number>;
  }

  async smembers(key: string): Promise<string[]> {
    return this.sendCommand(['SMEMBERS', key]) as Promise<string[]>;
  }

  async sismember(key: string, member: string): Promise<number> {
    return this.sendCommand(['SISMEMBER', key, member]) as Promise<number>;
  }

  // Sorted set commands
  async zadd(key: string, score: number, member: string): Promise<number>;
  async zadd(key: string, members: Array<{ score: number; member: string }>): Promise<number>;
  async zadd(
    key: string,
    scoreOrMembers: number | Array<{ score: number; member: string }>,
    member?: string
  ): Promise<number> {
    const args: Command = ['ZADD', key];

    if (typeof scoreOrMembers === 'number') {
      args.push(scoreOrMembers, member!);
    } else {
      for (const { score, member } of scoreOrMembers) {
        args.push(score, member);
      }
    }

    return this.sendCommand(args) as Promise<number>;
  }

  async zrange(key: string, start: number, stop: number, withScores?: boolean): Promise<string[]> {
    const args: Command = ['ZRANGE', key, start, stop];
    if (withScores) args.push('WITHSCORES');
    return this.sendCommand(args) as Promise<string[]>;
  }

  async zrem(key: string, ...members: string[]): Promise<number> {
    return this.sendCommand(['ZREM', key, ...members]) as Promise<number>;
  }

  // CacheStorm-specific commands
  async invalidate(tag: string): Promise<number> {
    return this.sendCommand(['INVALIDATE', tag]) as Promise<number>;
  }

  async tagKeys(tag: string): Promise<string[]> {
    return this.sendCommand(['TAGKEYS', tag]) as Promise<string[]>;
  }

  async tags(key: string): Promise<string[]> {
    return this.sendCommand(['TAGS', key]) as Promise<string[]>;
  }

  // Pub/Sub
  subscribe(channels: string | string[], callback?: (message: Message) => void): PubSub {
    const chs = Array.isArray(channels) ? channels : [channels];
    return new PubSub(this, chs, callback);
  }

  psubscribe(patterns: string | string[], callback?: (message: Message) => void): PubSub {
    const pts = Array.isArray(patterns) ? patterns : [patterns];
    return new PubSub(this, [], callback, pts);
  }

  async publish(channel: string, message: string): Promise<number> {
    return this.sendCommand(['PUBLISH', channel, message]) as Promise<number>;
  }

  // Pipeline
  pipeline(): Pipeline {
    return new Pipeline(this);
  }

  sendCommands(commands: Command[], options: { returnBuffers?: boolean } = {}, retry = true): Promise<unknown>[] {
    if (!this.connected || !this.socket) {
      if (retry && this.wasConnected && this.options.reconnect && commands.length > 0) {
        const sent = this.reconnect().then(() => this.sendCommands(commands, options, false));
        return commands.map((_, i) => sent.then(promises => promises[i]));
      }
      return commands.map(() => Promise.reject(new Error('Not connected')));
    }
    if (commands.length === 0) return [];

    const promises = commands.map(command => this.enqueue(command, options.returnBuffers ?? false));
    this.socket.write(Buffer.concat(commands.map(command => this.encodeResp(command))));
    return promises;
  }

  private reconnect(): Promise<void> {
    if (!this.reconnecting) {
      this.reconnecting = (async () => {
        let lastError: unknown = new Error('Not connected');
        for (let attempt = 0; attempt < Math.max(1, this.options.reconnectAttempts); attempt++) {
          if (attempt > 0) {
            await new Promise(resolve => setTimeout(resolve, Math.min(100 * 2 ** (attempt - 1), 2000)));
          }
          if (!this.wasConnected) break;
          try {
            await this.connect();
            this.emit('reconnect');
            return;
          } catch (err) {
            lastError = err;
          }
        }
        throw lastError;
      })().finally(() => {
        this.reconnecting = null;
      });
    }
    return this.reconnecting;
  }

  private async sendCommand(command: Command): Promise<unknown> {
    return this.sendCommands([command])[0];
  }

  writeUnqueued(command: Command): void {
    if (!this.connected || !this.socket) {
      throw new Error('Not connected');
    }
    this.socket.write(this.encodeResp(command));
  }

  setPushHandler(handler: PushHandler | null): void {
    this.pushHandler = handler;
  }

  private enqueue(command: Command, returnBuffers: boolean): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const request: PendingRequest = { command, resolve, reject, returnBuffers };
      request.timer = setTimeout(() => {
        if (request.settled) return;
        request.settled = true;
        reject(new Error('Command timeout'));
      }, this.options.commandTimeout);
      this.pending.push(request);
    });
  }

  private encodeResp(command: Command): Buffer {
    return encodeCommand(command);
  }

  private handleResponse(data: Buffer): void {
    let replies: RespValue[];
    try {
      replies = this.decoder.feed(data);
    } catch (err) {
      this.decoder.reset();
      this.failPending(err as Error);
      this.socket?.destroy();
      return;
    }
    for (const reply of replies) {
      this.dispatch(reply);
    }
  }

  private dispatch(raw: RespValue): void {
    if (this.pushHandler) {
      const text = buffersToStrings(raw);
      if (Array.isArray(text) && typeof text[0] === 'string' && PUSH_KINDS.has(text[0].toLowerCase())) {
        this.pushHandler(text);
        return;
      }
      if (this.pending.length === 0) {
        this.pushHandler(text);
        return;
      }
    }
    const request = this.pending.shift();
    if (!request) {
      const err = new ProtocolError('Received a reply with no pending request');
      if (this.listenerCount('error') > 0) this.emit('error', err);
      this.socket?.destroy();
      return;
    }
    if (request.settled) return;
    request.settled = true;
    if (request.timer) clearTimeout(request.timer);
    const reply = request.returnBuffers ? raw : buffersToStrings(raw);
    if (reply instanceof ReplyError) {
      request.reject(reply);
    } else {
      request.resolve(reply);
    }
  }

  private failPending(err: Error): void {
    const requests = this.pending;
    this.pending = [];
    for (const request of requests) {
      if (request.settled) continue;
      request.settled = true;
      if (request.timer) clearTimeout(request.timer);
      request.reject(err);
    }
  }

  duplicate(overrides: CacheStormClientOptions = {}): CacheStormClient {
    return new CacheStormClient({ ...this.options, ...overrides });
  }
}
