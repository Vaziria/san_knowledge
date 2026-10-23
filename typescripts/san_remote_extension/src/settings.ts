// Shared by the service worker and the popup. The settings live in
// chrome.storage.local; the connection status in chrome.storage.session,
// written by the service worker and shown by the popup.

export const defaultServer = 'http://127.0.0.1:7476';

export interface Settings {
  server: string;
  name: string;
}

export async function loadSettings(): Promise<Settings> {
  const s = await chrome.storage.local.get(['server', 'name']);
  return {
    server: (s.server as string | undefined) || defaultServer,
    name: (s.name as string | undefined) || defaultName(),
  };
}

export function saveSettings(s: Settings): Promise<void> {
  return chrome.storage.local.set(s);
}

function defaultName(): string {
  return navigator.userAgent.includes('Edg/') ? 'Edge' : 'Chrome';
}

// browserId is made once and kept, so the server sees a restarted service
// worker as the same browser.
export async function browserId(): Promise<string> {
  const { browserId } = await chrome.storage.local.get('browserId');
  if (typeof browserId === 'string' && browserId) return browserId;
  const id = crypto.randomUUID();
  await chrome.storage.local.set({ browserId: id });
  return id;
}

export interface Status {
  state: 'connecting' | 'connected' | 'disconnected';
  server: string;
  // When it entered this state, in ms.
  since: number;
  // The server's last ping, while connected.
  lastPing?: number;
  error?: string;
}

export async function loadStatus(): Promise<Status | undefined> {
  const { status } = await chrome.storage.session.get('status');
  return status as Status | undefined;
}

export function saveStatus(status: Status): Promise<void> {
  return chrome.storage.session.set({ status });
}

// userScriptsAllowed says whether "Allow User Scripts" is on for the
// extension: until it is, chrome.userScripts throws.
export function userScriptsAllowed(): boolean {
  try {
    chrome.userScripts.getScripts().catch(() => {});
    return true;
  } catch {
    return false;
  }
}

export function extensionPage(): string {
  return `chrome://extensions/?id=${chrome.runtime.id}`;
}
