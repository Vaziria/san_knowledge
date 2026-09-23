// OSC relay: run it on the computer Sonic Pi runs on, when the preview's dev
// server is on another machine that UDP can't reach (behind ngrok or a VS Code
// tunnel). It receives OSC on this computer, on UDP port 57121, and posts
// each packet to the dev server's /__osc, which passes it on to the page.
// Needs Node 18 or newer; no packages.
//
//   node osc-relay.mjs https://your-tunnel.ngrok-free.app [port]
//
// In Sonic Pi:
//
//   use_osc "localhost", 57121
//   osc "/piano/chord", "Am"
//
// Packets are posted one request at a time, in the order they came; any that
// arrive while a request is on its way go together in the next one, as an
// OSC bundle, so a slow tunnel never reorders them.

import { createSocket } from 'node:dgram';

const [url, portArg] = process.argv.slice(2);
if (!url || !/^https?:\/\//.test(url)) {
  console.error('usage: node osc-relay.mjs https://your-tunnel.ngrok-free.app [port]');
  process.exit(1);
}
const endpoint = url.replace(/\/+$/, '') + '/__osc';
const port = Number(portArg ?? 57121);

// One bundle of packets: '#bundle', an 8-byte time tag meaning "now", then
// each packet's size and bytes.
function bundle(packets) {
  const now = Buffer.alloc(8);
  now.writeUInt32BE(1, 4);
  const parts = [Buffer.from('#bundle\0'), now];
  for (const packet of packets) {
    const size = Buffer.alloc(4);
    size.writeInt32BE(packet.length);
    parts.push(size, packet);
  }
  return Buffer.concat(parts);
}

const queue = [];
let posting = false;
let sent = 0;

async function flush() {
  if (posting || queue.length === 0) return;
  posting = true;
  const packets = queue.splice(0);
  try {
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream', 'ngrok-skip-browser-warning': '1' },
      body: packets.length === 1 ? packets[0] : bundle(packets),
    });
    if (response.ok) {
      if (sent === 0) console.log(`first message passed on to ${endpoint}`);
      sent += packets.length;
    } else {
      console.error(`${endpoint} answered ${response.status}: ${(await response.text()).trim()}`);
    }
  } catch (error) {
    console.error(`can't reach ${endpoint}: ${error.cause?.message ?? error.message}`);
  }
  posting = false;
  void flush();
}

// Listen on both loopback addresses, since "localhost" may mean either.
for (const [type, host] of [['udp4', '127.0.0.1'], ['udp6', '::1']]) {
  const socket = createSocket(type);
  socket.on('message', (packet) => {
    queue.push(packet);
    void flush();
  });
  socket.on('error', (error) => {
    console.error(`not listening on ${host} port ${port}: ${error.message}`);
    socket.close();
  });
  socket.on('listening', () => console.log(`listening on udp://${host}:${port}`));
  socket.bind(port, host);
}
console.log(`passing OSC on to ${endpoint}; Ctrl+C stops`);
