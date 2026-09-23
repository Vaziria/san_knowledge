import { createSocket, type Socket } from 'node:dgram';
import { readFileSync } from 'node:fs';
import type { ServerResponse } from 'node:http';
import { join } from 'node:path';
import type { Connect, Plugin } from 'vite';

// Open Sound Control (OSC) for the preview. Browsers can't receive UDP, so the
// dev server receives OSC messages and passes each one on to the page as the
// Vite event 'osc:message', which src/osc.ts hands to whoever listens (a
// story). They come in two ways:
//
// - UDP on this machine only (127.0.0.1 and ::1), port 57121 or OSC_PORT.
//   From Sonic Pi on this machine: use_osc "localhost", 57121.
// - HTTP: POST /__osc with one OSC packet as application/octet-stream, from
//   anywhere that reaches the dev server, such as through ngrok or a VS Code
//   tunnel, which can't carry UDP. osc-relay.mjs, served at /__osc/relay.mjs,
//   takes Sonic Pi's UDP on another computer and posts it here.
//
// OSC 1.0 messages and bundles are read. A bundle's messages are passed on as
// it arrives; its time tag is ignored.

const MAX_POST = 64 * 1024; // bytes; an OSC packet is far smaller

export const DEFAULT_OSC_PORT = 57121;

// One message, as the page gets it; src/osc.ts has the same shape.
export interface OscMessage {
  address: string;
  args: OscArgument[];
}
export type OscArgument = number | string | boolean | null;

export function oscBridge(port = Number(process.env.OSC_PORT ?? DEFAULT_OSC_PORT)): Plugin {
  return {
    name: 'osc-bridge',
    apply: 'serve',
    configureServer(server) {
      const { logger } = server.config;
      const open = new Set<Socket>();
      const close = (socket: Socket) => {
        if (open.delete(socket)) socket.close();
      };
      const senders = new Set<string>();

      // Passes a packet's messages on to the page, or says why it can't.
      const deliver = (packet: Buffer, sender: string): string | null => {
        // Say once where messages come from, so a sender can be seen to work.
        if (!senders.has(sender)) {
          senders.add(sender);
          logger.info(`osc: messages from ${sender}`, { timestamp: true });
        }
        let messages: OscMessage[];
        try {
          messages = readOsc(packet);
        } catch (error) {
          const reason = (error as Error).message;
          logger.warn(`osc: dropped a packet from ${sender}: ${reason}`, { timestamp: true });
          return reason;
        }
        for (const message of messages) server.hot.send('osc:message', message);
        return null;
      };

      server.middlewares.use('/__osc', (req, res) => httpOsc(req, res, deliver, server.config.root));

      const listen = () => {
        for (const [type, host] of [['udp4', '127.0.0.1'], ['udp6', '::1']] as const) {
          const socket = createSocket(type);
          open.add(socket);
          socket.on('listening', () => logger.info(`  osc: listening on udp://${host}:${port}`));
          socket.on('error', (error) => {
            logger.warn(`osc: not listening on ${host} port ${port}: ${error.message}`, { timestamp: true });
            close(socket);
          });
          socket.on('message', (packet, from) => deliver(packet, `udp ${from.address}:${from.port}`));
          socket.bind(port, host);
        }
      };

      // When a config change restarts the dev server, Vite starts the new
      // server before it closes the old one, so the port is still taken here.
      // Listen once this server listens, after the old one has let go.
      if (!server.httpServer) return listen();
      server.httpServer.once('listening', listen);
      server.httpServer.once('close', () => {
        for (const socket of [...open]) close(socket);
      });
    },
  };
}

// /__osc: POST one OSC packet (application/octet-stream) to pass it on,
// answered 204, or 400 with the reason when it can't be read. GET
// /__osc/relay.mjs downloads the relay for another computer.
function httpOsc(
  req: Connect.IncomingMessage,
  res: ServerResponse,
  deliver: (packet: Buffer, sender: string) => string | null,
  root: string,
): void {
  const answer = (status: number, text = '') => {
    res.statusCode = status;
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');
    res.end(text && `${text}\n`);
  };
  if (req.method === 'GET' && req.url === '/relay.mjs') {
    res.setHeader('Content-Type', 'text/javascript; charset=utf-8');
    res.end(readFileSync(join(root, 'osc-relay.mjs')));
    return;
  }
  if (req.url !== '/' && req.url !== '') return answer(404);
  if (req.method !== 'POST') return answer(405, 'POST an OSC packet');
  if (req.headers['content-type'] !== 'application/octet-stream') {
    return answer(415, 'send the OSC packet as application/octet-stream');
  }

  const chunks: Buffer[] = [];
  let size = 0;
  req.on('data', (chunk: Buffer) => {
    size += chunk.length;
    if (size <= MAX_POST) chunks.push(chunk);
  });
  req.on('end', () => {
    if (size > MAX_POST) return answer(413, `an OSC packet is at most ${MAX_POST} bytes here`);
    // Behind a tunnel the connection comes from the tunnel's agent; the
    // sender is in X-Forwarded-For.
    const from = req.headers['x-forwarded-for'] ?? req.socket.remoteAddress ?? 'unknown';
    const problem = deliver(Buffer.concat(chunks), `http ${from}`);
    if (problem) answer(400, problem);
    else answer(204);
  });
}

// Writes one OSC message: strings as s, whole numbers as i (f outside 32
// bits), other numbers as f, booleans as T or F and null as N.
export function writeOsc({ address, args }: OscMessage): Buffer {
  const string = (text: string) => {
    const bytes = Buffer.from(`${text}\0`, 'utf8');
    return Buffer.concat([bytes, Buffer.alloc((4 - (bytes.length % 4)) % 4)]);
  };
  let tags = ',';
  const data: Buffer[] = [];
  for (const arg of args) {
    if (typeof arg === 'string') {
      tags += 's';
      data.push(string(arg));
    } else if (typeof arg === 'number') {
      const whole = Number.isInteger(arg) && arg >= -(2 ** 31) && arg < 2 ** 31;
      tags += whole ? 'i' : 'f';
      const bytes = Buffer.alloc(4);
      if (whole) bytes.writeInt32BE(arg);
      else bytes.writeFloatBE(arg);
      data.push(bytes);
    } else {
      tags += arg === null ? 'N' : arg ? 'T' : 'F';
    }
  }
  return Buffer.concat([string(address), string(tags), ...data]);
}

// Reads one OSC packet: a message, or a bundle of messages and bundles.
export function readOsc(packet: Buffer): OscMessage[] {
  if (packet.subarray(0, 8).toString('latin1') !== '#bundle\0') return [readMessage(packet)];

  // '#bundle', an 8-byte time tag, then each element as its size and bytes.
  const messages: OscMessage[] = [];
  let offset = 16;
  while (offset < packet.length) {
    if (offset + 4 > packet.length) throw new Error('bundle ends early');
    const size = packet.readInt32BE(offset);
    offset += 4;
    if (size < 0 || offset + size > packet.length) throw new Error('bundle element runs past the end');
    messages.push(...readOsc(packet.subarray(offset, offset + size)));
    offset += size;
  }
  return messages;
}

function readMessage(packet: Buffer): OscMessage {
  let offset = 0;
  const need = (bytes: number) => {
    if (offset + bytes > packet.length) throw new Error('message ends early');
  };
  // A string ends with a null and is padded with nulls to a multiple of 4 bytes.
  const string = () => {
    const end = packet.indexOf(0, offset);
    if (end < 0) throw new Error('string without its ending null');
    const text = packet.toString('utf8', offset, end);
    offset = (end + 4) & ~3;
    return text;
  };
  const int32 = () => {
    need(4);
    offset += 4;
    return packet.readInt32BE(offset - 4);
  };
  const float32 = () => {
    need(4);
    offset += 4;
    return packet.readFloatBE(offset - 4);
  };
  const float64 = () => {
    need(8);
    offset += 8;
    return packet.readDoubleBE(offset - 8);
  };
  const int64 = () => {
    need(8);
    offset += 8;
    return Number(packet.readBigInt64BE(offset - 8));
  };

  const address = string();
  if (!address.startsWith('/')) throw new Error(`"${address}" is not an OSC address`);
  // Old senders may leave out the type tags; then there are no arguments.
  const tags = offset < packet.length ? string() : ',';
  if (!tags.startsWith(',')) throw new Error(`"${tags}" are not OSC type tags`);

  const args: OscArgument[] = [];
  for (const tag of tags.slice(1)) {
    switch (tag) {
      case 'i':
        args.push(int32());
        break;
      case 'f':
        args.push(float32());
        break;
      case 'd':
        args.push(float64());
        break;
      case 'h':
        args.push(int64());
        break;
      case 's':
      case 'S':
        args.push(string());
        break;
      case 'c':
        args.push(String.fromCharCode(int32()));
        break;
      case 'r': // an RGBA colour
      case 'm': // four MIDI bytes
        args.push(int32());
        break;
      case 't': // a time tag
        int64();
        args.push(null);
        break;
      case 'b': {
        // A blob: its size, then its bytes padded to a multiple of 4. Skipped.
        const size = int32();
        need(size);
        offset += (size + 3) & ~3;
        args.push(null);
        break;
      }
      case 'T':
        args.push(true);
        break;
      case 'F':
        args.push(false);
        break;
      case 'N':
        args.push(null);
        break;
      case 'I': // an impulse, a "bang"
        args.push(true);
        break;
      case '[':
      case ']': // an array's items are read as plain arguments
        break;
      default:
        throw new Error(`unknown OSC type tag "${tag}"`);
    }
  }
  return { address, args };
}
