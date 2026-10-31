export interface CacheStormClientOptions {
  host?: string;
  port?: number;
  password?: string;
  database?: number;
  connectTimeout?: number;
  commandTimeout?: number;
  /**
   * Reconnect when a command is sent after the connection dropped. AUTH and
   * SELECT run again; commands that were in flight when it dropped are not
   * resent. Default true.
   */
  reconnect?: boolean;
  /** Connection attempts per reconnect, with exponential backoff. Default 3. */
  reconnectAttempts?: number;
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
