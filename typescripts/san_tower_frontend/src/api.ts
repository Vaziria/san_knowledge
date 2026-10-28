import { createRegistry } from '@bufbuild/protobuf';
import { StringValueSchema, ValueSchema } from '@bufbuild/protobuf/wkt';
import { ConnectError, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { MessageService } from './gen/san/message/v1/message_pb';
import { BrokerService } from './gen/san/tower/v1/broker_pb';
import { BrowserRemoteService } from './gen/san/tower/v1/browser_remote_pb';
import { ChatService } from './gen/san/tower/v1/chat_pb';
import { ConnectionService } from './gen/san/tower/v1/connection_pb';
import { StorageService } from './gen/san/tower/v1/storage_pb';
import { TowerService } from './gen/san/tower/v1/tower_pb';
import { WorkflowService } from './gen/san/tower/v1/workflow_pb';

// The tower serves this UI, so its services are on the same origin. In
// `npm run dev`, Vite proxies them to a running `san_tower serve`. The
// registry has the message types a google.protobuf.Any can hold in a reply (a
// run's step outputs), which reading JSON needs.
const transport = createConnectTransport({
  baseUrl: window.location.origin,
  jsonOptions: { registry: createRegistry(ValueSchema, StringValueSchema) },
});

export const tower = createClient(TowerService, transport);

// broker is the San Message broker the tower runs, and brokerSettings the
// tower's setup of it (its data folder).
export const broker = createClient(MessageService, transport);
export const brokerSettings = createClient(BrokerService, transport);

// workflows are the workflows in the tower's database.
export const workflows = createClient(WorkflowService, transport);

// connections are the connections to other programs that workflow steps use.
export const connections = createClient(ConnectionService, transport);

// browserRemote is the San Remote server the tower can run, for the browser
// extension and the site services.
export const browserRemote = createClient(BrowserRemoteService, transport);

// storages are the named base folders that workflow steps use.
export const storages = createClient(StorageService, transport);

// chats are the Chat page's chats with workflows (their Tower Chat steps).
export const chats = createClient(ChatService, transport);

// errorText is an error as a sentence: the server's own message, without
// Connect's "[code]" prefix.
export const errorText = (err: unknown) =>
  err instanceof ConnectError ? err.rawMessage : err instanceof Error ? err.message : String(err);

// ClaudeHealth is GET /api/claude-health: the installed Claude Code's version,
// the oldest the Claude Ask (CLI) steps support, and the probe error when
// claude is not on PATH or printed something unexpected. Layout's startup
// banner shows a warning when version is empty or older than minVersion.
export type ClaudeHealth = { version: string; minVersion: string; error?: string };

export async function claudeHealth(): Promise<ClaudeHealth> {
  const r = await fetch('/api/claude-health');
  if (!r.ok) {
    throw new Error(`GET /api/claude-health: ${r.status}`);
  }
  return (await r.json()) as ClaudeHealth;
}

// cmpVersion compares two "major.minor.patch" strings; empty = older than
// everything. Returns <0 when a<b, 0 when equal, >0 when a>b. Used by the
// banner to decide whether the installed claude is behind the minimum.
export function cmpVersion(a: string, b: string): number {
  const pa = a.split('.').map((n) => parseInt(n, 10) || 0);
  const pb = b.split('.').map((n) => parseInt(n, 10) || 0);
  const n = Math.max(pa.length, pb.length);
  for (let i = 0; i < n; i++) {
    const x = pa[i] ?? 0;
    const y = pb[i] ?? 0;
    if (x !== y) return x - y;
  }
  return 0;
}
