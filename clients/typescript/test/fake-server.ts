import * as net from 'node:net';
import { AddressInfo } from 'node:net';
import { RespDecoder } from '../src/resp';

export interface FakeServer {
  port: number;
  commands: string[][];
  close(): Promise<void>;
}

function bulk(value: string | null): string {
  if (value === null) return '$-1\r\n';
  return `$${Buffer.byteLength(value)}\r\n${value}\r\n`;
}

function push(parts: (string | number | null)[]): string {
  return `*${parts.length}\r\n` + parts.map(p => (typeof p === 'number' ? `:${p}\r\n` : bulk(p))).join('');
}

function globMatch(pattern: string, text: string): boolean {
  const re = new RegExp('^' + pattern.split('*').map(s => s.replace(/[.+?^${}()|[\]\\]/g, '\\$&')).join('.*') + '$');
  return re.test(text);
}

export async function startFakeServer(options: { password?: string; binary?: Buffer } = {}): Promise<FakeServer> {
  const commands: string[][] = [];
  const subscribers = new Set<{ socket: net.Socket; channels: Set<string>; patterns: Set<string> }>();
  const sockets = new Set<net.Socket>();
  const server = net.createServer(socket => {
    sockets.add(socket);
    socket.setNoDelay(true);
    socket.on('close', () => sockets.delete(socket));
    const decoder = new RespDecoder();
    const state = { socket, channels: new Set<string>(), patterns: new Set<string>() };
    let authed = !options.password;
    const count = () => state.channels.size + state.patterns.size;
    socket.on('data', data => {
      for (const raw of decoder.feed(data)) {
        const args = (raw as string[]).map(String);
        commands.push(args);
        const name = args[0].toUpperCase();
        if (name === 'AUTH') {
          if (args[1] === options.password) {
            authed = true;
            socket.write('+OK\r\n');
          } else {
            socket.write('-WRONGPASS invalid username-password pair\r\n');
          }
          continue;
        }
        if (!authed) {
          socket.write('-NOAUTH Authentication required.\r\n');
          continue;
        }
        switch (name) {
          case 'SELECT':
          case 'SET':
            socket.write('+OK\r\n');
            break;
          case 'GET':
            if (args[1] === 'bin' && options.binary) {
              socket.write(Buffer.concat([Buffer.from(`$${options.binary.length}\r\n`), options.binary, Buffer.from('\r\n')]));
            } else {
              socket.write(bulk(null));
            }
            break;
          case 'QUIT':
            socket.end('+OK\r\n');
            break;
          case 'SUBSCRIBE':
          case 'PSUBSCRIBE': {
            subscribers.add(state);
            const set = name === 'SUBSCRIBE' ? state.channels : state.patterns;
            for (const ch of args.slice(1)) {
              set.add(ch);
              socket.write(push([name.toLowerCase(), ch, count()]));
            }
            break;
          }
          case 'UNSUBSCRIBE':
          case 'PUNSUBSCRIBE': {
            const set = name === 'UNSUBSCRIBE' ? state.channels : state.patterns;
            const targets = args.length > 1 ? args.slice(1) : [...set];
            for (const ch of targets) {
              set.delete(ch);
              socket.write(push([name.toLowerCase(), ch, count()]));
            }
            break;
          }
          case 'PUBLISH': {
            let receivers = 0;
            for (const sub of subscribers) {
              if (sub.channels.has(args[1])) {
                receivers++;
                sub.socket.write(push(['message', args[1], args[2]]));
              }
              for (const pattern of sub.patterns) {
                if (globMatch(pattern, args[1])) {
                  receivers++;
                  sub.socket.write(push(['pmessage', pattern, args[1], args[2]]));
                }
              }
            }
            socket.write(`:${receivers}\r\n`);
            break;
          }
          default:
            socket.write(`-ERR unknown command '${args[0]}'\r\n`);
        }
      }
    });
    socket.on('close', () => subscribers.delete(state));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  return {
    port: (server.address() as AddressInfo).port,
    commands,
    close: () =>
      new Promise<void>(resolve => {
        for (const socket of sockets) socket.destroy();
        server.close(() => resolve());
      }),
  };
}

