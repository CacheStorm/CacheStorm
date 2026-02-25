import { CacheStormClient } from './client';
import { Command } from './types';

export class Pipeline {
  private client: CacheStormClient;
  private commands: Command[] = [];

  constructor(client: CacheStormClient) {
    this.client = client;
  }

  set(key: string, value: string | number | Buffer): this {
    this.commands.push(['SET', key, value]);
    return this;
  }

  get(key: string): this {
    this.commands.push(['GET', key]);
    return this;
  }

  del(...keys: string[]): this {
    this.commands.push(['DEL', ...keys]);
    return this;
  }

  hset(key: string, field: string, value: string | number | Buffer): this;
  hset(key: string, fields: Record<string, string | number | Buffer>): this;
  hset(
    key: string,
    fieldOrFields: string | Record<string, string | number | Buffer>,
    value?: string | number | Buffer
  ): this {
    const args: Command = ['HSET', key];

    if (typeof fieldOrFields === 'string') {
      args.push(fieldOrFields, value!);
    } else {
      for (const [field, val] of Object.entries(fieldOrFields)) {
        args.push(field, val);
      }
    }

    this.commands.push(args);
    return this;
  }

  hget(key: string, field: string): this {
    this.commands.push(['HGET', key, field]);
    return this;
  }

  lpush(key: string, ...values: (string | number | Buffer)[]): this {
    this.commands.push(['LPUSH', key, ...values]);
    return this;
  }

  rpush(key: string, ...values: (string | number | Buffer)[]): this {
    this.commands.push(['RPUSH', key, ...values]);
    return this;
  }

  lpop(key: string): this {
    this.commands.push(['LPOP', key]);
    return this;
  }

  rpop(key: string): this {
    this.commands.push(['RPOP', key]);
    return this;
  }

  sadd(key: string, ...members: (string | number | Buffer)[]): this {
    this.commands.push(['SADD', key, ...members]);
    return this;
  }

  zadd(key: string, score: number, member: string): this {
    this.commands.push(['ZADD', key, score, member]);
    return this;
  }

  exec(): Promise<unknown[]> {
    // Execute all commands in pipeline
    // This is a simplified implementation
    return Promise.all(
      this.commands.map(cmd => {
        // In real implementation, send all at once
        return Promise.resolve('OK');
      })
    );
  }
}
