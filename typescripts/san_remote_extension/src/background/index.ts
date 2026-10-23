import { type Client, ConnectError, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { type AttachResponse, BrowserService } from '../gen/san/remote/v1/remote_pb';
import { clearLog, logCommand, logReply } from '../log';
import { browserId, loadSettings, saveStatus } from '../settings';
import { type Result, run } from './commands';

// The service worker attaches to san_remote and runs the commands that come
// down the Attach stream. Chrome stops an idle worker after 30s, and any
// extension API call resets that timer, so each ping (every 20s) saves the
// status. When the worker was stopped anyway, or the server is not running,
// the "reattach" alarm wakes it every 30s to try again.

let current: AbortController | undefined;
let retry: ReturnType<typeof setTimeout> | undefined;
let backoff = 1000;

async function attach(): Promise<void> {
  if (current) return;
  clearTimeout(retry);
  const abort = new AbortController();
  current = abort;
  const { server, name } = await loadSettings();
  // Binary, because it skips fields this build does not know, so a newer
  // server works before the extension is reloaded. JSON would refuse them.
  const client = createClient(BrowserService, createConnectTransport({ baseUrl: server, useBinaryFormat: true }));
  await saveStatus({ state: 'connecting', server, since: Date.now() });
  let error: string;
  try {
    const stream = client.attach(
      {
        browserId: await browserId(),
        name,
        userAgent: navigator.userAgent,
        extensionVersion: chrome.runtime.getManifest().version,
      },
      { signal: abort.signal },
    );
    let since = 0;
    for await (const msg of stream) {
      if (msg.command.case === 'ping') {
        since ||= Date.now();
        backoff = 1000;
        await saveStatus({ state: 'connected', server, since, lastPing: Date.now() });
        continue;
      }
      void answer(client, msg);
    }
    error = 'the server closed the connection';
  } catch (e) {
    error = ConnectError.from(e).message;
  } finally {
    if (current === abort) current = undefined;
  }
  if (abort.signal.aborted) return; // the settings changed, and a new attach has started
  await saveStatus({ state: 'disconnected', server, since: Date.now(), error });
  retry = setTimeout(() => void attach(), backoff);
  backoff = Math.min(backoff * 2, 30_000);
}

async function answer(client: Client<typeof BrowserService>, msg: AttachResponse): Promise<void> {
  const entry = await logCommand(msg);
  let result: Result;
  try {
    result = await run(msg.command);
  } catch (e) {
    result = { case: 'error', value: e instanceof Error ? e.message : String(e) };
  }
  await logReply(entry, result);
  try {
    await client.reply({ commandId: msg.commandId, result });
  } catch (e) {
    console.warn('san_remote: reply failed', e);
  }
}

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'reattach') void attach();
});
// The popup asks to attach on open, so it shows a fresh status; the side
// panel asks to clear the log, which only the worker writes.
chrome.runtime.onMessage.addListener((msg) => {
  if (msg === 'attach') void attach();
  if (msg === 'clear-log') void clearLog();
});
chrome.storage.onChanged.addListener((changes, area) => {
  if (area === 'local' && ('server' in changes || 'name' in changes)) {
    current?.abort();
    current = undefined;
    backoff = 1000;
    void attach();
  }
});

void chrome.alarms.create('reattach', { periodInMinutes: 0.5 });
void attach();
