import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt';
import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  Link,
  Menu,
  NativeSelect,
  Portal,
  Spinner,
  Stack,
  Switch,
  Text,
  Textarea,
} from '@chakra-ui/react';
import {
  EllipsisVerticalIcon,
  MessageSquarePlusIcon,
  PencilIcon,
  SendHorizontalIcon,
  Trash2Icon,
} from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react';
import Markdown from 'react-markdown';
import { Link as RouterLink, useSearchParams } from 'react-router';
import remarkGfm from 'remark-gfm';
import { chats, errorText, workflows } from '../api';
import {
  ChatFrom,
  type Chat as ChatT,
  type ChatMessage,
  type ChatRun,
  type ChatWorkflow,
} from '../gen/san/tower/v1/chat_pb';
import { RunStatus } from '../gen/san/tower/v1/workflow_pb';
import { usePoll } from '../poll';
import { FormDialog, FormField, toast } from '../ui';

// Chat is the tower's own chat (the user, 2026-10-28): each chat talks with
// one workflow that has a Tower Chat step. A message sent here starts a run of
// that workflow; its Tower Chat Send steps answer in the chat. The chats are
// listed on the left, the newest message first; the chat shown is in the URL
// (?c=3).
export function Chat() {
  const [params, setParams] = useSearchParams();
  const picked = Number(params.get('c')) || 0;
  const [list, setList] = useState<ChatT[]>();
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const [creating, setCreating] = useState(false);

  usePoll(
    async (stale) => {
      try {
        const r = await chats.listChats({});
        if (stale()) return;
        setList(r.chats);
        setError('');
      } catch (err) {
        if (!stale()) setError(errorText(err));
      }
    },
    5000,
    [reload],
  );
  const changed = () => setReload((n) => n + 1);
  const open = (id: number) => setParams(id ? { c: String(id) } : {});

  // With no chat picked, the newest is shown.
  const shown = list?.find((c) => c.id === picked) ?? (picked ? undefined : list?.[0]);

  return (
    <Flex h="calc(100dvh - 3rem)" gap="4" direction={{ base: 'column', md: 'row' }}>
      <Stack w={{ base: 'full', md: '72' }} flexShrink={0} gap="3" minH="0" maxH={{ base: '40%', md: 'full' }}>
        <HStack justify="space-between">
          <Heading size="lg">Chat</Heading>
          <Button size="sm" colorPalette="blue" onClick={() => setCreating(true)}>
            <MessageSquarePlusIcon />
            New chat
          </Button>
        </HStack>
        {error ? (
          <Text color="fg.error">The tower does not answer: {error}</Text>
        ) : !list ? (
          <Text color="fg.muted">Loading...</Text>
        ) : list.length === 0 ? (
          <Text color="fg.muted" textStyle="sm">
            No chats yet. A chat talks with a workflow that has a Tower Chat step (Messengers › Tower Chat).
          </Text>
        ) : (
          <Stack gap="1" overflowY="auto" minH="0">
            {list.map((c) => (
              <ChatRow key={c.id} chat={c} current={c.id === shown?.id} onClick={() => open(c.id)} />
            ))}
          </Stack>
        )}
      </Stack>
      <Flex flex="1" minW="0" minH="0" direction="column" borderWidth="1px" rounded="md">
        {shown ? (
          <Conversation key={shown.id} chat={shown} onChanged={changed} onDeleted={() => (open(0), changed())} />
        ) : (
          <Flex flex="1" align="center" justify="center" p="6">
            <Text color="fg.muted" textAlign="center">
              {picked && list ? 'That chat is gone.' : 'Pick a chat, or start one with New chat.'}
            </Text>
          </Flex>
        )}
      </Flex>
      {creating && (
        <NewChatDialog
          onClose={() => setCreating(false)}
          onCreated={(c) => {
            changed();
            open(c.id);
          }}
        />
      )}
    </Flex>
  );
}

// ChatRow is a chat in the list: its name, its workflow, and its newest
// message.
function ChatRow({ chat, current, onClick }: { chat: ChatT; current: boolean; onClick: () => void }) {
  const last = chat.last;
  return (
    <Box
      as="button"
      textAlign="start"
      px="3"
      py="2"
      rounded="md"
      bg={current ? 'bg.emphasized' : undefined}
      _hover={{ bg: current ? 'bg.emphasized' : 'bg.muted' }}
      onClick={onClick}
    >
      <HStack justify="space-between" gap="2">
        <Text fontWeight="semibold" truncate>
          {chat.name}
        </Text>
        <Text textStyle="xs" color="fg.muted" flexShrink={0}>
          {when(chat.updatedAt)}
        </Text>
      </HStack>
      <HStack gap="1.5" textStyle="xs" color="fg.muted">
        <Box w="2" h="2" rounded="full" flexShrink={0} bg={chat.workflowActive ? 'green.solid' : 'fg.subtle'} />
        <Text truncate>{chat.workflowName || 'workflow gone'}</Text>
      </HStack>
      {last && (
        <Text textStyle="sm" color="fg.muted" truncate>
          {last.from === ChatFrom.USER ? 'You: ' : ''}
          {last.text}
        </Text>
      )}
    </Box>
  );
}

// gone is the status of a watched run the tower no longer keeps.
const gone = -1;

// Conversation is one chat: its messages, oldest at the top, how the runs its
// messages started are doing, and the box to write in. It asks every second
// for what came after its newest message and how the runs still going are
// doing, while the browser tab shows.
function Conversation({ chat, onChanged, onDeleted }: { chat: ChatT; onChanged: () => void; onDeleted: () => void }) {
  const [current, setCurrent] = useState(chat);
  const [messages, setMessages] = useState<ChatMessage[]>();
  const [more, setMore] = useState(false);
  const [runs, setRuns] = useState<Map<number, ChatRun | typeof gone>>(new Map());
  const [error, setError] = useState('');
  const [text, setText] = useState('');
  const [sending, setSending] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const scroller = useRef<HTMLDivElement>(null);
  const stick = useRef(true); // keep the newest message in view
  const keep = useRef<number>(undefined); // the scroll height before older messages came in on top

  // Messages come in once: from the poll, or from a send.
  const add = (list: ChatMessage[]) =>
    setMessages((old) => {
      const have = new Set(old?.map((m) => m.id));
      const added = list.filter((m) => !have.has(m.id));
      return added.length ? [...(old ?? []), ...added].sort((a, b) => a.id - b.id) : old;
    });
  const note = (list: ChatRun[], asked: number[]) =>
    setRuns((old) => {
      const next = new Map(old);
      for (const id of asked) if (!list.some((r) => r.id === id)) next.set(id, gone);
      for (const r of list) next.set(r.id, r);
      return next;
    });

  const latest = useRef({ messages, runs });
  latest.current = { messages, runs };
  usePoll(
    async (stale) => {
      const { messages, runs } = latest.current;
      const lastId = messages?.at(-1)?.id ?? 0;
      // The runs of the user's messages not known to have ended: on the
      // first ask after the messages came, all of them.
      const watch = (messages ?? [])
        .filter((m) => m.from === ChatFrom.USER)
        .flatMap((m) => m.runIds)
        .filter((id) => {
          const r = runs.get(id);
          return r === undefined || (r !== gone && r.status === RunStatus.RUNNING);
        });
      try {
        const r = await chats.listChatMessages({ chatId: chat.id, afterId: lastId, watchRuns: watch });
        if (stale()) return;
        if (!messages) {
          setMessages(r.messages);
          setMore(r.more);
        } else {
          add(r.messages);
          note(r.runs, watch);
        }
        if (r.chat) {
          if (r.chat.workflowActive !== current.workflowActive || r.chat.name !== current.name) onChanged();
          setCurrent(r.chat);
        }
        setError('');
      } catch (err) {
        if (!stale()) setError(errorText(err));
      }
    },
    1000,
    [chat.id],
  );

  const older = async () => {
    const first = messages?.[0];
    if (!first) return;
    try {
      const r = await chats.listChatMessages({ chatId: chat.id, beforeId: first.id });
      keep.current = scroller.current?.scrollHeight;
      setMessages((old) => [...r.messages, ...(old ?? [])]);
      setMore(r.more);
    } catch (err) {
      toast(`Older messages: ${errorText(err)}`, true);
    }
  };

  // The newest message stays in view while the reader is at the bottom; older
  // messages coming in on top leave the view where it was.
  useLayoutEffect(() => {
    const el = scroller.current;
    if (!el) return;
    if (keep.current !== undefined) {
      el.scrollTop += el.scrollHeight - keep.current;
      keep.current = undefined;
    } else if (stick.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, runs]);

  const send = async () => {
    const t = text.trim();
    if (!t || sending) return;
    setSending(true);
    try {
      const r = await chats.sendChatMessage({ chatId: chat.id, text: t });
      setText('');
      stick.current = true;
      if (r.message) add([r.message]);
      note(r.runs, []);
      onChanged();
    } catch (err) {
      toast(errorText(err), true);
    }
    setSending(false);
  };
  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      void send();
    }
  };

  const switchActive = async (on: boolean) => {
    try {
      await workflows.setWorkflowActive({ id: current.workflowId, active: on });
      setCurrent({ ...current, workflowActive: on });
      onChanged();
    } catch (err) {
      toast(`${current.workflowName} not switched ${on ? 'on' : 'off'}: ${errorText(err)}`, true);
    }
  };

  const closed = !current.workflowName
    ? 'Its workflow is gone: start a new chat.'
    : !current.workflowActive
      ? `${current.workflowName} is switched off: switch it on to chat with it.`
      : '';

  return (
    <>
      <HStack px="4" py="2.5" borderBottomWidth="1px" justify="space-between" gap="3">
        <Box minW="0">
          <Text fontWeight="semibold" truncate>
            {current.name}
          </Text>
          {current.workflowName && (
            <Text textStyle="xs" color="fg.muted" truncate>
              with{' '}
              <Link asChild color="fg.muted" textDecoration="underline">
                <RouterLink to={`/workflow/${current.workflowId}`}>{current.workflowName}</RouterLink>
              </Link>
            </Text>
          )}
        </Box>
        <HStack gap="2" flexShrink={0}>
          {current.workflowName && (
            <Switch.Root
              size="sm"
              colorPalette="green"
              checked={current.workflowActive}
              onCheckedChange={(e) => void switchActive(e.checked)}
              title={`${current.workflowName} ${current.workflowActive ? 'is on' : 'is off'}`}
            >
              <Switch.HiddenInput />
              <Switch.Control>
                <Switch.Thumb />
              </Switch.Control>
              <Switch.Label textStyle="sm" color="fg.muted">
                {current.workflowActive ? 'On' : 'Off'}
              </Switch.Label>
            </Switch.Root>
          )}
          <ChatMenu name={current.name} onRename={() => setRenaming(true)} onDelete={() => setDeleting(true)} />
        </HStack>
      </HStack>
      <Box
        ref={scroller}
        flex="1"
        minH="0"
        overflowY="auto"
        px="4"
        py="3"
        onScroll={(e) => {
          const el = e.currentTarget;
          stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
        }}
      >
        {error && <Text color="fg.error">The tower does not answer: {error}</Text>}
        {!messages ? (
          <Text color="fg.muted">Loading...</Text>
        ) : (
          <Stack gap="3">
            {more && (
              <Button size="xs" variant="ghost" alignSelf="center" onClick={() => void older()}>
                Older messages
              </Button>
            )}
            {messages.length === 0 && (
              <Text color="fg.muted" textAlign="center" py="6">
                No messages yet. What you write starts a run of {current.workflowName || 'its workflow'}, which answers
                with its Tower Chat Send steps.
              </Text>
            )}
            {messages.map((m) => (
              <Bubble key={m.id} message={m} runs={runs} workflowName={current.workflowName} />
            ))}
          </Stack>
        )}
      </Box>
      <Box px="4" py="3" borderTopWidth="1px">
        {closed ? (
          <Text textStyle="sm" color="fg.muted">
            {closed}
          </Text>
        ) : (
          <HStack align="flex-end" gap="2">
            <Textarea
              autoFocus
              autoresize
              maxH="40"
              rows={1}
              value={text}
              placeholder={`Message ${current.workflowName}... (Enter sends, Shift+Enter a new line)`}
              onChange={(e) => setText(e.currentTarget.value)}
              onKeyDown={onKey}
            />
            <IconButton
              aria-label="Send"
              title="Send"
              colorPalette="blue"
              loading={sending}
              disabled={!text.trim()}
              onClick={() => void send()}
            >
              <SendHorizontalIcon />
            </IconButton>
          </HStack>
        )}
      </Box>
      {renaming && (
        <RenameChatDialog
          chat={current}
          onClose={() => setRenaming(false)}
          onRenamed={(c) => {
            setCurrent(c);
            onChanged();
          }}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete chat ${current.name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(false)}
          onSubmit={async () => {
            await chats.deleteChat({ id: current.id });
            toast(`Deleted chat ${current.name}`);
            onDeleted();
          }}
        >
          <Text>Its messages go with it. The workflow and its runs stay.</Text>
        </FormDialog>
      )}
    </>
  );
}

// Bubble is one message: the user's on the right, plain; a workflow's on the
// left, as Markdown when it was sent so. Under the user's are the runs it
// started: going on with their last line, or how they ended.
function Bubble({
  message: m,
  runs,
  workflowName,
}: {
  message: ChatMessage;
  runs: Map<number, ChatRun | typeof gone>;
  workflowName: string;
}) {
  const mine = m.from === ChatFrom.USER;
  return (
    <Stack align={mine ? 'flex-end' : 'flex-start'} gap="1">
      <Box
        maxW={{ base: '92%', md: '75%' }}
        px="3"
        py="2"
        rounded="lg"
        bg={mine ? 'blue.solid' : 'bg.muted'}
        color={mine ? 'blue.contrast' : 'fg'}
        overflowWrap="anywhere"
      >
        {m.markdown ? (
          <Box css={markdownStyles}>
            <Markdown remarkPlugins={[remarkGfm]} components={{ a: ({ node: _, ...p }) => <a {...p} target="_blank" rel="noreferrer" /> }}>
              {m.text}
            </Markdown>
          </Box>
        ) : (
          <Text whiteSpace="pre-wrap">{m.text}</Text>
        )}
      </Box>
      <HStack gap="2" textStyle="xs" color="fg.muted" px="1">
        {!mine && <Text>{workflowName || 'workflow'}</Text>}
        <Text title={m.createdAt && timestampDate(m.createdAt).toLocaleString()}>{when(m.createdAt)}</Text>
      </HStack>
      {mine &&
        m.runIds.map((id) => <RunNote key={id} run={runs.get(id)} workflowId={m.workflowId} />)}
    </Stack>
  );
}

// RunNote is how a run a message started is doing, in a line under it.
function RunNote({ run, workflowId }: { run?: ChatRun | typeof gone; workflowId: number }) {
  if (run === undefined || run === gone) return null;
  const logs = (
    <Link asChild color="inherit" textDecoration="underline" flexShrink={0}>
      <RouterLink to={`/workflow/${workflowId}?tab=runs`}>run {run.id}</RouterLink>
    </Link>
  );
  switch (run.status) {
    case RunStatus.RUNNING:
      return (
        <HStack gap="1.5" textStyle="xs" color="fg.muted" maxW="full" px="1">
          <Spinner size="xs" flexShrink={0} />
          <Text flexShrink={0}>Running{run.line ? ':' : '...'}</Text>
          {run.line && (
            <Text truncate fontFamily="mono">
              {run.line}
            </Text>
          )}
        </HStack>
      );
    case RunStatus.FAILED:
    case RunStatus.INTERRUPTED:
      return (
        <HStack gap="1.5" textStyle="xs" color={run.status === RunStatus.FAILED ? 'fg.error' : 'fg.warning'} maxW="full" px="1">
          <Badge size="xs" colorPalette={run.status === RunStatus.FAILED ? 'red' : 'orange'} flexShrink={0}>
            {run.status === RunStatus.FAILED ? 'Failed' : 'Interrupted'}
          </Badge>
          <Text truncate title={run.error}>
            {run.error}
          </Text>
          {logs}
        </HStack>
      );
  }
  return null;
}

function ChatMenu({ name, onRename, onDelete }: { name: string; onRename: () => void; onDelete: () => void }) {
  return (
    <Menu.Root
      positioning={{ placement: 'bottom-end' }}
      onSelect={(e) => (({ rename: onRename, delete: onDelete }) as Record<string, () => void>)[e.value]?.()}
    >
      <Menu.Trigger asChild>
        <IconButton aria-label={`Actions for ${name}`} title="Actions" size="xs" variant="ghost" color="fg.muted">
          <EllipsisVerticalIcon />
        </IconButton>
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content minW="36">
            <Menu.Item value="rename">
              <PencilIcon />
              Rename
            </Menu.Item>
            <Menu.Separator />
            <Menu.Item value="delete" color="fg.error" _hover={{ bg: 'bg.error', color: 'fg.error' }}>
              <Trash2Icon />
              Delete
            </Menu.Item>
          </Menu.Content>
        </Menu.Positioner>
      </Portal>
    </Menu.Root>
  );
}

// NewChatDialog starts a chat with a workflow that has a Tower Chat step.
function NewChatDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (c: ChatT) => void }) {
  const [list, setList] = useState<ChatWorkflow[]>();
  const [workflowId, setWorkflowId] = useState(0);
  const [name, setName] = useState('');
  useEffect(() => {
    chats.listChatWorkflows({}).then(
      (r) => {
        setList(r.workflows);
        if (r.workflows.length) setWorkflowId(r.workflows[0].id);
      },
      (err: unknown) => toast(errorText(err), true),
    );
  }, []);
  const picked = list?.find((w) => w.id === workflowId);
  return (
    <FormDialog
      title="New chat"
      submitText="Start chat"
      onClose={onClose}
      onSubmit={async () => {
        if (!workflowId) throw new Error('Pick a workflow with a Tower Chat step');
        const r = await chats.createChat({ workflowId, name: name.trim() });
        if (r.chat) onCreated(r.chat);
      }}
    >
      {list?.length === 0 ? (
        <Text textStyle="sm" color="fg.muted">
          No workflow has a Tower Chat step yet. In a workflow's editor, add Messengers › Tower Chat › Tower Chat, and a
          Tower Chat Send that takes its Chat, to answer.
        </Text>
      ) : (
        <>
          <FormField label="Workflow" help="Each message starts a run of its Tower Chat step, while it is switched on.">
            <NativeSelect.Root disabled={!list}>
              <NativeSelect.Field value={workflowId} onChange={(e) => setWorkflowId(Number(e.currentTarget.value))}>
                {list?.map((w) => (
                  <option key={w.id} value={w.id}>
                    {w.name}
                    {w.active ? '' : ' (off)'}
                  </option>
                ))}
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
            {picked?.description && (
              <Text textStyle="xs" color="fg.muted">
                {picked.description}
              </Text>
            )}
          </FormField>
          <FormField label="Name" help="Empty: the workflow's name.">
            <Input value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder={picked?.name} />
          </FormField>
        </>
      )}
    </FormDialog>
  );
}

function RenameChatDialog({ chat, onClose, onRenamed }: { chat: ChatT; onClose: () => void; onRenamed: (c: ChatT) => void }) {
  const [name, setName] = useState(chat.name);
  return (
    <FormDialog
      title={`Rename chat ${chat.name}`}
      submitText="Rename"
      onClose={onClose}
      onSubmit={async () => {
        const r = await chats.updateChat({ id: chat.id, name: name.trim() });
        if (r.chat) onRenamed(r.chat);
      }}
    >
      <FormField label="Name">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} />
      </FormField>
    </FormDialog>
  );
}

// when is a time as the chat shows it: the time today, else the date too.
function when(t?: Timestamp) {
  if (!t) return '';
  const d = timestampDate(t);
  const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  return d.toDateString() === new Date().toDateString()
    ? time
    : `${d.toLocaleDateString([], { day: 'numeric', month: 'short' })} ${time}`;
}

// markdownStyles lay out a workflow's Markdown in its bubble: compact, with
// code, tables and quotes set apart.
const markdownStyles = {
  '& > :first-child': { marginTop: 0 },
  '& > :last-child': { marginBottom: 0 },
  '& p, & ul, & ol, & pre, & blockquote, & table': { marginBlock: '0.5em' },
  '& ul': { paddingInlineStart: '1.25em', listStyleType: 'disc' },
  '& ol': { paddingInlineStart: '1.5em', listStyleType: 'decimal' },
  '& li + li': { marginTop: '0.15em' },
  '& h1, & h2, & h3, & h4, & h5, & h6': { fontWeight: 'semibold', marginBlock: '0.6em 0.3em' },
  '& h1': { fontSize: 'lg' },
  '& h2': { fontSize: 'md' },
  '& code': { fontFamily: 'mono', fontSize: '0.875em', bg: 'bg.emphasized', px: '1', rounded: 'sm' },
  '& pre': { bg: 'bg.emphasized', px: '3', py: '2', rounded: 'md', overflowX: 'auto', whiteSpace: 'pre' },
  '& pre code': { bg: 'transparent', px: '0' },
  '& a': { color: 'blue.fg', textDecoration: 'underline' },
  '& blockquote': { borderInlineStartWidth: '3px', borderColor: 'border.emphasized', paddingInlineStart: '3', color: 'fg.muted' },
  '& table': { display: 'block', overflowX: 'auto', borderCollapse: 'collapse' },
  '& th, & td': { borderWidth: '1px', borderColor: 'border.emphasized', px: '2', py: '1', textAlign: 'start' },
  '& th': { fontWeight: 'semibold' },
  '& hr': { marginBlock: '0.75em', borderColor: 'border.emphasized' },
};
