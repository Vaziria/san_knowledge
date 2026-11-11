import {
  Alert,
  Box,
  Code,
  CloseButton,
  Flex,
  Heading,
  IconButton,
  Image,
  Input,
  InputGroup,
  Kbd,
  Popover,
  Portal,
  SimpleGrid,
  Text,
} from '@chakra-ui/react';
import { SearchIcon, type LucideIcon } from 'lucide-react';
import { useCallback, useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router';
import { claudeHealth, cmpVersion, type ClaudeHealth } from './api';

export type Page = { path: string; label: string; icon: LucideIcon; element: ReactNode };

// Layout is the frame every page shares: the page over the whole window, and
// the menu behind a round San Tower button floating at the top left (the
// user, 2026-11-11: no sidebar; a menu button that shows the pages in a
// grid, with a search). The button, or Ctrl+K anywhere, opens and closes it.
export function Layout({ pages }: { pages: Page[] }) {
  const [open, setOpen] = useState(false);
  // opened counts the openings: each one starts a fresh Menu (empty search,
  // the current page marked), even while the last one is still fading out.
  const [opened, setOpened] = useState(0);
  const openNow = useRef(false);
  const show = useCallback((next: boolean) => {
    if (next && !openNow.current) setOpened((n) => n + 1);
    openNow.current = next;
    setOpen(next);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        show(!openNow.current);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [show]);

  return (
    <Box minH="100vh" bg="bg" color="fg">
      <Popover.Root
        open={open}
        onOpenChange={(e) => show(e.open)}
        positioning={{ placement: 'bottom-start', gutter: 8 }}
        lazyMount
        unmountOnExit
      >
        <Popover.Trigger asChild>
          <IconButton
            aria-label="Menu (Ctrl+K)"
            title="Menu (Ctrl+K)"
            position="fixed"
            top="3"
            left="3"
            zIndex="sticky"
            rounded="full"
            variant="outline"
            bg="bg.panel"
            shadow="md"
          >
            <Image src="/logo.svg" alt="" boxSize="6" />
          </IconButton>
        </Popover.Trigger>
        <Portal>
          <Popover.Positioner>
            <Popover.Content w="md" maxW="calc(100vw - 24px)">
              <Popover.Body p="3">
                <Menu key={opened} pages={pages} onDone={() => show(false)} />
              </Popover.Body>
            </Popover.Content>
          </Popover.Positioner>
        </Portal>
      </Popover.Root>
      {/* The page starts right of the button, which stays over the top left. */}
      <Box as="main" minW="0" p="6" ps="16">
        <ClaudeHealthBanner />
        <Outlet />
      </Box>
    </Box>
  );
}

// columns is how many pages the menu shows in a row.
const columns = 3;

// isCurrent says whether path is the page open now, or a page under it (a
// workflow marks Workflow).
function isCurrent(path: string, pathname: string) {
  return path === '/' ? pathname === '/' : pathname === path || pathname.startsWith(path + '/');
}

// Menu is the popup: a search box over the pages in a grid. The search keeps
// the pages whose name has every word typed. The arrows move the marked page
// (it starts on the page open now), Enter opens it; Escape and a click
// outside close the popup.
function Menu({ pages, onDone }: { pages: Page[]; onDone: () => void }) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [marked, setMarked] = useState(() => Math.max(0, pages.findIndex((p) => isCurrent(p.path, pathname))));

  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const shown = pages.filter((p) => words.every((w) => p.label.toLowerCase().includes(w)));
  const at = Math.min(marked, shown.length - 1);

  const onKeyDown = (e: ReactKeyboardEvent) => {
    const step = ({ ArrowRight: 1, ArrowLeft: -1, ArrowDown: columns, ArrowUp: -columns } as Record<string, number>)[
      e.key
    ];
    if (step !== undefined) {
      e.preventDefault();
      const next = at + step;
      if (next >= 0 && next < shown.length) setMarked(next);
      return;
    }
    if (e.key === 'Enter' && shown[at]) {
      e.preventDefault();
      navigate(shown[at].path);
      onDone();
    }
  };

  return (
    <>
      <Flex align="center" gap="2" mb="3">
        <Image src="/logo.svg" alt="" boxSize="6" />
        <Heading size="sm" flex="1">
          San Tower
        </Heading>
        <Kbd size="sm">Ctrl+K</Kbd>
      </Flex>
      <InputGroup startElement={<SearchIcon />} mb="3">
        <Input
          autoFocus
          size="sm"
          placeholder="Search pages"
          aria-label="Search pages"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setMarked(0);
          }}
          onKeyDown={onKeyDown}
        />
      </InputGroup>
      {shown.length === 0 ? (
        <Text color="fg.muted" textStyle="sm" textAlign="center" py="6">
          No page matches "{query}"
        </Text>
      ) : (
        <SimpleGrid as="nav" columns={columns} gap="2">
          {shown.map((p, i) => {
            const current = isCurrent(p.path, pathname);
            return (
              <Flex
                key={p.path}
                asChild
                direction="column"
                align="center"
                justify="center"
                gap="1.5"
                minH="20"
                px="1"
                py="3"
                rounded="md"
                borderWidth="1px"
                borderColor={i === at ? 'border.emphasized' : 'transparent'}
                bg={current ? 'bg.emphasized' : i === at ? 'bg.muted' : undefined}
                color={current ? 'fg' : 'fg.muted'}
                fontWeight={current ? 'semibold' : undefined}
                textAlign="center"
                _hover={{ color: 'fg' }}
                onMouseEnter={() => setMarked(i)}
              >
                <NavLink to={p.path} onClick={onDone} aria-current={current ? 'page' : undefined}>
                  <p.icon size={24} />
                  <Text textStyle="xs" lineHeight="short">
                    {p.label}
                  </Text>
                </NavLink>
              </Flex>
            );
          })}
        </SimpleGrid>
      )}
      <Text color="fg.subtle" textStyle="2xs" mt="3">
        Arrows to move · Enter to open · Esc to close
      </Text>
    </>
  );
}

// dismissKey keeps, per browser, the version the banner was last dismissed
// for: once claude is updated to a newer version, the banner comes back.
const dismissKey = 'san-tower-claude-banner-dismissed';

function loadDismissed(): string {
  try {
    return localStorage.getItem(dismissKey) ?? '';
  } catch {
    return '';
  }
}
function saveDismissed(v: string) {
  try {
    localStorage.setItem(dismissKey, v);
  } catch {
    // Storage blocked: the banner just comes back next time.
  }
}

// ClaudeHealthBanner polls /api/claude-health once on load and shows an
// orange banner when claude is missing or older than the minimum the Claude
// Ask (CLI) steps need (engine.ClaudeCLIMinVersion). Dismiss remembers which
// state the person waved off, so an older version still shows the banner when
// they come back later; a new install (or an update) makes it reappear.
function ClaudeHealthBanner() {
  const [health, setHealth] = useState<ClaudeHealth | undefined>();
  const [dismissed, setDismissed] = useState<string>(loadDismissed);

  useEffect(() => {
    claudeHealth().then(setHealth, () => undefined);
  }, []);

  if (!health) return null;
  const stale = !health.version || cmpVersion(health.version, health.minVersion) < 0;
  if (!stale) return null;
  // The dismiss state covers one shape: "missing" or a specific installed version.
  const stateKey = health.version || 'missing';
  if (dismissed === stateKey) return null;
  const dismiss = () => {
    saveDismissed(stateKey);
    setDismissed(stateKey);
  };

  return (
    <Alert.Root status="warning" mb="4">
      <Alert.Indicator />
      <Alert.Content>
        <Alert.Title>Claude Code needs updating</Alert.Title>
        <Alert.Description>
          {health.version ? (
            <>
              Claude Code <Code>{health.version}</Code> is installed; the Claude Ask (CLI) steps need{' '}
              <Code>{health.minVersion}</Code> or newer.{' '}
            </>
          ) : (
            <>Claude Code is not installed or not on PATH ({health.error}). </>
          )}
          Close every Claude Code window (including the one in VS Code), open a plain terminal and run{' '}
          <Code>winget upgrade Anthropic.ClaudeCode</Code> (Windows) or <Code>claude update</Code>, then start the tower
          again.
        </Alert.Description>
      </Alert.Content>
      <CloseButton pos="absolute" top="2" right="2" onClick={dismiss} />
    </Alert.Root>
  );
}
