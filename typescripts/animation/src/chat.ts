import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';
import type {} from 'vite/types/customEvent.d.ts';
import type { ForestNumbered, ForestState } from './story/forest_lake_meeting/events';
import type { ChatStatus, MeetingState, Numbered, TestChat } from './story/lake_meeting/events';

// The YouTube live chat for the lake meeting and the forest lake meeting, as
// the page gets it from the dev server (chat-bridge.ts), which keeps both
// from the one chat: each meeting's events as a Vite event ('meeting:event',
// 'forest-meeting:event'; a story listens with onMeeting()), where a meeting
// stands when a page opens (fetchMeeting()), and how reading the chat goes,
// for the Story tab (chatStatus), which picks the channel and can send
// made-up messages. Only the dev server (npm run dev) has the bridge.

declare module 'vite/types/customEvent.d.ts' {
  interface CustomEventMap {
    'meeting:event': Numbered;
    'forest-meeting:event': ForestNumbered;
    'chat:status': ChatStatus;
  }
}

// Each meeting's events and state, by its story.
interface Meetings {
  lake_meeting: { numbered: Numbered; state: MeetingState };
  forest_lake_meeting: { numbered: ForestNumbered; state: ForestState };
}
type Story = keyof Meetings;
const PATHS: Record<Story, string> = { lake_meeting: '/__chat/meeting', forest_lake_meeting: '/__chat/forest-meeting' };

const listeners: { [S in Story]: Set<(numbered: Meetings[S]['numbered']) => void> } = { lake_meeting: new Set(), forest_lake_meeting: new Set() };

import.meta.hot?.on('meeting:event', (numbered) => {
  for (const listener of listeners.lake_meeting) listener(numbered);
});
import.meta.hot?.on('forest-meeting:event', (numbered) => {
  for (const listener of listeners.forest_lake_meeting) listener(numbered);
});

export function onMeeting<S extends Story = 'lake_meeting'>(listener: (numbered: Meetings[S]['numbered']) => void, story?: S): () => void {
  const set = listeners[story ?? 'lake_meeting'] as Set<(numbered: Meetings[S]['numbered']) => void>;
  set.add(listener);
  return () => {
    set.delete(listener);
  };
}

// A meeting now, or null without a dev server to ask (a built page).
export async function fetchMeeting<S extends Story = 'lake_meeting'>(story?: S): Promise<Meetings[S]['state'] | null> {
  try {
    const answer = await fetch(PATHS[story ?? 'lake_meeting'], { cache: 'no-store' });
    if (!answer.ok || !answer.headers.get('content-type')?.includes('json')) return null;
    return (await answer.json()) as Meetings[S]['state'];
  } catch {
    return null;
  }
}

const OFF: ChatStatus = { channel: null, state: 'off', message: '', video: '' };

export const chatStatus = createStore<ChatStatus>()(() => OFF);

import.meta.hot?.on('chat:status', (status) => chatStatus.setState(status, true));

export function useChatStatus<T>(select: (status: ChatStatus) => T): T {
  return useStore(chatStatus, select);
}

// Asks the dev server how reading the chat goes (a page that opens while it
// reads one).
export async function refreshChatStatus(): Promise<void> {
  try {
    const answer = await fetch('/__chat/status', { cache: 'no-store' });
    if (answer.ok && answer.headers.get('content-type')?.includes('json')) chatStatus.setState((await answer.json()) as ChatStatus, true);
  } catch {
    // no dev server: the chat stays off
  }
}

// Reads a channel's live chat (@handle, a channel or a video), or stops with
// null. Why it couldn't, or null.
export function readChat(channel: string | null): Promise<string | null> {
  return post('/__chat/channel', { channel });
}

// Sends a made-up chat message, as if from YouTube. Why it couldn't, or null.
export function sendTestChat(message: TestChat): Promise<string | null> {
  return post('/__chat/say', message);
}

async function post(path: string, body: unknown): Promise<string | null> {
  try {
    const answer = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (answer.ok) {
      if (answer.headers.get('content-type')?.includes('json')) chatStatus.setState((await answer.json()) as ChatStatus, true);
      return null;
    }
    if (answer.status === 404) return 'This server has no chat bridge: it needs npm run dev.';
    return (await answer.text()).trim() || `The dev server said ${answer.status}.`;
  } catch {
    return 'The dev server did not answer.';
  }
}
