export interface CacheStormClientOptions {
  host?: string;
  port?: number;
  password?: string;
  database?: number;
  connectTimeout?: number;
  commandTimeout?: number;
}

export interface SetOptions {
  EX?: number;
  PX?: number;
  NX?: boolean;
  XX?: boolean;
  TAGS?: string[];
}

export interface Message {
  channel: string;
  pattern?: string;
  payload: string;
}

export type Command = (string | number | Buffer)[];
export type Callback<T = unknown> = (err: Error | null, result?: T) => void;
