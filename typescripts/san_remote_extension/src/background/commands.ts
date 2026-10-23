import type { MessageInitShape } from '@bufbuild/protobuf';
import {
  type AttachResponse,
  type ExecRequest,
  type ReplyRequestSchema,
  World,
} from '../gen/san/remote/v1/remote_pb';
import { extensionPage, userScriptsAllowed } from '../settings';

export type Result = MessageInitShape<typeof ReplyRequestSchema>['result'];

const defaultTimeout = 30_000;

// run carries out one command from the server.
export async function run(cmd: AttachResponse['command']): Promise<Result> {
  switch (cmd.case) {
    case 'listTabs':
      return { case: 'listTabs', value: { tabs: (await chrome.tabs.query({})).map(toTab) } };
    case 'openTab': {
      const { url, background, timeoutMs, focus, newWindow } = cmd.value;
      const tab = await load(async () => {
        if (newWindow) {
          // The size of the window in use, not state 'maximized': on Windows
          // a window created maximized came out 0x0 and stayed hidden.
          const cur = await chrome.windows.getLastFocused();
          const w = await chrome.windows.create({ url, focused: focus, left: cur.left, top: cur.top, width: cur.width, height: cur.height });
          if (!w) throw new Error('Chrome did not open a window');
          return w.tabs?.[0] ?? (await chrome.tabs.query({ windowId: w.id }))[0];
        }
        const tab = await chrome.tabs.create({ url, active: !background });
        if (focus) await chrome.windows.update(tab.windowId, { focused: true });
        return tab;
      }, timeoutMs);
      return { case: 'openTab', value: { tab: toTab(tab) } };
    }
    case 'focusTab': {
      const tab = await target(cmd.value.tabId);
      await chrome.tabs.update(tab.id!, { active: true });
      await chrome.windows.update(tab.windowId, { focused: true });
      if ((await chrome.windows.get(tab.windowId)).state === 'minimized') {
        await chrome.windows.update(tab.windowId, { state: 'normal' });
      }
      return { case: 'focusTab', value: { tab: toTab(await chrome.tabs.get(tab.id!)) } };
    }
    case 'screenshot': {
      const tab = await target(cmd.value.tabId);
      if (!tab.active) await chrome.tabs.update(tab.id!, { active: true });
      const url = await chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' });
      const png = Uint8Array.from(atob(url.slice(url.indexOf(',') + 1)), (c) => c.charCodeAt(0));
      return { case: 'screenshot', value: { png } };
    }
    case 'navigate': {
      const { url, timeoutMs } = cmd.value;
      const id = (await target(cmd.value.tabId)).id!;
      const tab = await load(() => chrome.tabs.update(id, { url }) as Promise<chrome.tabs.Tab>, timeoutMs);
      return { case: 'navigate', value: { tab: toTab(tab) } };
    }
    case 'closeTab':
      if (!cmd.value.tabId) throw new Error('closing a tab needs its id');
      await chrome.tabs.remove(cmd.value.tabId);
      return { case: 'closeTab', value: {} };
    case 'exec':
      return { case: 'exec', value: await exec(cmd.value) };
  }
  throw new Error(`unknown command ${cmd.case}`);
}

// target is the tab with this id, or with 0 the active tab of the focused window.
async function target(id: number): Promise<chrome.tabs.Tab> {
  if (id) return chrome.tabs.get(id);
  const [tab] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
  if (!tab) throw new Error('there is no active tab: give a tab id');
  return tab;
}

interface Outcome {
  json?: string;
  error?: string;
}

async function exec(req: ExecRequest) {
  if (!userScriptsAllowed()) {
    throw new Error(`"Allow User Scripts" is off for San Remote, so it cannot run JavaScript: turn it on at ${extensionPage()}`);
  }
  const tab = await target(req.tabId);
  const ms = req.timeoutMs || defaultTimeout;
  const results = await within(
    chrome.userScripts.execute<Outcome>({
      target: { tabId: tab.id! },
      js: [{ code: wrap(req.code) }],
      world: req.world === World.MAIN ? 'MAIN' : 'USER_SCRIPT',
      injectImmediately: true,
    }),
    ms,
    `the code did not finish in ${ms}ms`,
  );
  const r = results[0];
  if (!r) throw new Error(`nothing ran in tab ${tab.id} (${tab.url})`);
  if (r.error !== undefined) throw new Error(r.error);
  // The wrapper always returns an object, so nothing means the code did not
  // run at all: it does not parse, and Chrome does not say so.
  if (!r.result) throw new Error(await whyNotRun(tab.id!, req.code));
  if (r.result.error !== undefined) throw new Error(r.result.error);
  return { resultJson: r.result.json ?? 'null', tab: toTab(tab) };
}

// syntaxWorld may compile code with new Function, which the page's CSP or the
// default user script world would refuse. It only compiles, never runs.
const syntaxWorld = 'san_remote_syntax';

async function whyNotRun(tabId: number, code: string): Promise<string> {
  await chrome.userScripts.configureWorld({ worldId: syntaxWorld, csp: "script-src 'self' 'unsafe-eval'" });
  const [r] = await chrome.userScripts.execute<string | null>({
    target: { tabId },
    worldId: syntaxWorld,
    js: [{ code: `(() => { try { new (async () => {}).constructor(${JSON.stringify(code)}); return null; } catch (e) { return String(e); } })()` }],
  });
  return r?.result ?? 'the code did not run to the end (did the page navigate away?)';
}

// wrap makes the code the body of an async function. Its value comes back as
// JSON text, so every value arrives the same way, and an error with its stack.
function wrap(code: string): string {
  return `(async () => {
  try {
    const value = await (async () => {
${code}
    })();
    return { json: JSON.stringify(value) ?? 'null' };
  } catch (e) {
    return { error: e instanceof Error ? e.stack || String(e) : String(e) };
  }
})()`;
}

type OnUpdated = Parameters<typeof chrome.tabs.onUpdated.addListener>[0];
type OnRemoved = Parameters<typeof chrome.tabs.onRemoved.addListener>[0];

// load runs change, which creates or updates a tab, and waits until that tab
// has loaded. It listens before the change, so a fast load is not missed.
function load(change: () => Promise<chrome.tabs.Tab>, timeoutMs: number): Promise<chrome.tabs.Tab> {
  const ms = timeoutMs || defaultTimeout;
  return new Promise((resolve, reject) => {
    let id: number | undefined;
    const loaded = new Set<number>();
    const timer = setTimeout(() => finish(new Error(`the page did not finish loading in ${ms}ms`)), ms);
    const onUpdated: OnUpdated = (tabId, info, tab) => {
      if (info.status !== 'complete') return;
      if (id === undefined) loaded.add(tabId);
      else if (tabId === id) finish(undefined, tab);
    };
    const onRemoved: OnRemoved = (tabId) => {
      if (tabId === id) finish(new Error(`tab ${tabId} was closed while it loaded`));
    };
    function finish(err?: Error, tab?: chrome.tabs.Tab) {
      clearTimeout(timer);
      chrome.tabs.onUpdated.removeListener(onUpdated);
      chrome.tabs.onRemoved.removeListener(onRemoved);
      if (err) reject(err);
      else resolve(tab!);
    }
    chrome.tabs.onUpdated.addListener(onUpdated);
    chrome.tabs.onRemoved.addListener(onRemoved);
    change().then(
      async (tab) => {
        id = tab.id!;
        if (loaded.has(id)) finish(undefined, await chrome.tabs.get(id));
      },
      (e) => finish(e instanceof Error ? e : new Error(String(e))),
    );
  });
}

function within<T>(p: Promise<T>, ms: number, message: string): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const late = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(message)), ms);
  });
  return Promise.race([p, late]).finally(() => clearTimeout(timer));
}

function toTab(t: chrome.tabs.Tab) {
  return {
    id: t.id ?? 0,
    windowId: t.windowId,
    index: t.index,
    active: t.active,
    url: t.url || t.pendingUrl || '',
    title: t.title ?? '',
    status: t.status ?? '',
  };
}
