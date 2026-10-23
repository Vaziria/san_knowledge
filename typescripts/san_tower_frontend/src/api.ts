import { ConnectError, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { MessageService } from './gen/san/message/v1/message_pb';
import { TowerService } from './gen/san/tower/v1/tower_pb';

// The tower serves this UI, so its services are on the same origin. In
// `npm run dev`, Vite proxies them to a running `san_tower serve`.
const transport = createConnectTransport({ baseUrl: window.location.origin });

export const tower = createClient(TowerService, transport);

// broker is the San Message broker the tower runs.
export const broker = createClient(MessageService, transport);

// errorText is an error as a sentence: the server's own message, without
// Connect's "[code]" prefix.
export const errorText = (err: unknown) =>
  err instanceof ConnectError ? err.rawMessage : err instanceof Error ? err.message : String(err);
