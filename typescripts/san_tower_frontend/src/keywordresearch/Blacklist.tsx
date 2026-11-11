import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  Box,
  Button,
  ButtonGroup,
  Checkbox,
  Dialog,
  HStack,
  IconButton,
  Input,
  InputGroup,
  Link,
  Pagination,
  Portal,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
} from '@chakra-ui/react';
import { BanIcon, ChevronLeftIcon, ChevronRightIcon, SearchIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { errorText, keywordResearch } from '../api';
import type { KeywordBlacklistEntry, KeywordResearch } from '../gen/san/tower/v1/keyword_research_pb';
import { FormDialog, toast } from '../ui';

// splitEntries is what was typed in the blacklist's box as entries: one per
// line or comma separated, trimmed, the empty ones left out.
export const splitEntries = (text: string) =>
  text
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean);

// copyText puts text on the clipboard: through navigator.clipboard where
// the page may use it (localhost, https), else through a hidden text box's
// copy (the tower opened over LAN http).
export async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    return;
  } catch {
    // Not a secure page: the old way.
  }
  const box = document.createElement('textarea');
  box.value = text;
  box.style.position = 'fixed';
  box.style.opacity = '0';
  document.body.append(box);
  box.select();
  const ok = document.execCommand('copy');
  box.remove();
  if (!ok) throw new Error('the browser did not let the page copy');
}

// plural is n and a word, in its plural (words: an s added) when n is not 1.
export const plural = (n: number, word: string, words = `${word}s`) => `${n.toLocaleString()} ${n === 1 ? word : words}`;

type Preview = { added: string[]; removed: number; matched: number } | { error: string };

// MATCHING says how blacklist entries match keywords, for the page.
export const MATCHING =
  "An entry matches a keyword when the entry's words are in it as whole words, in order: 'kode pos' matches " +
  "'kode pos', 'kode pos jakarta' and 'cek kode pos', but 'pos' does not match 'posisi'. Adding one removes the " +
  'keywords it matches and, unless you keep them, their children: those found only through them (one also found ' +
  'from a keyword that stays, stays). A crawl never adds a matching keyword again. Seed keywords cannot be ' +
  'blacklisted.';

// GONE is what the page says of deleting entries.
const GONE = "The keywords they removed don't come back; a later lookup may find them again.";

// PER_PAGE is how many entries a page of the list shows (the user,
// 2026-11-12: "is blacklist paginated ?").
const PER_PAGE = 40;

// BlacklistPanel is a research page's Blacklist tab (the user, 2026-11-12:
// "Add tab blacklist keyword"). At a readable width: how matching works and
// a box to add entries, one per line or comma separated, with "Also remove
// their children" (on by default; off blacklists only the keywords: the
// user, 2026-11-12: "in blacklist, ask blacklist selected keyword or
// selected keyword and child") and how many keywords adding them would
// remove (a dry run of AddToBlacklist, asked again as the typing pauses, the
// box is ticked and the graph changes). Then the entries, the newest first,
// over the page's whole width: a search that filters them, 40 a page (the
// user: "is blacklist paginated ?"), in two columns on a wide screen ("if
// screen to wide, make it two grid in blacklist keyword"), each a compact
// row: a checkbox, the entry, how many keywords it removed and when it was
// added, a delete button. Select all on this page, Select all N matching,
// Delete selected (N) ("and add delete selected"; asks for more than one)
// and Delete all ("add button delete all"; asks) take them out in one call.
// The selection clears after a delete, and entries gone (deleted in another
// tab) drop out of it. Deleting entries does not bring back their keywords.
export function BlacklistPanel({ research, keywords }: { research: KeywordResearch; keywords: number }) {
  const [text, setText] = useState('');
  const [children, setChildren] = useState(true);
  const [preview, setPreview] = useState<Preview>();
  const [counting, setCounting] = useState(false);
  const [adding, setAdding] = useState(false);
  const [find, setFind] = useState('');
  const [page, setPage] = useState(1);
  const [picked, setPicked] = useState<ReadonlySet<string>>(new Set());
  const [asking, setAsking] = useState<'all' | 'selected'>();
  const entries = useMemo(() => splitEntries(text), [text]);
  const key = entries.join('\n');

  useEffect(() => {
    setPreview(undefined);
    if (!entries.length) return;
    let stale = false;
    setCounting(true);
    const t = setTimeout(async () => {
      try {
        const r = await keywordResearch.addToBlacklist({ id: research.id, entries, dryRun: true, keepChildren: !children });
        if (!stale) setPreview({ added: r.added, removed: r.removed.length, matched: r.matched });
      } catch (err) {
        if (!stale) setPreview({ error: errorText(err) });
      } finally {
        if (!stale) setCounting(false);
      }
    }, 350);
    return () => {
      stale = true;
      clearTimeout(t);
      setCounting(false);
    };
  }, [key, children, research.id, keywords, research.blacklist.length]);

  const add = async () => {
    setAdding(true);
    try {
      const r = await keywordResearch.addToBlacklist({ id: research.id, entries, keepChildren: !children });
      setText('');
      toast(
        r.added.length
          ? `Blacklisted ${r.added.map((e) => `'${e}'`).join(', ')}: removed ${plural(r.removed.length, 'keyword')}`
          : 'Already in the blacklist',
      );
    } catch (err) {
      toast(`Not blacklisted: ${errorText(err)}`, true);
    } finally {
      setAdding(false);
    }
  };

  // The entries, the newest first, those the search finds, and this page's.
  const all = useMemo(() => [...research.blacklist].reverse(), [research.blacklist]);
  const q = find.trim().toLowerCase();
  const matching = useMemo(() => (q ? all.filter((e) => e.entry.includes(q)) : all), [all, q]);
  const last = Math.max(1, Math.ceil(matching.length / PER_PAGE));
  const at = Math.min(page, last);
  const shown = matching.slice((at - 1) * PER_PAGE, at * PER_PAGE);
  // A new search starts at the first page.
  useEffect(() => setPage(1), [q]);
  // Entries gone (deleted here or in another tab) drop out of the selection.
  useEffect(() => {
    setPicked((p) => {
      const have = new Set(all.map((e) => e.entry));
      const next = new Set([...p].filter((e) => have.has(e)));
      return next.size === p.size ? p : next;
    });
  }, [all]);
  const pick = (names: string[], on: boolean) =>
    setPicked((p) => {
      const next = new Set(p);
      for (const n of names) {
        if (on) next.add(n);
        else next.delete(n);
      }
      return next;
    });
  const pageNames = shown.map((e) => e.entry);
  const pagePicked = pageNames.filter((n) => picked.has(n)).length;
  const allMatchingPicked = matching.length > 0 && matching.every((e) => picked.has(e.entry));

  // remove takes entries out in one call (everything: every one); the
  // selection clears after.
  const remove = async (list: string[], everything = false) => {
    const r = await keywordResearch.removeFromBlacklist(
      everything
        ? { id: research.id, all: true }
        : list.length === 1
          ? { id: research.id, entry: list[0] }
          : { id: research.id, entries: list },
    );
    setPicked(new Set());
    toast(
      list.length === 1 && !everything
        ? `'${list[0]}' is out of the blacklist: the keywords it removed stay out until a lookup finds them again`
        : `Deleted ${plural(r.deleted || list.length, 'entry', 'entries')} from the blacklist`,
    );
  };
  const removeOne = async (entry: string) => {
    try {
      await remove([entry]);
    } catch (err) {
      toast(`Not deleted: ${errorText(err)}`, true);
    }
  };
  const selected = [...picked];
  const deleteSelected = () => {
    if (selected.length > 1) setAsking('selected');
    else if (selected.length === 1) void removeOne(selected[0]);
  };

  const ready = preview && !('error' in preview) && preview.added.length > 0;
  return (
    <Stack gap="4" flex="1" minH="0" overflowY="auto" pb="4">
      <Stack gap="2" maxW="3xl">
        <Text textStyle="sm" color="fg.muted">
          {MATCHING}
        </Text>
        <Textarea
          aria-label="Keywords to blacklist"
          placeholder={'kode pos\ncek ongkir, resi'}
          rows={3}
          value={text}
          onChange={(e) => setText(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && ready && !adding) void add();
          }}
        />
        <Checkbox.Root size="sm" checked={children} onCheckedChange={(e) => setChildren(e.checked === true)}>
          <Checkbox.HiddenInput />
          <Checkbox.Control />
          <Checkbox.Label fontWeight="normal">Also remove their children (found only through them)</Checkbox.Label>
        </Checkbox.Root>
        <HStack gap="3" minH="8">
          <Button size="sm" colorPalette="red" loading={adding} disabled={!ready || counting} onClick={() => void add()}>
            <BanIcon />
            Add
          </Button>
          <Text textStyle="sm" color={preview && 'error' in preview ? 'fg.error' : 'fg.muted'} data-testid="blacklist-preview">
            {!entries.length
              ? 'One per line or comma separated.'
              : counting || !preview
                ? 'Counting...'
                : 'error' in preview
                  ? preview.error
                  : preview.added.length === 0
                    ? 'Already in the blacklist.'
                    : `Adds ${plural(preview.added.length, 'entry', 'entries')}, removes ${plural(preview.removed, 'keyword')} from the graph` +
                      (preview.removed > preview.matched
                        ? ` (${preview.matched} matching, ${preview.removed - preview.matched} found only through them)`
                        : !children && preview.removed > 0
                          ? ' (only the matching ones: their children stay)'
                          : '')}
          </Text>
        </HStack>
      </Stack>

      {all.length === 0 ? (
        <Text color="fg.muted" textStyle="sm">
          No entries yet. A right-click on a keyword in the graph blacklists it too.
        </Text>
      ) : (
        <Stack gap="2">
          <HStack gap="3" wrap="wrap">
            <InputGroup maxW="xs" startElement={<SearchIcon size="16" />}>
              <Input
                size="sm"
                aria-label="Search the blacklist"
                placeholder={`Search ${plural(all.length, 'entry', 'entries')}`}
                value={find}
                onChange={(e) => setFind(e.currentTarget.value)}
              />
            </InputGroup>
            <Checkbox.Root
              size="sm"
              disabled={shown.length === 0}
              checked={pagePicked === 0 ? false : pagePicked === pageNames.length ? true : 'indeterminate'}
              onCheckedChange={(e) => pick(pageNames, e.checked === true)}
            >
              <Checkbox.HiddenInput />
              <Checkbox.Control />
              <Checkbox.Label fontWeight="normal">Select all on this page</Checkbox.Label>
            </Checkbox.Root>
            {matching.length > shown.length && !allMatchingPicked && (
              <Link as="button" textStyle="sm" onClick={() => pick(matching.map((e) => e.entry), true)}>
                Select all {matching.length.toLocaleString()}
                {q ? ' matching' : ''}
              </Link>
            )}
            {picked.size > 0 && (
              <Link as="button" textStyle="sm" color="fg.muted" onClick={() => setPicked(new Set())}>
                Clear selection
              </Link>
            )}
            <HStack gap="2" ms="auto">
              <Button size="sm" variant="outline" colorPalette="red" disabled={picked.size === 0} onClick={deleteSelected}>
                <Trash2Icon />
                Delete selected ({picked.size.toLocaleString()})
              </Button>
              <Button size="sm" variant="outline" colorPalette="red" onClick={() => setAsking('all')}>
                <Trash2Icon />
                Delete all
              </Button>
            </HStack>
          </HStack>
          {matching.length === 0 ? (
            <Text color="fg.muted" textStyle="sm">
              No entry has '{find.trim()}'.
            </Text>
          ) : (
            <SimpleGrid columns={{ base: 1, xl: 2 }} columnGap="3" rowGap="1.5" data-testid="blacklist-grid">
              {shown.map((e) => (
                <Entry
                  key={e.entry}
                  entry={e}
                  picked={picked.has(e.entry)}
                  onPick={(on) => pick([e.entry], on)}
                  onDelete={() => void removeOne(e.entry)}
                />
              ))}
            </SimpleGrid>
          )}
          {matching.length > PER_PAGE && (
            <Pagination.Root count={matching.length} pageSize={PER_PAGE} page={at} onPageChange={(e) => setPage(e.page)}>
              <HStack justify="space-between">
                <Pagination.PageText format="long" color="fg.muted" textStyle="sm" />
                <ButtonGroup variant="ghost" size="sm">
                  <Pagination.PrevTrigger asChild>
                    <IconButton aria-label="Previous page">
                      <ChevronLeftIcon />
                    </IconButton>
                  </Pagination.PrevTrigger>
                  <Pagination.Items
                    render={(p) => <IconButton variant={{ base: 'ghost', _selected: 'outline' }}>{p.value}</IconButton>}
                  />
                  <Pagination.NextTrigger asChild>
                    <IconButton aria-label="Next page">
                      <ChevronRightIcon />
                    </IconButton>
                  </Pagination.NextTrigger>
                </ButtonGroup>
              </HStack>
            </Pagination.Root>
          )}
          <Text textStyle="xs" color="fg.muted">
            Deleting entries does not bring back the keywords they removed: a later lookup may find them again.
          </Text>
        </Stack>
      )}
      {asking === 'selected' && (
        <FormDialog
          title={`Delete ${plural(selected.length, 'entry', 'entries')}?`}
          submitText="Delete"
          danger
          onClose={() => setAsking(undefined)}
          onSubmit={() => remove(selected)}
        >
          <Text>{GONE}</Text>
          <Text textStyle="sm" color="fg.muted">
            {selected.slice(0, 12).join(', ')}
            {selected.length > 12 ? ` and ${selected.length - 12} more` : ''}
          </Text>
        </FormDialog>
      )}
      {asking === 'all' && (
        <FormDialog
          title={`Delete all ${plural(all.length, 'entry', 'entries')}?`}
          submitText="Delete all"
          danger
          onClose={() => setAsking(undefined)}
          onSubmit={() => remove([], true)}
        >
          <Text>{GONE}</Text>
        </FormDialog>
      )}
    </Stack>
  );
}

// Entry is a blacklist entry's compact row: its checkbox, the entry, how
// many keywords it removed and when it was added, and its delete button.
function Entry({
  entry: e,
  picked,
  onPick,
  onDelete,
}: {
  entry: KeywordBlacklistEntry;
  picked: boolean;
  onPick: (on: boolean) => void;
  onDelete: () => void;
}) {
  return (
    <HStack
      gap="2"
      px="2"
      py="1"
      borderWidth="1px"
      rounded="md"
      bg={picked ? 'bg.muted' : undefined}
      minW="0"
      data-testid="blacklist-entry"
    >
      <Checkbox.Root size="sm" checked={picked} onCheckedChange={(c) => onPick(c.checked === true)}>
        <Checkbox.HiddenInput aria-label={`Select '${e.entry}'`} />
        <Checkbox.Control />
      </Checkbox.Root>
      <Text textStyle="sm" flex="1" minW="0" wordBreak="break-word">
        {e.entry}
      </Text>
      <Text textStyle="xs" color="fg.muted" whiteSpace="nowrap">
        removed {e.removed.toLocaleString()} · added {e.addedAt ? timestampDate(e.addedAt).toLocaleString() : '-'}
      </Text>
      <IconButton
        size="2xs"
        variant="ghost"
        aria-label={`Remove '${e.entry}' from the blacklist`}
        title="Remove from the blacklist (the keywords it removed do not come back)"
        onClick={onDelete}
      >
        <Trash2Icon />
      </IconButton>
    </HStack>
  );
}

// BlacklistAsk is what blacklisting keywords of the graph would take out,
// both ways (dry runs): only the keywords (only), or with the keywords
// found only through them (children); seeds is how many seeds were left
// out of the entries.
export type BlacklistAsk = { entries: string[]; only: string[]; children: string[]; seeds: number };

// BlacklistChoice asks before keywords of the graph are blacklisted (the
// user, 2026-11-12: "in blacklist, ask blacklist selected keyword or
// selected keyword and child"), always, with two choices and what each
// removes: only the keyword(s), their children staying without the links
// from them, or with the children, as before. onChoose gets keep children.
export function BlacklistChoice({
  ask,
  onChoose,
  onClose,
}: {
  ask: BlacklistAsk;
  onChoose: (keepChildren: boolean) => Promise<void>;
  onClose: () => void;
}) {
  const [error, setError] = useState('');
  const [pending, setPending] = useState<'only' | 'children'>();
  const one = ask.entries.length === 1;
  const choose = async (keep: boolean) => {
    setPending(keep ? 'only' : 'children');
    try {
      await onChoose(keep);
      onClose();
    } catch (err) {
      setError(errorText(err));
      setPending(undefined);
    }
  };
  const list = (names: string[]) =>
    names.slice(0, 12).join(', ') + (names.length > 12 ? ` and ${names.length - 12} more` : '');
  // It opens after the dry runs, a moment after the click: focus going back
  // to the button that asked (seen with the side panel's in the dev UI)
  // must not close it. Escape, Cancel or the backdrop do.
  return (
    <Dialog.Root
      open
      placement="center"
      size="lg"
      onOpenChange={(e) => !e.open && onClose()}
      onFocusOutside={(e) => e.preventDefault()}
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content mx="4" data-testid="blacklist-choice">
            <Dialog.Header>
              <Dialog.Title>{one ? `Blacklist '${ask.entries[0]}'?` : `Blacklist ${ask.entries.length} keywords?`}</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body display="flex" flexDirection="column" gap="3">
              {error && <Text color="fg.error">{error}</Text>}
              <Text textStyle="sm">
                {one ? 'It becomes a blacklist entry' : 'Each becomes a blacklist entry'}: a crawl never adds a keyword
                with its words again. Taking an entry out later does not bring back what it removed.
              </Text>
              {ask.seeds > 0 && (
                <Text textStyle="sm" color="fg.warning">
                  {ask.seeds === 1 ? 'The seed stays' : `The ${ask.seeds} seeds stay`}: seed keywords cannot be
                  blacklisted.
                </Text>
              )}
              <Choice
                title={`${one ? 'Only the keyword' : 'Only the keywords'}: removes ${ask.only.length}`}
                testid="blacklist-only"
              >
                The keywords it matches leave the graph; the keywords found through them stay, without the links from
                them, with their own values. Removes: {list(ask.only)}
              </Choice>
              <Choice title={`With children: removes ${ask.children.length}`} testid="blacklist-children">
                Also the keywords found only through them: those no seed reaches any more. Removes:{' '}
                {list(ask.children)}
              </Choice>
            </Dialog.Body>
            <Dialog.Footer flexWrap="wrap">
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button
                variant="outline"
                colorPalette="red"
                loading={pending === 'only'}
                disabled={!!pending}
                onClick={() => void choose(true)}
              >
                {one ? 'Blacklist only the keyword' : 'Blacklist only the keywords'}
              </Button>
              <Button colorPalette="red" loading={pending === 'children'} disabled={!!pending} onClick={() => void choose(false)}>
                Blacklist with children
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function Choice({ title, testid, children }: { title: string; testid: string; children: ReactNode }) {
  return (
    <Box borderWidth="1px" rounded="md" px="3" py="2" data-testid={testid}>
      <Text fontWeight="semibold" textStyle="sm">
        {title}
      </Text>
      <Text textStyle="sm" color="fg.muted">
        {children}
      </Text>
    </Box>
  );
}
