import { EventEmitter } from 'events';
import { CacheStormClient } from './client';
import { Message } from './types';
import { ReplyError, RespValue } from './resp';

type ConfirmKind = 'subscribe' | 'unsubscribe' | 'psubscribe' | 'punsubscribe';

interface Waiter {
  resolve: () => void;
  reject: (err: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

export class PubSub extends EventEmitter {
  private client: CacheStormClient;
  private channels: string[];
  private patterns: string[];
  private connection: CacheStormClient | null = null;
  private connecting: Promise<CacheStormClient> | null = null;
  private activeChannels = new Set<string>();
  private activePatterns = new Set<string>();
  private waiters = new Map<string, Waiter[]>();

  constructor(
    client: CacheStormClient,
    channels: string[],
    private callback?: (message: Message) => void,
    patterns: string[] = []
  ) {
    super();
    this.client = client;
    this.channels = channels;
    this.patterns = patterns;

    if (callback) {
      this.on('message', callback);
      this.on('pmessage', callback);
    }
  }

  get subscribed(): boolean {
    return this.activeChannels.size > 0 || this.activePatterns.size > 0;
  }

  async subscribe(...channels: string[]): Promise<void> {
    const targets = channels.length > 0 ? channels : this.channels;
    for (const channel of targets) {
      if (!this.channels.includes(channel)) this.channels.push(channel);
    }
    if (targets.length > 0) await this.send('subscribe', targets);
    if (channels.length === 0 && this.patterns.length > 0) await this.send('psubscribe', this.patterns);
  }

  async psubscribe(...patterns: string[]): Promise<void> {
    for (const pattern of patterns) {
      if (!this.patterns.includes(pattern)) this.patterns.push(pattern);
    }
    if (patterns.length > 0) await this.send('psubscribe', patterns);
  }

  async unsubscribe(...channels: string[]): Promise<void> {
    const targets = channels.length > 0 ? channels : [...this.activeChannels];
    this.channels = channels.length > 0 ? this.channels.filter(c => !channels.includes(c)) : [];
    if (targets.length > 0 && this.connection) await this.send('unsubscribe', targets);
  }

  async punsubscribe(...patterns: string[]): Promise<void> {
    const targets = patterns.length > 0 ? patterns : [...this.activePatterns];
    this.patterns = patterns.length > 0 ? this.patterns.filter(p => !patterns.includes(p)) : [];
    if (targets.length > 0 && this.connection) await this.send('punsubscribe', targets);
  }

  async quit(): Promise<void> {
    const connection = this.connection;
    if (connection) {
      try {
        await this.unsubscribe();
        await this.punsubscribe();
        await connection.quit();
      } finally {
        this.teardown(new Error('PubSub closed'));
      }
    }
    this.removeAllListeners();
  }

  onMessage(callback: (message: Message) => void): void {
    this.on('message', callback);
    this.on('pmessage', callback);
  }

  getChannels(): string[] {
    return [...this.channels];
  }

  getPatterns(): string[] {
    return [...this.patterns];
  }

  private async send(kind: ConfirmKind, names: string[]): Promise<void> {
    const connection = await this.connect();
    const confirmations = names.map(name => this.expect(kind, name, connection.commandTimeout));
    try {
      connection.writeUnqueued([kind.toUpperCase(), ...names]);
    } catch (err) {
      this.rejectAll(err as Error);
    }
    await Promise.all(confirmations);
  }

  private connect(): Promise<CacheStormClient> {
    if (this.connection) return Promise.resolve(this.connection);
    if (!this.connecting) {
      // A dropped subscription connection is reported, not silently
      // replaced by one that has no subscriptions.
      const connection = this.client.duplicate({ reconnect: false });
      connection.setPushHandler(reply => this.onPush(reply));
      connection.on('error', err => {
        if (this.listenerCount('error') > 0) this.emit('error', err);
      });
      connection.on('close', () => {
        if (this.connection === connection) this.teardown(new Error('Connection closed'));
      });
      this.connecting = connection.connect().then(
        () => {
          this.connection = connection;
          this.connecting = null;
          return connection;
        },
        err => {
          this.connecting = null;
          throw err;
        }
      );
    }
    return this.connecting;
  }

  private expect(kind: ConfirmKind, name: string, timeoutMs: number): Promise<void> {
    return new Promise((resolve, reject) => {
      const key = `${kind}:${name}`;
      const waiter: Waiter = {
        resolve,
        reject,
        timer: setTimeout(() => {
          const list = this.waiters.get(key);
          if (list) {
            const index = list.indexOf(waiter);
            if (index >= 0) list.splice(index, 1);
          }
          reject(new Error(`${kind.toUpperCase()} ${name} timed out`));
        }, timeoutMs),
      };
      const list = this.waiters.get(key) ?? [];
      list.push(waiter);
      this.waiters.set(key, list);
    });
  }

  private confirm(kind: ConfirmKind, name: string): void {
    const list = this.waiters.get(`${kind}:${name}`);
    const waiter = list?.shift();
    if (!waiter) return;
    clearTimeout(waiter.timer);
    waiter.resolve();
  }

  private rejectAll(err: Error): void {
    const lists = [...this.waiters.values()];
    this.waiters.clear();
    for (const list of lists) {
      for (const waiter of list) {
        clearTimeout(waiter.timer);
        waiter.reject(err);
      }
    }
  }

  private teardown(err: Error): void {
    this.connection = null;
    this.activeChannels.clear();
    this.activePatterns.clear();
    this.rejectAll(err);
  }

  private onPush(reply: RespValue): void {
    if (reply instanceof ReplyError) {
      this.rejectAll(reply);
      if (this.listenerCount('error') > 0) this.emit('error', reply);
      return;
    }
    if (!Array.isArray(reply) || typeof reply[0] !== 'string') return;
    const kind = reply[0].toLowerCase();
    const name = reply[1] == null ? '' : String(reply[1]);

    switch (kind) {
      case 'message':
        this.emit('message', { channel: name, payload: String(reply[2] ?? '') } as Message);
        return;
      case 'pmessage':
        this.emit('pmessage', {
          pattern: name,
          channel: String(reply[2] ?? ''),
          payload: String(reply[3] ?? ''),
        } as Message);
        return;
      case 'subscribe':
        this.activeChannels.add(name);
        break;
      case 'unsubscribe':
        this.activeChannels.delete(name);
        break;
      case 'psubscribe':
        this.activePatterns.add(name);
        break;
      case 'punsubscribe':
        this.activePatterns.delete(name);
        break;
      default:
        return;
    }
    this.confirm(kind as ConfirmKind, name);
    this.emit(kind, name, reply[2]);
  }
}
