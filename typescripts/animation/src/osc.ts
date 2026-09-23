import type {} from 'vite/types/customEvent.d.ts';

// OSC (Open Sound Control) messages for the page. The dev server receives
// them over UDP and passes each one on as the event 'osc:message'
// (osc-bridge.ts). A story listens with onOsc(), which returns the function
// that stops it. Only the dev server (npm run dev) has the bridge; a built
// page gets no messages.

export type OscArgument = number | string | boolean | null;
export interface OscMessage {
  address: string;
  args: OscArgument[];
}

declare module 'vite/types/customEvent.d.ts' {
  interface CustomEventMap {
    'osc:message': OscMessage;
  }
}

const listeners = new Set<(message: OscMessage) => void>();

import.meta.hot?.on('osc:message', (message) => {
  for (const listener of listeners) listener(message);
});

export function onOsc(listener: (message: OscMessage) => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
