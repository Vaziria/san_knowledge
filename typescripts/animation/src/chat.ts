import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';
import type {} from 'vite/types/customEvent.d.ts';
import type { ChatStatus, MeetingState, Numbered, TestChat } from './story/lake_meeting/events';

// The YouTube live chat for the lake meeting, as the page gets it from the dev
// server (chat-bridge.ts): the meeting's events as the Vite event
// 'meeting:event' (the story listens with onMeeting()), where the meeting
// stands when a page opens (fetchMeeting()), and how reading the chat goes,
// for the Story tab (chatStatus), which picks the channel and can send
// made-up messages. Only the dev server (npm run dev) has the bridge.

declare module 'vite/types/customEvent.d.ts' {
  interface CustomEventMap {
    'meeting:event': Numbered;
    'chat:status': ChatStatus;
  }
}

const listeners = new Set<(numbered: Numbered) => void>();

import.meta.hot?.on('meeting:event', (numbered) => {
  for (const listener of listeners) listener(numbered);
});

export function onMeeting(listener: (numbered: Numbered) => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// The meeting now, or null without a dev server to ask (a built page).
export async function fetchMeeting(): Promise<MeetingState | null> {
  try {
    const answer = await fetch('/__chat/meeting', { cache: 'no-store' });
    if (!answer.ok || !answer.headers.get('content-type')?.includes('json')) return null;
    return (await answer.json()) as MeetingState;
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
