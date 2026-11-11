import { HStack, Box, Text } from '@chakra-ui/react';
import { Code, ConnectError } from '@connectrpc/connect';
import { useEffect, useRef, useState } from 'react';

// StreamState is how a page's server stream is doing: connecting (the first
// time), live (its first message came), reconnecting (it dropped and is
// opened again), paused (the tab is hidden) or ended (the server said there
// is nothing to watch, e.g. the research was deleted).
export type StreamState = 'connecting' | 'live' | 'reconnecting' | 'paused' | 'ended';

// FINAL are the errors a new try would only get again.
const FINAL = [Code.NotFound, Code.InvalidArgument, Code.PermissionDenied];

// useStream keeps a server stream open while the page shows it (the user,
// 2026-11-11: "is crawl realtime update the graph? make it realtime"):
// open(signal) starts it, and each message goes to onMessage, the first after
// each (re)connect being the server's snapshot, so the page resyncs from it.
//   - When it drops (the tower restarting, or no message, not even a
//     heartbeat, for silentMs: two of the tower's 15 s heartbeats missed;
//     a tower that died behind Vite's proxy leaves the stream open and
//     silent), it is opened again after 1, 2, 4 ... up to 30 s.
//   - While the tab is hidden it is closed, and opened again when it shows:
//     the browser keeps six connections to the tower for all its tabs, and
//     each open stream holds one (and the pages stay light, as usePoll's).
//   - A NotFound (or another error in FINAL) ends it for good: onEnd.
// deps restart it, as an effect's.
export function useStream<T>(
  open: (signal: AbortSignal) => AsyncIterable<T>,
  onMessage: (m: T) => void,
  deps: unknown[],
  opts: { silentMs?: number; onEnd?: (err: unknown) => void } = {},
): StreamState {
  const [state, setState] = useState<StreamState>('connecting');
  const latest = useRef({ open, onMessage, onEnd: opts.onEnd });
  latest.current = { open, onMessage, onEnd: opts.onEnd };
  const silentMs = opts.silentMs ?? 32_000;

  useEffect(() => {
    const stop = new AbortController();
    let attempt: AbortController | undefined;
    // wake ends a wait early: the backoff when the tab shows, the pause.
    let wake: (() => void) | undefined;
    const wait = (ms: number) =>
      new Promise<void>((resolve) => {
        const done = () => {
          clearTimeout(t);
          stop.signal.removeEventListener('abort', done);
          wake = undefined;
          resolve();
        };
        const t = ms < Infinity ? setTimeout(done, ms) : undefined;
        wake = done;
        stop.signal.addEventListener('abort', done);
      });
    const onVisibility = () => {
      if (document.hidden) attempt?.abort();
      else wake?.();
    };
    document.addEventListener('visibilitychange', onVisibility);
    setState('connecting');

    void (async () => {
      let delay = 1000;
      while (!stop.signal.aborted) {
        if (document.hidden) {
          setState('paused');
          await wait(Infinity);
          setState('connecting');
          continue;
        }
        const ac = new AbortController();
        attempt = ac;
        const abort = () => ac.abort();
        stop.signal.addEventListener('abort', abort);
        let quiet: ReturnType<typeof setTimeout> | undefined;
        const heard = () => {
          clearTimeout(quiet);
          quiet = setTimeout(abort, silentMs);
        };
        try {
          heard();
          let first = true;
          for await (const m of latest.current.open(ac.signal)) {
            heard();
            if (first) {
              first = false;
              delay = 1000;
              setState('live');
            }
            latest.current.onMessage(m);
          }
        } catch (err) {
          if (stop.signal.aborted) return;
          if (err instanceof ConnectError && FINAL.includes(err.code)) {
            setState('ended');
            latest.current.onEnd?.(err);
            return;
          }
        } finally {
          clearTimeout(quiet);
          stop.signal.removeEventListener('abort', abort);
        }
        if (stop.signal.aborted) return;
        if (document.hidden) continue;
        setState('reconnecting');
        await wait(delay + Math.random() * 300);
        delay = Math.min(delay * 2, 30_000);
      }
    })();
    return () => {
      stop.abort();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, deps);
  return state;
}

// StreamIndicator is a stream's state in a few words: a green dot and Live,
// or Connecting... / Reconnecting... in orange.
export function StreamIndicator({ state }: { state: StreamState }) {
  const [color, text, title] =
    state === 'live'
      ? ['green.500', 'Live', 'Changes show as they happen']
      : state === 'reconnecting'
        ? ['orange.400', 'Reconnecting...', 'The tower does not answer: trying again, and asking every 15 s']
        : state === 'connecting'
          ? ['orange.400', 'Connecting...', '']
          : state === 'paused'
            ? ['gray.400', 'Paused', 'The tab is hidden']
            : ['gray.400', 'Not live', ''];
  return (
    <HStack gap="1.5" title={title} aria-label={`Stream: ${text}`} data-stream={state} flexShrink="0">
      <Box w="2" h="2" rounded="full" bg={color} />
      <Text textStyle="xs" color="fg.muted">
        {text}
      </Text>
    </HStack>
  );
}
