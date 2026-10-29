import { Badge, Box, Flex, HStack, Input, Kbd, Portal, Stack, Text } from '@chakra-ui/react';
import type { XYPosition } from '@xyflow/react';
import {
  ActivityIcon,
  AppWindowIcon,
  ArrowRightLeftIcon,
  AsteriskIcon,
  AtSignIcon,
  BookOpenIcon,
  BookSearchIcon,
  BotIcon,
  BotMessageSquareIcon,
  BoxIcon,
  BrainIcon,
  BracesIcon,
  BriefcaseIcon,
  CalendarClockIcon,
  CalendarIcon,
  CalendarPlusIcon,
  CameraIcon,
  CalendarRangeIcon,
  CalendarSearchIcon,
  ChevronRightIcon,
  ChartCandlestickIcon,
  CheckCheckIcon,
  ChevronsUpIcon,
  CircleXIcon,
  ClapperboardIcon,
  ClockIcon,
  CoinsIcon,
  CropIcon,
  DownloadIcon,
  FileDownIcon,
  FileIcon,
  FileInputIcon,
  FileJsonIcon,
  FileTextIcon,
  FileXIcon,
  FilterIcon,
  FingerprintPatternIcon,
  FileSearchIcon,
  FolderOpenIcon,
  FolderSearchIcon,
  FolderXIcon,
  GemIcon,
  GitBranchIcon,
  GlobeIcon,
  HardDriveIcon,
  HandIcon,
  HashIcon,
  HistoryIcon,
  ImageDownIcon,
  ImagePlusIcon,
  LayersIcon,
  LibraryBigIcon,
  LightbulbIcon,
  ListIcon,
  LogsIcon,
  MegaphoneIcon,
  MessageCircleIcon,
  MessageSquareTextIcon,
  MessagesSquareIcon,
  MusicIcon,
  NewspaperIcon,
  NotebookPenIcon,
  OctagonXIcon,
  PlayIcon,
  RadioTowerIcon,
  RegexIcon,
  ScissorsIcon,
  ScrollTextIcon,
  SearchIcon,
  SendHorizontalIcon,
  SendIcon,
  Share2Icon,
  SparklesIcon,
  SquareTerminalIcon,
  TerminalIcon,
  TextQuoteIcon,
  TextSearchIcon,
  TimerIcon,
  Trash2Icon,
  UploadIcon,
  UsersIcon,
  VideoIcon,
  WalletIcon,
  ZapIcon,
  type LucideIcon,
} from 'lucide-react';
import { useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type MouseEvent, type Ref } from 'react';
import { categories, categoryPalette, stepKinds, type StepKind } from './catalog';

// Icons for the categories, the groups in them (Social › Threads), and the
// kinds that have one of their own; a kind or group without one shows its
// category's.
const categoryIcons: Record<string, LucideIcon> = {
  Trigger: ZapIcon,
  Logic: GitBranchIcon,
  Data: BracesIcon,
  Storage: HardDriveIcon,
  Datetime: CalendarIcon,
  Action: PlayIcon,
  Messengers: MessagesSquareIcon,
  Social: Share2Icon,
  Finance: ChartCandlestickIcon,
  Knowledge: BrainIcon,
  AI: SparklesIcon,
};
const groupIcons: Record<string, LucideIcon> = {
  Threads: AtSignIcon,
  X: MegaphoneIcon,
  Facebook: UsersIcon,
  Vidara: VideoIcon,
  Medium: BookOpenIcon,
  LinkedIn: BriefcaseIcon,
  TikTok: MusicIcon,
  Instagram: CameraIcon,
  'Tower Chat': MessageSquareTextIcon,
  Telegram: SendIcon,
  WhatsApp: MessageCircleIcon,
  Binance: CoinsIcon,
  ChatGPT: BotIcon,
  Gemini: GemIcon,
  Claude: AsteriskIcon,
};
const kindIcons: Record<string, LucideIcon> = {
  manual: HandIcon,
  'folder-watch': FolderOpenIcon,
  'file-watch': FileSearchIcon,
  'topic-event': RadioTowerIcon,
  schedule: ClockIcon,
  'log-event': LogsIcon,
  filter: FilterIcon,
  'prefix-filter': TextSearchIcon,
  'regex-filter': RegexIcon,
  'pick-greater': ChevronsUpIcon,
  template: FileTextIcon,
  'format-text': TextQuoteIcon,
  storage: HardDriveIcon,
  'file-path': FileIcon,
  'read-file': FileInputIcon,
  glob: FolderSearchIcon,
  'download-file': FileDownIcon,
  'cut-video': ScissorsIcon,
  'crop-video': CropIcon,
  'delete-file': FileXIcon,
  'delete-folder': FolderXIcon,
  uuid: FingerprintPatternIcon,
  md5: HashIcon,
  stringify: FileJsonIcon,
  now: TimerIcon,
  datetime: CalendarIcon,
  'datetime-operation': CalendarPlusIcon,
  'string-to-datetime': CalendarSearchIcon,
  'http-request': GlobeIcon,
  bash: TerminalIcon,
  publish: SendIcon,
  log: ScrollTextIcon,
  fail: OctagonXIcon,
  ack: CheckCheckIcon,
  'threads-post': AtSignIcon,
  'x-post': MegaphoneIcon,
  'x-download': ImageDownIcon,
  'facebook-post': UsersIcon,
  'vidara-upload': UploadIcon,
  'vidara-download': DownloadIcon,
  'medium-draft': NotebookPenIcon,
  'linkedin-draft': NewspaperIcon,
  'tiktok-post': ClapperboardIcon,
  'instagram-post': ImagePlusIcon,
  'instagram-delete': Trash2Icon,
  'chatgpt-ask': BotMessageSquareIcon,
  'gemini-ask': GemIcon,
  'claude-ask-cli': TerminalIcon,
  'claude-chat-session': HistoryIcon,
  'browser-chat-session': HistoryIcon,
  'remote-shell': SquareTerminalIcon,
  'claude-ask-browser': AppWindowIcon,
  'social-status': ActivityIcon,
  'social-posts': ListIcon,
  'last-scheduled-post': CalendarClockIcon,
  'post-count': CalendarRangeIcon,
  'tower-chat': MessageSquareTextIcon,
  'tower-chat-send': SendHorizontalIcon,
  'telegram-message': MessageCircleIcon,
  'telegram-send': SendIcon,
  'telegram-messages': MessagesSquareIcon,
  'binance-order': ArrowRightLeftIcon,
  'binance-candles': ChartCandlestickIcon,
  'binance-portfolio': WalletIcon,
  'binance-orders': HistoryIcon,
  'binance-positions': LayersIcon,
  'binance-close-position': CircleXIcon,
  'knowledge-base': LibraryBigIcon,
  'knowledge-search': BookSearchIcon,
  'knowledge-explain': LightbulbIcon,
};
const iconOf = (k: StepKind) => kindIcons[k.kind] ?? categoryIcons[k.category] ?? BoxIcon;

// A menu holds sub-menus and steps: the categories, and the groups in them.
type Menu = { name: string; icon: LucideIcon; palette: string; entries: Entry[] };
type Entry = { menu: Menu } | { kind: StepKind };

// tree is the menus of the Add palette: the categories in their order (one
// the UI does not know yet goes last), each with its groups first, in the
// order their steps come, then the steps that are in no group.
function tree(kinds: StepKind[]): Menu[] {
  const names = [...categories, ...new Set(kinds.map((k) => k.category).filter((c) => !categories.includes(c)))];
  return names
    .map((name) => {
      const inCategory = kinds.filter((k) => k.category === name);
      const palette = categoryPalette(name);
      const icon = categoryIcons[name] ?? BoxIcon;
      const groups = [...new Set(inCategory.map((k) => k.group).filter(Boolean))].map((group) => ({
        menu: {
          name: group,
          icon: groupIcons[group] ?? icon,
          palette,
          entries: inCategory.filter((k) => k.group === group).map((kind) => ({ kind })),
        },
      }));
      const loose = inCategory.filter((k) => !k.group).map((kind) => ({ kind }));
      return { name, icon, palette, entries: [...groups, ...loose] };
    })
    .filter((m) => m.entries.length);
}

// search finds the kinds whose title, description, category, group or kind
// has every word of the query, those whose title starts with it first.
function search(kinds: StepKind[], query: string) {
  const q = query.trim().toLowerCase();
  const words = q.split(/\s+/);
  const found = kinds.filter((k) => {
    const text = `${k.title} ${k.description} ${k.category} ${k.group} ${k.kind}`.toLowerCase();
    return words.every((w) => text.includes(w));
  });
  const rank = (k: StepKind) => (k.title.toLowerCase().startsWith(q) ? 0 : k.title.toLowerCase().includes(q) ? 1 : 2);
  return found.sort((a, b) => rank(a) - rank(b));
}

const rootWidth = 220;
const columnWidth = 420;

// AddPalette is the Add menu, as Blender's Shift+A: a search field that has
// the keys at once, and menus that open beside each other, category ›
// group › step (Social › Threads › Threads Post), each step with its icon,
// name and description. Hovering a menu, or moving to it, shows what is in
// it in the next column. Typing searches every step. ↑↓ move, → or Enter
// open a menu, ← goes back, Enter adds a step, Escape or a click outside
// closes. It opens with its top-left corner at at (screen coordinates),
// moved in to stay on screen.
export function AddPalette({
  at,
  onPick,
  onClose,
}: {
  at: XYPosition;
  onPick: (kind: string) => void;
  onClose: () => void;
}) {
  const all = stepKinds();
  const root = useMemo(() => tree(all), [all]);
  const [query, setQuery] = useState('');
  // path is the entry picked in each column so far; depth the column the
  // keys move in. The column after it shows what the picked menu holds.
  const [path, setPath] = useState<number[]>([0]);
  const [depth, setDepth] = useState(0);
  const [found, setFound] = useState(0);
  const searching = query.trim() !== '';
  const results = searching ? search(all, query) : [];

  const columns: Entry[][] = [root.map((menu) => ({ menu }))];
  for (let c = 0; c <= depth; c++) {
    const picked = columns[c]?.[path[c] ?? 0];
    if (!picked || !('menu' in picked)) break;
    columns.push(picked.menu.entries);
  }

  const panel = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState(at);
  useLayoutEffect(() => {
    const r = panel.current!.getBoundingClientRect();
    setPos({
      x: Math.max(8, Math.min(at.x, window.innerWidth - r.width - 8)),
      y: Math.max(8, Math.min(at.y, window.innerHeight - r.height - 8)),
    });
  }, [at, columns.length, searching]);
  const active = useRef<HTMLDivElement>(null);
  // Braces: scrollIntoView returns a promise in newer browsers, and an effect
  // must not return one.
  useLayoutEffect(() => {
    active.current?.scrollIntoView({ block: 'nearest' });
  }, [path, depth, found, query]);

  // The pointer selects only once it moves, as in Blender: the palette opens,
  // or moves in to stay on screen, under a pointer that is still, which must
  // not take the selection from the keys.
  const pointer = useRef<{ x: number; y: number; moved: boolean } | null>(null);
  const hovering = (e: MouseEvent) => {
    const p = pointer.current;
    if (!p) {
      pointer.current = { x: e.clientX, y: e.clientY, moved: false };
      return false;
    }
    if (e.clientX !== p.x || e.clientY !== p.y) p.moved = true;
    return p.moved;
  };

  const pick = (k?: StepKind) => {
    if (!k) return;
    onPick(k.kind);
    onClose();
  };
  // choose takes entry i of column c: a menu opens in the next column, a step
  // is added.
  const choose = (c: number, i: number) => {
    const e = columns[c]?.[i];
    if (!e) return;
    if ('kind' in e) return pick(e.kind);
    setPath([...path.slice(0, c), i, 0]);
    setDepth(c + 1);
  };
  const onKey = (e: KeyboardEvent) => {
    const wrap = (n: number, i: number) => (n <= 0 ? 0 : ((i % n) + n) % n);
    const d = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0;
    if (e.key === 'Escape') onClose();
    else if (searching) {
      if (d) setFound((i) => wrap(results.length, i + d));
      else if (e.key === 'Enter') pick(results[found]);
      else return;
    } else if (d) setPath([...path.slice(0, depth), wrap(columns[depth].length, (path[depth] ?? 0) + d)]);
    else if (e.key === 'ArrowRight' || e.key === 'Enter') {
      const entry = columns[depth]?.[path[depth] ?? 0];
      if (e.key === 'ArrowRight' && entry && 'kind' in entry) return;
      choose(depth, path[depth] ?? 0);
    } else if (e.key === 'ArrowLeft') {
      if (depth === 0) return;
      setPath(path.slice(0, depth));
      setDepth(depth - 1);
    } else return;
    e.preventDefault();
  };

  return (
    <Portal>
      {/* A click anywhere else closes it. */}
      <Box
        position="fixed"
        inset="0"
        zIndex="popover"
        onMouseDown={onClose}
        onContextMenu={(e) => e.preventDefault()}
      />
      <Stack
        ref={panel}
        position="fixed"
        left={`${pos.x}px`}
        top={`${pos.y}px`}
        zIndex="popover"
        gap="0"
        bg="bg.panel"
        borderWidth="1px"
        rounded="l3"
        shadow="xl"
        overflow="hidden"
        w={searching ? `${columnWidth + 120}px` : `${rootWidth + (columns.length - 1) * columnWidth}px`}
        maxW="calc(100vw - 16px)"
      >
        <HStack px="3" py="2" gap="2" borderBottomWidth="1px">
          <SearchIcon size="16" color="var(--chakra-colors-fg-muted)" />
          <Input
            autoFocus
            variant="flushed"
            size="sm"
            border="none"
            placeholder="Search steps..."
            value={query}
            onChange={(e) => {
              setQuery(e.currentTarget.value);
              setFound(0);
            }}
            onKeyDown={onKey}
          />
        </HStack>
        <Flex h="min(40rem, 70vh)" minH="0">
          {searching ? (
            <Stack flex="1" minW="0" gap="1" p="2" overflowY="auto">
              {results.length === 0 && (
                <Text textStyle="sm" color="fg.muted" p="3">
                  No step matches "{query.trim()}".
                </Text>
              )}
              {results.map((k, i) => (
                <KindRow
                  key={k.kind}
                  kind={k}
                  where={k.group ? `${k.category} › ${k.group}` : k.category}
                  on={i === found}
                  activeRef={i === found ? active : undefined}
                  onHover={(e) => hovering(e) && setFound(i)}
                  onPick={() => pick(k)}
                />
              ))}
            </Stack>
          ) : (
            columns.map((entries, c) => (
              <Stack
                key={c}
                w={c === 0 ? `${rootWidth}px` : `${columnWidth}px`}
                flexShrink={0}
                gap="1"
                p="2"
                borderEndWidth={c < columns.length - 1 ? '1px' : undefined}
                bg={c > depth ? 'bg.subtle' : undefined}
                overflowY="auto"
              >
                {entries.map((e, i) => {
                  const on = c <= depth && i === (path[c] ?? 0);
                  const hover = (ev: MouseEvent) => {
                    if (!hovering(ev) || (on && c === depth)) return;
                    setPath([...path.slice(0, c), i]);
                    setDepth(c);
                  };
                  return 'menu' in e ? (
                    <MenuRow
                      key={e.menu.name}
                      menu={e.menu}
                      on={on}
                      current={on && c === depth}
                      activeRef={on && c === depth ? active : undefined}
                      onHover={hover}
                      onPick={() => choose(c, i)}
                    />
                  ) : (
                    <KindRow
                      key={e.kind.kind}
                      kind={e.kind}
                      on={on}
                      activeRef={on ? active : undefined}
                      onHover={hover}
                      onPick={() => pick(e.kind)}
                    />
                  );
                })}
              </Stack>
            ))
          )}
        </Flex>
        <HStack px="3" py="1.5" gap="3" borderTopWidth="1px" textStyle="xs" color="fg.muted">
          <HStack gap="1">
            <Kbd size="sm">↑</Kbd>
            <Kbd size="sm">↓</Kbd> move
          </HStack>
          {!searching && (
            <HStack gap="1">
              <Kbd size="sm">←</Kbd>
              <Kbd size="sm">→</Kbd> back / open
            </HStack>
          )}
          <HStack gap="1">
            <Kbd size="sm">Enter</Kbd> add
          </HStack>
          <HStack gap="1">
            <Kbd size="sm">Esc</Kbd> close
          </HStack>
          <Text ms="auto" textStyle="xs">
            Type to search every step
          </Text>
        </HStack>
      </Stack>
    </Portal>
  );
}

// MenuRow is a category or a group: its icon, name, how many steps it holds,
// and an arrow to what is in it. on: it is open, its steps in the next
// column; current: the keys are on it.
function MenuRow({
  menu,
  on,
  current,
  activeRef,
  onHover,
  onPick,
}: {
  menu: Menu;
  on: boolean;
  current: boolean;
  activeRef?: Ref<HTMLDivElement>;
  onHover: (e: MouseEvent) => void;
  onPick: () => void;
}) {
  const count = (m: Menu): number => m.entries.reduce((n, e) => n + ('menu' in e ? count(e.menu) : 1), 0);
  return (
    <HStack
      ref={activeRef}
      role="option"
      aria-selected={on}
      px="2"
      py="1.5"
      gap="2"
      rounded="l2"
      cursor="pointer"
      bg={on ? (current ? 'bg.emphasized' : 'bg.muted') : undefined}
      onMouseMove={onHover}
      onClick={onPick}
    >
      <Box color={`${menu.palette}.fg`}>
        <menu.icon size="16" />
      </Box>
      <Text flex="1" textStyle="sm" fontWeight={on ? 'medium' : undefined}>
        {menu.name}
      </Text>
      <Text textStyle="xs" color="fg.muted">
        {count(menu)}
      </Text>
      <ChevronRightIcon size="14" color="var(--chakra-colors-fg-subtle)" />
    </HStack>
  );
}

// KindRow is a step to add: its icon in its category's colour, its name and
// what it does; where says which menu it is in, in search results.
function KindRow({
  kind,
  where,
  on,
  activeRef,
  onHover,
  onPick,
}: {
  kind: StepKind;
  where?: string;
  on: boolean;
  activeRef?: Ref<HTMLDivElement>;
  onHover: (e: MouseEvent) => void;
  onPick: () => void;
}) {
  const Icon = iconOf(kind);
  const palette = categoryPalette(kind.category);
  return (
    <HStack
      ref={activeRef}
      role="option"
      aria-selected={on}
      align="flex-start"
      px="2"
      py="2"
      gap="3"
      rounded="l2"
      cursor="pointer"
      bg={on ? 'bg.emphasized' : undefined}
      onMouseMove={onHover}
      onClick={onPick}
    >
      <Flex
        flexShrink={0}
        w="8"
        h="8"
        rounded="l2"
        align="center"
        justify="center"
        bg={`${palette}.subtle`}
        color={`${palette}.fg`}
      >
        <Icon size="16" />
      </Flex>
      <Stack gap="0.5" minW="0" flex="1">
        <HStack gap="2" wrap="wrap">
          <Text textStyle="sm" fontWeight="medium">
            {kind.title}
          </Text>
          {where && (
            <Badge size="xs" variant="subtle" colorPalette={palette}>
              {where}
            </Badge>
          )}
          {!kind.runnable && (
            <Badge size="xs" variant="outline">
              soon
            </Badge>
          )}
        </HStack>
        {kind.description && (
          <Text textStyle="xs" color="fg.muted" lineClamp={2}>
            {kind.description}
          </Text>
        )}
      </Stack>
    </HStack>
  );
}
