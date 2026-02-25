export interface CacheStormClientOptions {
  host?: string;
  port?: number;
  password?: string;
  database?: number;
  connectTimeout?: number;
  commandTimeout?: number;
  pool?: PoolOptions;
  retry?: RetryOptions;
}

export interface PoolOptions {
  min?: number;
  max?: number;
  acquireTimeout?: number;
  idleTimeout?: number;
}

export interface RetryOptions {
  maxRetries?: number;
  retryDelay?: number;
  retryDelayFactor?: number;
}

export interface SetOptions {
  EX?: number; // Expiration in seconds
  PX?: number; // Expiration in milliseconds
  NX?: boolean; // Only set if not exists
  XX?: boolean; // Only set if exists
  TAGS?: string[]; // CacheStorm tags
  namespace?: string; // CacheStorm namespace
}

export interface GetOptions {
  namespace?: string;
}

export interface Message {
  channel: string;
  pattern?: string;
  payload: string;
}

export type Command = (string | number | Buffer)[];
export type Callback<T = unknown> = (err: Error | null, result?: T) => void;
