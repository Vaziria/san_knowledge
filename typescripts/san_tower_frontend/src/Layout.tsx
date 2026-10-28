import { Alert, Box, Button, Code, CloseButton, Flex, Heading, IconButton, Image, Stack } from '@chakra-ui/react';
import { PanelLeftCloseIcon, PanelLeftOpenIcon, type LucideIcon } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { NavLink, Outlet } from 'react-router';
import { claudeHealth, cmpVersion, type ClaudeHealth } from './api';

export type Page = { path: string; label: string; icon: LucideIcon; element: ReactNode };

// collapsedKey keeps, per browser, whether the sidebar is collapsed.
const collapsedKey = 'san-tower-sidebar-collapsed';

function loadCollapsed() {
  try {
    return localStorage.getItem(collapsedKey) === '1';
  } catch {
    return false;
  }
}

function saveCollapsed(collapsed: boolean) {
  try {
    localStorage.setItem(collapsedKey, collapsed ? '1' : '0');
  } catch {
    // Storage blocked: the sidebar just starts expanded next time.
  }
}

// Layout is the frame every page shares: the menu in a sidebar on the left,
// and the current page beside it. The sidebar collapses to its icons.
export function Layout({ pages }: { pages: Page[] }) {
  const [collapsed, setCollapsed] = useState(loadCollapsed);
  const toggle = () => {
    setCollapsed(!collapsed);
    saveCollapsed(!collapsed);
  };

  return (
    <Flex minH="100vh" bg="bg" color="fg">
      <Stack
        as="aside"
        w={collapsed ? '16' : '56'}
        transition="width 0.15s"
        flexShrink={0}
        h="100vh"
        position="sticky"
        top="0"
        overflowX="hidden"
        overflowY="auto"
        gap="4"
        p="3"
        bg="bg.subtle"
        borderEndWidth="1px"
      >
        <Flex align="center" justify="space-between" direction={collapsed ? 'column' : 'row'} gap="2">
          <Flex align="center" gap="2" px={collapsed ? '0' : '2.5'}>
            <Image src="/logo.svg" alt="San Tower" boxSize="7" />
            {!collapsed && (
              <Heading size="md" whiteSpace="nowrap">
                San Tower
              </Heading>
            )}
          </Flex>
          <IconButton
            aria-label={collapsed ? 'Expand the menu' : 'Collapse the menu'}
            title={collapsed ? 'Expand the menu' : 'Collapse the menu'}
            variant="ghost"
            size="sm"
            color="fg.muted"
            onClick={toggle}
          >
            {collapsed ? <PanelLeftOpenIcon /> : <PanelLeftCloseIcon />}
          </IconButton>
        </Flex>
        <Stack as="nav" gap="1">
          {pages.map((p) => (
            <Button
              key={p.path}
              asChild
              variant="ghost"
              justifyContent={collapsed ? 'center' : 'flex-start'}
              px={collapsed ? '0' : undefined}
              whiteSpace="nowrap"
              color="fg.muted"
              _currentPage={{ bg: 'bg.emphasized', color: 'fg', fontWeight: 'semibold' }}
            >
              <NavLink to={p.path} end={p.path === '/'} title={collapsed ? p.label : undefined} aria-label={p.label}>
                <p.icon />
                {!collapsed && p.label}
              </NavLink>
            </Button>
          ))}
        </Stack>
      </Stack>
      <Box as="main" flex="1" minW="0" p="6">
        <ClaudeHealthBanner />
        <Outlet />
      </Box>
    </Flex>
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
