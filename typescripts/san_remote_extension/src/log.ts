import { create, type JsonValue, toJson } from '@bufbuild/protobuf';
import type { Result } from './background/commands';
import { type AttachResponse, AttachResponseSchema, ReplyRequestSchema, World } from './gen/san/remote/v1/remote_pb';

// The debug log: every command the server sent and the reply to it, newest
// last, kept in chrome.storage.session for the side panel. Only the service
// worker writes it; the panel asks it to clear.

export interface LogEntry {
  id: string; // the command id
  kind: string; // exec, openTab, navigate, closeTab, listTabs
  at: number; // when it arrived, in ms
  summary: string; // one line: the tab, the world, the url
  request: string; // exec: its code; otherwise the command as JSON
  state: 'running' | 'ok' | 'error';
  ms?: number; // how long it took
  reply?: string; // exec: its result; otherwise the answer as JSON; or the error
}

const keep = 200;
const maxText = 20_000;

export async function loadLog(): Promise<LogEntry[]> {
  const { log } = await chrome.storage.session.get('log');
  return (log as LogEntry[] | undefined) ?? [];
}

// ---- written by the service worker ----

let entries: Promise<LogEntry[]> | undefined;

async function save(): Promise<void> {
  const log = await entries!;
  if (log.length > keep) log.splice(0, log.length - keep);
  await chrome.storage.session.set({ log });
}

export async function logCommand(msg: AttachResponse): Promise<LogEntry> {
  entries ??= loadLog();
  const cmd = msg.command;
  const entry: LogEntry = { id: msg.commandId, kind: cmd.case ?? 'unknown', at: Date.now(), summary: '', request: '', state: 'running' };
  const tab = (id: number) => (id ? `tab ${id}` : 'active tab');
  switch (cmd.case) {
    case 'exec':
      entry.summary = `${tab(cmd.value.tabId)} · ${cmd.value.world === World.MAIN ? 'main' : 'isolated'} world`;
      entry.request = clip(cmd.value.code);
      break;
    case 'openTab':
      entry.summary = [
        cmd.value.url,
        cmd.value.background && 'background',
        cmd.value.focus && 'focus',
        cmd.value.newWindow && 'new window',
      ].filter(Boolean).join(' · ');
      break;
    case 'focusTab':
    case 'screenshot':
      entry.summary = tab(cmd.value.tabId);
      break;
    case 'navigate':
      entry.summary = `${tab(cmd.value.tabId)} → ${cmd.value.url}`;
      break;
    case 'closeTab':
      entry.summary = tab(cmd.value.tabId);
      break;
  }
  if (cmd.case !== 'exec') entry.request = clip(JSON.stringify(oneof(toJson(AttachResponseSchema, msg)), null, 2));
  (await entries).push(entry);
  await save();
  return entry;
}

export async function logReply(entry: LogEntry, result: Result): Promise<void> {
  entry.ms = Date.now() - entry.at;
  const reply = create(ReplyRequestSchema, { result });
  const r = reply.result;
  entry.state = r.case === 'error' ? 'error' : 'ok';
  if (r.case === 'error') entry.reply = clip(r.value);
  else if (r.case === 'exec') entry.reply = clip(pretty(r.value.resultJson));
  else if (r.case === 'screenshot') entry.reply = `PNG, ${Math.round(r.value.png.length / 1024)} KB`;
  else entry.reply = clip(JSON.stringify(oneof(toJson(ReplyRequestSchema, reply)), null, 2));
  await save();
}

// oneof is the value of the message's one set oneof field, as JSON.
function oneof(json: JsonValue): JsonValue {
  const fields = Object.entries(json as Record<string, JsonValue>).filter(([k]) => k !== 'commandId');
  return fields[0]?.[1] ?? {};
}

export async function clearLog(): Promise<void> {
  entries = Promise.resolve([]);
  await save();
}

function pretty(json: string): string {
  try {
    return JSON.stringify(JSON.parse(json), null, 2);
  } catch {
    return json;
  }
}

function clip(s: string): string {
  return s.length > maxText ? `${s.slice(0, maxText)}\n… ${s.length - maxText} more characters` : s;
}
