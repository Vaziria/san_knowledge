import {
  Badge,
  Box,
  HoverCard,
  HStack,
  Input,
  NativeSelect,
  Portal,
  Spacer,
  Stack,
  Text,
  Textarea,
} from '@chakra-ui/react';
import { Handle, Position, useNodeConnections, useReactFlow, type NodeProps } from '@xyflow/react';
import { useEffect, useState, type ReactNode } from 'react';
import {
  categoryPalette,
  handleId,
  loadConnections,
  loadStorages,
  socketColor,
  stepKind,
  type Setting,
  type Socket,
  type Step,
} from './catalog';
import type { StepRun } from '../gen/san/tower/v1/workflow_pb';
import { LogLine, stepDuration, useStepRun } from './runs';

// StepNode draws a step the way Blender draws a node: a header coloured by
// category, then the outputs (sockets on the right), the settings, and the
// inputs (sockets on the left). A kind the engine cannot run yet says "soon".
// When the Runs panel shows a run, the header says what the step did in it.
// Form controls carry the nodrag class, so typing or picking in them does not
// move the step.
export function StepNode({ id, data, selected }: NodeProps<Step>) {
  const { updateNodeData } = useReactFlow<Step>();
  const ran = useStepRun(id);
  const kind = stepKind(data.kind);
  if (!kind) {
    return (
      <Box w="56" p="3" bg="bg.panel" rounded="md" borderWidth="1px" borderColor="border.error" fontSize="xs">
        <Text color="fg.error">Unknown step: {data.kind}</Text>
      </Box>
    );
  }
  const palette = categoryPalette(kind.category);
  const set = (name: string, value: string) => updateNodeData(id, { values: { ...data.values, [name]: value } });

  return (
    <Box
      w="56"
      bg={{ base: 'bg.panel', _dark: 'bg.muted' }}
      rounded="md"
      borderWidth="1px"
      shadow="md"
      fontSize="xs"
      outline="2px solid"
      outlineColor={selected ? 'orange.solid' : 'transparent'}
    >
      <HStack px="3" py="1" gap="1" roundedTop="md" bg={`${palette}.solid`} color={`${palette}.contrast`}>
        <Text fontWeight="semibold">{kind.title}</Text>
        {!kind.runnable && (
          <Text opacity={0.75} title="The engine cannot run this kind of step yet">
            · soon
          </Text>
        )}
        <Spacer />
        {ran && <RunBadge ran={ran} />}
      </HStack>
      <Stack gap="1" py="2">
        {kind.outputs.map((s) => (
          <SocketRow key={s.name} socket={s} side="out">
            <Text>{s.name}</Text>
          </SocketRow>
        ))}
        {kind.settings.map((s) => (
          <SettingField key={s.name} setting={s} value={data.values[s.name] ?? ''} onChange={(v) => set(s.name, v)} />
        ))}
        {kind.inputs.map((s) => (
          <InputRow
            key={s.name}
            socket={s}
            shown={holds(s.when, kind.settings, data.values)}
            value={data.values[s.name] ?? ''}
            onChange={(v) => set(s.name, v)}
          />
        ))}
      </Stack>
    </Box>
  );
}

// RunBadge says what the step did in the picked run: its duration, or
// "failed". Hovering it shows its error and every line it logged in full (a
// native tooltip cuts long ones off), in a card outside the canvas, so it is
// not zoomed and its text can be selected.
function RunBadge({ ran }: { ran: StepRun }) {
  const badge = (
    <Badge size="xs" variant="solid" colorPalette={ran.error ? 'red' : 'gray'}>
      {ran.error ? 'failed' : stepDuration(ran)}
    </Badge>
  );
  if (!ran.error && ran.logs.length === 0) return badge;
  return (
    <HoverCard.Root openDelay={150} closeDelay={200} positioning={{ placement: 'top' }} lazyMount unmountOnExit>
      <HoverCard.Trigger asChild>{badge}</HoverCard.Trigger>
      <Portal>
        <HoverCard.Positioner>
          <HoverCard.Content w="md" maxW="90vw" maxH="min(60vh, var(--available-height))" overflowY="auto" p="3">
            <Stack gap="1" fontSize="xs">
              {ran.error && (
                <Text color="fg.error" whiteSpace="pre-wrap" overflowWrap="anywhere">
                  {ran.error}
                </Text>
              )}
              {ran.logs.map((l, i) => (
                <LogLine key={i} text={l} level={ran.logLevels[i]} display="block" />
              ))}
              <Text color="fg.muted">{stepDuration(ran)}</Text>
            </Stack>
          </HoverCard.Content>
        </HoverCard.Positioner>
      </Portal>
    </HoverCard.Root>
  );
}

function SocketRow({ socket, side, children }: { socket: Socket; side: 'in' | 'out'; children: ReactNode }) {
  return (
    <HStack position="relative" minH="6" px="3" justify={side === 'out' ? 'flex-end' : 'flex-start'}>
      {children}
      <Handle
        id={handleId(socket)}
        type={side === 'out' ? 'source' : 'target'}
        position={side === 'out' ? Position.Right : Position.Left}
        style={{ width: 11, height: 11, background: socketColor(socket.type), border: '1.5px solid #1d1d1d' }}
      />
    </HStack>
  );
}

// holds says whether an input's when ("Source=File", "Media!=None",
// "Format=Plain,Source=Text") holds: for each condition, the setting's
// value, or its first option while it has none, is that value (or, with !=,
// is not).
function holds(when: string, settings: Setting[], values: Record<string, string>) {
  if (!when) return true;
  return when.split(',').every((cond) => {
    const not = cond.includes('!=');
    const [name, want] = cond.split(not ? '!=' : '=');
    const value = values[name] || settings.find((s) => s.name === name)?.options[0] || '';
    return not ? value !== want : value === want;
  });
}

// InputRow is an input socket. A field input that nothing is linked to shows
// its value to type in, growing with its lines up to about ten, then
// scrolling (nowheel, so the wheel scrolls it rather than zooming); once
// linked, only its name. One that is not shown (its when does not hold) is
// left out, unless it is linked, so its link still has an end.
function InputRow({
  socket,
  shown,
  value,
  onChange,
}: {
  socket: Socket;
  shown: boolean;
  value: string;
  onChange: (v: string) => void;
}) {
  const linked = useNodeConnections({ handleType: 'target', handleId: handleId(socket) }).length > 0;
  if (!shown && !linked) return null;
  return (
    <SocketRow socket={socket} side="in">
      {socket.field && !linked ? (
        <FieldText socket={socket} value={value} onChange={onChange} />
      ) : (
        <Text>{socket.name}</Text>
      )}
    </SocketRow>
  );
}

// pickerTypes are the HTML inputs of a field input's picker. A picked
// datetime is 2026-10-25T09:00, a date 2026-10-25, a time 09:00; a value
// in another form (typed before, or RFC 3339) shows as empty.
const pickerTypes: Record<string, string> = { datetime: 'datetime-local', date: 'date', time: 'time' };

function FieldText({ socket, value, onChange }: { socket: Socket; value: string; onChange: (v: string) => void }) {
  const typing = useTyping(value, onChange);
  if (pickerTypes[socket.picker]) {
    // A picker shows no placeholder, so its name goes before it.
    return (
      <>
        <Text color="fg.muted" flexShrink={0}>
          {socket.name}
        </Text>
        <Input
          className="nodrag"
          size="2xs"
          type={pickerTypes[socket.picker]}
          css={{ colorScheme: 'light dark' }}
          {...typing}
        />
      </>
    );
  }
  return (
    <Textarea
      className="nodrag nowheel"
      size="xs"
      py="1"
      rows={1}
      autoresize
      maxH="40"
      placeholder={socket.name}
      {...typing}
    />
  );
}

// useTyping gives a text box its text while it has the focus. A step's
// values come back through React Flow's store, and on the way the box got
// its old text back for a moment and then the new one, which moves the
// cursor to the end: typing in the middle went to the end. So while it has
// the focus the box shows what is typed in it, and the step's value, which
// undo and redo change, again once it has not.
function useTyping(value: string, onChange: (v: string) => void) {
  const [typed, setTyped] = useState<string | null>(null);
  return {
    value: typed ?? value,
    onFocus: () => setTyped(value),
    onBlur: () => setTyped(null),
    onChange: (e: { target: { value: string } }) => {
      setTyped(e.target.value);
      onChange(e.target.value);
    },
  };
}

function SettingText({ setting, value, onChange }: { setting: Setting; value: string; onChange: (v: string) => void }) {
  return <Input className="nodrag" size="2xs" placeholder={setting.placeholder} {...useTyping(value, onChange)} />;
}

function SettingField({
  setting,
  value,
  onChange,
}: {
  setting: Setting;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <HStack px="3" gap="2">
      <Text color="fg.muted" flexShrink={0}>
        {setting.name}
      </Text>
      {setting.connectionType ? (
        <PickSelect
          what="connection"
          load={() => loadConnections(setting.connectionType)}
          loadKey={setting.connectionType}
          value={value}
          onChange={onChange}
        />
      ) : setting.storage ? (
        <PickSelect
          what="storage"
          load={loadStorages}
          loadKey="storage"
          none={setting.placeholder}
          value={value}
          onChange={onChange}
        />
      ) : setting.options.length > 0 ? (
        <NativeSelect.Root size="xs" className="nodrag">
          <NativeSelect.Field value={value} onChange={(e) => onChange(e.target.value)}>
            {/* A value no longer offered still shows, so the step says what it is set to. */}
            {(setting.options.includes(value) ? setting.options : [value, ...setting.options]).map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      ) : (
        <SettingText setting={setting} value={value} onChange={onChange} />
      )}
    </HStack>
  );
}

// PickSelect picks a connection (of one type) or a storage; the value is its
// id. One that is gone still shows, so the step says what it was set to.
// load runs again when loadKey changes. none names the empty choice when
// picking nothing is allowed (File Path: no storage).
function PickSelect({
  what,
  load,
  loadKey,
  none,
  value,
  onChange,
}: {
  what: 'connection' | 'storage';
  load: () => Promise<{ id: number; name: string }[]>;
  loadKey: string;
  none?: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const [list, setList] = useState<{ id: number; name: string }[]>();
  useEffect(() => {
    let stale = false;
    load().then(
      (l) => !stale && setList(l),
      () => !stale && setList([]),
    );
    return () => {
      stale = true;
    };
    // load is new on each render; loadKey says what it loads.
  }, [loadKey]);
  const gone = value !== '' && list !== undefined && !list.some((c) => String(c.id) === value);
  const Name = what === 'connection' ? 'Connection' : 'Storage';
  return (
    <NativeSelect.Root size="xs" className="nodrag">
      <NativeSelect.Field value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">
          {list === undefined ? 'Loading...' : none || (list.length > 0 ? `Pick a ${what}` : `No ${what}s yet`)}
        </option>
        {list?.map((c) => (
          <option key={c.id} value={String(c.id)}>
            {c.name}
          </option>
        ))}
        {gone && (
          <option value={value}>
            {Name} {value} (gone)
          </option>
        )}
      </NativeSelect.Field>
      <NativeSelect.Indicator />
    </NativeSelect.Root>
  );
}
