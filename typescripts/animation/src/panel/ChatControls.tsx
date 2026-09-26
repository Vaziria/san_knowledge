import { useEffect, useState } from 'react';
import { readChat, refreshChatStatus, sendTestChat, useChatStatus } from '../chat';
import type { ChatStatus, TestChat } from '../story/lake_meeting/events';
import { Action, Select, TextField } from './controls';

// The lake meeting's chat, on the Story tab while it plays: which YouTube
// channel's live chat the dev server reads (chat-bridge.ts), how that goes,
// and made-up messages to try the meeting without YouTube. The channel is
// kept in this browser, so it is there after a reload; reading it is an
// action, like going live, so nothing of it is in the URL.

const CHANNEL_KEY = 'animation-chat-channel';
const AS: { value: TestChat['as']; label: string }[] = [
  { value: 'viewer', label: 'a viewer' },
  { value: 'owner', label: 'the owner (the Bear)' },
  { value: 'super_chat', label: 'a Super Chat' },
  { value: 'member', label: 'a new member' },
];

export function ChatControls() {
  const status = useChatStatus((s) => s);
  const [channel, setChannel] = useState(() => stored(CHANNEL_KEY) ?? '');
  const [name, setName] = useState('Tester');
  const [text, setText] = useState('Hello everyone!');
  const [as, setAs] = useState<TestChat['as']>('viewer');
  const [problem, setProblem] = useState<string | null>(null);
  useEffect(() => {
    void refreshChatStatus();
  }, []);
  const reading = status.state !== 'off';

  const read = async () => {
    const wanted = channel.trim();
    if (!reading) store(CHANNEL_KEY, wanted);
    setProblem(await readChat(reading ? null : wanted));
  };
  const send = async () => setProblem(await sendTestChat({ name, text, as }));

  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
      <p className="col-span-full text-xs font-medium">YouTube live chat</p>
      <TextField label="Channel" value={channel} placeholder="@yourchannel" disabled={reading} onChange={setChannel} />
      <Action
        label={reading ? 'Stop reading' : 'Read chat'}
        wide
        tone={reading ? 'danger' : 'main'}
        disabled={!reading && !channel.trim()}
        onClick={() => void read()}
      />
      <StatusLine status={status} />

      <p className="col-span-full mt-2 text-xs font-medium">Try it without YouTube</p>
      <TextField label="Name" value={name} onChange={setName} />
      <TextField label="Message" value={text} onChange={setText} />
      <Select label="As" value={as} options={AS} onChange={(value) => setAs(value as TestChat['as'])} />
      <Action label="Send" wide onClick={() => void send()} />
      {problem && <p className="col-span-full text-xs font-medium text-destructive">{problem}</p>}
      <p className="col-span-full text-xs text-muted-foreground">
        Commands: !jump, !walk, !run, !stop, !flap (a penguin). !cat, !wolf, !deer, !bird, !fox, !snake or !penguin switch the animal.
      </p>
    </div>
  );
}

function StatusLine({ status }: { status: ChatStatus }) {
  const line = 'col-span-full text-xs';
  switch (status.state) {
    case 'connecting':
      return <p className={line}>Finding {status.channel}'s live stream…</p>;
    case 'reading':
      return (
        <p className={`${line} font-medium`}>
          <span className="text-destructive">●</span> Reading the chat of{' '}
          <a className="underline" href={status.video} target="_blank" rel="noreferrer">
            {status.video.replace(/^https:\/\/www\./, '')}
          </a>
        </p>
      );
    case 'waiting':
      return <p className={`${line} text-muted-foreground`}>{status.message}</p>;
    case 'error':
      return <p className={`${line} font-medium text-destructive`}>{status.message}</p>;
    default:
      return <p className={`${line} text-muted-foreground`}>Not reading any chat. Seven bots keep the meeting until viewers come.</p>;
  }
}

function stored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null; // storage can be blocked
  }
}

function store(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // not remembered, that's all
  }
}
