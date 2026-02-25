import { EventEmitter } from 'events';
import * as net from 'net';
import {
  CacheStormClientOptions,
  SetOptions,
  GetOptions,
  Message,
  Command,
} from './types';
import { Pipeline } from './pipeline';
import { PubSub } from './pubsub';

export class CacheStormClient extends EventEmitter {
  private socket: net.Socket | null = null;
  private options: Required<CacheStormClientOptions>;
  private connected = false;
  private commandQueue: Array<{ command: Command; resolve: Function; reject: Function }> = [];
  private currentCommand: { command: Command; resolve: Function; reject: Function } | null = null;

  constructor(options: CacheStormClientOptions = {}) {
    super();
    this.options = {
      host: options.host ?? 'localhost',
      port: options.port ?? 6379,
      password: options.password ?? '',
      database: options.database ?? 0,
      connectTimeout: options.connectTimeout ?? 10000,
      commandTimeout: options.commandTimeout ?? 5000,
      pool: options.pool ?? {},
      retry: options.retry ?? {},
    };
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Connection timeout'));
      }, this.options.connectTimeout);

      this.socket = net.createConnection({
        host: this.options.host,
        port: this.options.port,
      });

      this.socket.on('connect', () => {
        clearTimeout(timeout);
        this.connected = true;
        this.emit('connect');
        resolve();
      });

      this.socket.on('error', (err) => {
        clearTimeout(timeout);
        this.emit('error', err);
        reject(err);
      });

      this.socket.on('data', (data) => {
        this.handleResponse(data);
      });

      this.socket.on('close', () => {
        this.connected = false;
        this.emit('close');
      });
    });
  }

  async quit(): Promise<void> {
    if (!this.connected) return;
    await this.sendCommand(['QUIT']);
    this.socket?.end();
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
    if (options?.TAGS) {
      args.push('TAGS');
      args.push(...options.TAGS);
    }

    return this.sendCommand(args) as Promise<string>;
  }

  async setWithTags(key: string, value: string | number | Buffer, tags: string[]): Promise<string> {
    return this.set(key, value, { TAGS: tags });
  }

  async get(key: string, options?: GetOptions): Promise<string | null> {
    return this.sendCommand(['GET', key]) as Promise<string | null>;
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
    const result = await this.sendCommand(['HGETALL', key]) as string[];
    const obj: Record<string, string> = {};
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

  async publish(channel: string, message: string): Promise<number> {
    return this.sendCommand(['PUBLISH', channel, message]) as Promise<number>;
  }

  // Pipeline
  pipeline(): Pipeline {
    return new Pipeline(this);
  }

  // Internal methods
  private async sendCommand(command: Command): Promise<unknown> {
    if (!this.connected) {
      throw new Error('Not connected');
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Command timeout'));
      }, this.options.commandTimeout);

      this.commandQueue.push({
        command,
        resolve: (result: unknown) => {
          clearTimeout(timeout);
          resolve(result);
        },
        reject: (err: Error) => {
          clearTimeout(timeout);
          reject(err);
        },
      });

      this.processQueue();
    });
  }

  private processQueue(): void {
    if (this.currentCommand || this.commandQueue.length === 0) {
      return;
    }

    this.currentCommand = this.commandQueue.shift()!;
    this.writeCommand(this.currentCommand.command);
  }

  private writeCommand(command: Command): void {
    if (!this.socket) return;

    // Simple RESP encoding (simplified)
    const encoded = this.encodeResp(command);
    this.socket.write(encoded);
  }

  private encodeResp(command: Command): Buffer {
    // Simplified RESP encoding
    const parts = command.map(arg => {
      const str = arg.toString();
      return `$${str.length}\r\n${str}\r\n`;
    });

    return Buffer.from(`*${command.length}\r\n${parts.join('')}`);
  }

  private handleResponse(data: Buffer): void {
    // Simplified response handling
    const response = data.toString().trim();

    if (this.currentCommand) {
      const { resolve } = this.currentCommand;
      this.currentCommand = null;

      if (response.startsWith('+')) {
        resolve(response.slice(1));
      } else if (response.startsWith('-')) {
        // Error
      } else if (response.startsWith('$')) {
        // Bulk string
        resolve(response.split('\r\n')[1] ?? null);
      } else {
        resolve(response);
      }

      this.processQueue();
    }
  }

  duplicate(): CacheStormClient {
    return new CacheStormClient(this.options);
  }
}
