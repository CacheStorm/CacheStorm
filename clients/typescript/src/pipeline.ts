import { CacheStormClient, setCommand } from './client';
import { Command, SetOptions } from './types';

export class Pipeline {
  private client: CacheStormClient;
  private commands: Command[] = [];

  constructor(client: CacheStormClient) {
    this.client = client;
  }

  set(key: string, value: string | number | Buffer, options?: SetOptions): this {
    this.commands.push(setCommand(key, value, options));
    return this;
  }

  setWithTags(key: string, value: string | number | Buffer, tags: string[]): this {
    return this.set(key, value, { TAGS: tags });
  }

  /** Queues an INVALIDATE of the tags; its result is how many keys it deleted. */
  invalidate(...tags: string[]): this {
    this.commands.push(['INVALIDATE', ...tags]);
    return this;
  }

  invalidateCascade(...tags: string[]): this {
    this.commands.push(['INVALIDATE', ...tags, 'CASCADE']);
    return this;
  }

  addTags(key: string, ...tags: string[]): this {
    this.commands.push(['ADDTAG', key, ...tags]);
    return this;
  }

  removeTags(key: string, ...tags: string[]): this {
    this.commands.push(['REMTAG', key, ...tags]);
    return this;
  }

  expire(key: string, seconds: number): this {
    this.commands.push(['EXPIRE', key, seconds]);
    return this;
  }

  incr(key: string): this {
    this.commands.push(['INCR', key]);
    return this;
  }

  /** Queues any command, for those Pipeline has no method for. */
  command(...args: Command): this {
    if (args.length === 0) throw new Error('command needs a command name');
    this.commands.push(args);
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
    const commands = this.commands;
    this.commands = [];
    return Promise.allSettled(this.client.sendCommands(commands)).then(outcomes => {
      const results = outcomes.map(outcome =>
        outcome.status === 'fulfilled' ? outcome.value : outcome.reason
      );
      const failure = outcomes.find(outcome => outcome.status === 'rejected');
      if (failure) {
        throw new PipelineError((failure as PromiseRejectedResult).reason, results);
      }
      return results;
    });
  }
}

export class PipelineError extends Error {
  readonly cause: unknown;
  readonly results: unknown[];

  constructor(cause: unknown, results: unknown[]) {
    super(cause instanceof Error ? cause.message : String(cause));
    this.name = 'PipelineError';
    this.cause = cause;
    this.results = results;
  }
}
