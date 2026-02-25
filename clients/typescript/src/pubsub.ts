import { EventEmitter } from 'events';
import { CacheStormClient } from './client';
import { Message } from './types';

export class PubSub extends EventEmitter {
  private client: CacheStormClient;
  private channels: string[];
  private subscribed = false;

  constructor(
    client: CacheStormClient,
    channels: string[],
    private callback?: (message: Message) => void
  ) {
    super();
    this.client = client;
    this.channels = channels;

    if (callback) {
      this.on('message', callback);
    }
  }

  async subscribe(): Promise<void> {
    if (this.subscribed) return;

    for (const channel of this.channels) {
      // Send SUBSCRIBE command
    }

    this.subscribed = true;
  }

  async unsubscribe(...channels: string[]): Promise<void> {
    const chs = channels.length > 0 ? channels : this.channels;

    for (const channel of chs) {
      // Send UNSUBSCRIBE command
    }

    if (channels.length === 0) {
      this.subscribed = false;
    }
  }

  async quit(): Promise<void> {
    await this.unsubscribe();
    this.removeAllListeners();
  }

  onMessage(callback: (message: Message) => void): void {
    this.on('message', callback);
  }

  getChannels(): string[] {
    return [...this.channels];
  }
}
