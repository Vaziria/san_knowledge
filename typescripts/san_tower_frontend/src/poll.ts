import { useEffect, useRef } from 'react';

// usePoll calls load at once, then every `every` ms, for as long as the
// component shows it with the same deps (the user, 2026-10-27: keep the
// pages light, after the PC ran out of memory):
//   - not while the browser tab is hidden: the page asks again as soon as it
//     is shown;
//   - not while the last call is still going, so a slow tower does not pile
//     calls up;
//   - stale says that deps changed (or the page went) since the call
//     started: its answer is for what is no longer shown.
export function usePoll(
  load: (stale: () => boolean) => Promise<unknown>,
  every: number,
  deps: unknown[],
  enabled = true,
) {
  const latest = useRef(load);
  latest.current = load;
  useEffect(() => {
    if (!enabled) return;
    let gone = false;
    let busy = false;
    const stale = () => gone;
    const tick = () => {
      if (busy || document.hidden) return;
      busy = true;
      latest.current(stale).finally(() => {
        busy = false;
      });
    };
    tick();
    const t = setInterval(tick, every);
    const onShow = () => !document.hidden && tick();
    document.addEventListener('visibilitychange', onShow);
    return () => {
      gone = true;
      clearInterval(t);
      document.removeEventListener('visibilitychange', onShow);
    };
  }, [every, enabled, ...deps]);
}
