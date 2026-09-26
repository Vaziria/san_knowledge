import type {} from 'vite/types/customEvent.d.ts';
import { behaviours } from './behaviours';
import { settings, settingsFromQuery } from './settings';
import type { Stage } from './Stage';
import { record, type Follow } from './stream';
import { holdOnly } from './walk';

// The page as the dev server's hidden browser opens it to draw the stream
// (the Stream tab's "stream only"; stream-bridge.ts): ?renderer=1280x720 and
// the panel's settings. No panel: it draws the scene at the stream's size,
// records it for the dev server (stream.ts), and follows the panel, whose
// settings, behaviours and walking come as the Vite event 'stream:follow'. A small
// picture of it goes to the dev server every second, for the panel, and its
// errors go to the dev server's terminal, since nobody sees this page.

declare module 'vite/types/customEvent.d.ts' {
  interface CustomEventMap {
    'stream:follow': Follow;
  }
}

const PREVIEW_MS = 1000;
const PREVIEW_WIDTH = 960; // px
const PREVIEW_QUALITY = 0.7; // JPEG

// The size to draw at when this page is the stream's renderer, or null.
export function rendererSize(): { width: number; height: number } | null {
  const size = /^(\d{2,4})x(\d{2,4})$/.exec(new URLSearchParams(location.search).get('renderer') ?? '');
  return size ? { width: Number(size[1]), height: Number(size[2]) } : null;
}

export function drawForStream(stage: Stage): void {
  window.addEventListener('error', (event) => log(event.message));
  window.addEventListener('unhandledrejection', (event) => log(`unhandled: ${String(event.reason)}`));

  import.meta.hot?.on('stream:follow', (change) => {
    if (change.scene !== undefined) settings.setState(settingsFromQuery(change.scene));
    if (change.behaviour) {
      const { name, args } = change.behaviour;
      try {
        behaviours
          .getState()
          .behaviours.find((b) => b.name === name)
          ?.run(...args);
      } catch (error) {
        log(`${name}: ${error instanceof Error ? error.message : String(error)}`);
      }
    }
    if (change.restart) behaviours.getState().restart();
    if (change.walk) holdOnly(change.walk);
  });

  // Once the dev server takes no more (the stream is over, and this is a
  // browser left behind), drawing stops.
  record(stage.canvas, stage.graphics, (message) => {
    log(message);
    stage.setDrawing(false);
  });

  // The picture is taken right after the stage draws, while it is there.
  const small = document.createElement('canvas');
  small.width = PREVIEW_WIDTH;
  small.height = Math.round((PREVIEW_WIDTH * stage.canvas.height) / stage.canvas.width);
  const context = small.getContext('2d')!;
  let taken = 0;
  let sending = false;
  stage.onFrame = () => {
    const now = performance.now();
    if (sending || now - taken < PREVIEW_MS) return;
    taken = now;
    sending = true;
    context.drawImage(stage.canvas, 0, 0, small.width, small.height);
    small.toBlob(
      (picture) => {
        if (!picture) {
          sending = false;
          return;
        }
        void fetch('/__stream/preview', { method: 'POST', headers: { 'Content-Type': 'image/jpeg' }, body: picture })
          .catch(() => {})
          .finally(() => (sending = false));
      },
      'image/jpeg',
      PREVIEW_QUALITY,
    );
  };
}

// To the dev server's terminal.
function log(message: string): void {
  void fetch('/__stream/log', { method: 'POST', headers: { 'Content-Type': 'text/plain' }, body: message }).catch(() => {});
}
