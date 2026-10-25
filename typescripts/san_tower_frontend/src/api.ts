import { createRegistry } from '@bufbuild/protobuf';
import { StringValueSchema, ValueSchema } from '@bufbuild/protobuf/wkt';
import { ConnectError, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { MessageService } from './gen/san/message/v1/message_pb';
import { BrokerService } from './gen/san/tower/v1/broker_pb';
import { BrowserRemoteService } from './gen/san/tower/v1/browser_remote_pb';
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

// errorText is an error as a sentence: the server's own message, without
// Connect's "[code]" prefix.
export const errorText = (err: unknown) =>
  err instanceof ConnectError ? err.rawMessage : err instanceof Error ? err.message : String(err);
