import { createSocket } from 'node:dgram';
import { DEFAULT_OSC_PORT, writeOsc, type OscArgument, type OscMessage } from './osc-bridge.ts';

// Sends OSC messages to the dev server's bridge (osc-bridge.ts), to play a
// story without Sonic Pi. Run it on the machine the dev server runs on; with
// VS Code Remote Tunnels, that is VS Code's terminal, since UDP doesn't go
// through the tunnel.
//
//   npm run osc -- /piano/chord Am    one message; numbers are sent as numbers
//   npm run osc -- /piano/note 76
//   npm run osc:demo                  plays C G Am F with a melody, until Ctrl+C
//
// It sends to 127.0.0.1 on port 57121, or OSC_PORT.

const port = Number(process.env.OSC_PORT ?? DEFAULT_OSC_PORT);
const socket = createSocket('udp4');

function send(message: OscMessage): Promise<void> {
  return new Promise((resolve, reject) =>
    socket.send(writeOsc(message), port, '127.0.0.1', (error) => (error ? reject(error) : resolve())),
  );
}

// The same song as the MIDI piano's own demo: a chord each bar, four notes over it.
const SONG: [chord: string, melody: string[]][] = [
  ['C', ['G5', 'E5', 'C5', 'E5']],
  ['G', ['B5', 'G5', 'D5', 'G5']],
  ['Am', ['A5', 'E5', 'C5', 'E5']],
  ['F', ['A5', 'F5', 'C5', 'F5']],
];
const BEAT = 300; // milliseconds a note

function demo(): void {
  console.log(`playing to udp://127.0.0.1:${port}; Ctrl+C stops`);
  let step = 0;
  const timer = setInterval(() => {
    const [chord, melody] = SONG[Math.floor(step / 4) % SONG.length];
    if (step % 4 === 0) void send({ address: '/piano/chord', args: [chord] });
    void send({ address: '/piano/note', args: [melody[step % 4]] });
    step++;
  }, BEAT);
  process.once('SIGINT', () => {
    clearInterval(timer);
    void send({ address: '/piano/release', args: [] }).finally(() => socket.close());
  });
}

const [address, ...values] = process.argv.slice(2);
if (address === '--demo') {
  demo();
} else if (!address?.startsWith('/')) {
  console.error('usage: npm run osc -- /address [arguments...]   or   npm run osc:demo');
  process.exitCode = 1;
  socket.close();
} else {
  const args: OscArgument[] = values.map((value) => (/^-?\d+(\.\d+)?$/.test(value) ? Number(value) : value));
  await send({ address, args });
  console.log(`sent ${address} ${values.join(' ')} to udp://127.0.0.1:${port}`);
  socket.close();
}
