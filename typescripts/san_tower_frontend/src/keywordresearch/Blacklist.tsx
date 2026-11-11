import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Button, HStack, IconButton, Stack, Table, Text, Textarea } from '@chakra-ui/react';
import { BanIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { errorText, keywordResearch } from '../api';
import type { KeywordResearch } from '../gen/san/tower/v1/keyword_research_pb';
import { toast } from '../ui';

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
  'keywords it matches and those found only through them (one also found from a keyword that stays, stays), and ' +
  'a crawl never adds them again. The seed keyword cannot be blacklisted.';

// BlacklistPanel is a research page's Blacklist tab (the user, 2026-11-12:
// "Add tab blacklist keyword"): a box to add entries, one per line or comma
// separated, with how many keywords adding them would remove (a dry run of
// AddToBlacklist, asked again as the typing pauses and as the graph
// changes, keywords), how matching works, and the entries: each with when
// it was added, how many keywords it removed then, and a delete button.
// Deleting one does not bring its keywords back.
export function BlacklistPanel({ research, keywords }: { research: KeywordResearch; keywords: number }) {
  const [text, setText] = useState('');
  const [preview, setPreview] = useState<Preview>();
  const [counting, setCounting] = useState(false);
  const [adding, setAdding] = useState(false);
  const entries = useMemo(() => splitEntries(text), [text]);
  const key = entries.join('\n');

  useEffect(() => {
    setPreview(undefined);
    if (!entries.length) return;
    let stale = false;
    setCounting(true);
    const t = setTimeout(async () => {
      try {
        const r = await keywordResearch.addToBlacklist({ id: research.id, entries, dryRun: true });
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
  }, [key, research.id, keywords, research.blacklist.length]);

  const add = async () => {
    setAdding(true);
    try {
      const r = await keywordResearch.addToBlacklist({ id: research.id, entries });
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
  const remove = async (entry: string) => {
    try {
      await keywordResearch.removeFromBlacklist({ id: research.id, entry });
      toast(`'${entry}' is out of the blacklist: the keywords it removed stay out until a lookup finds them again`);
    } catch (err) {
      toast(`Not removed: ${errorText(err)}`, true);
    }
  };

  const ready = preview && !('error' in preview) && preview.added.length > 0;
  const list = [...research.blacklist].reverse(); // the newest first
  return (
    <Stack gap="4" flex="1" minH="0" overflowY="auto" maxW="4xl" pb="4">
      <Stack gap="2">
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
                        : '')}
          </Text>
        </HStack>
      </Stack>
      <Stack gap="2">
        {list.length === 0 ? (
          <Text color="fg.muted" textStyle="sm">
            No entries yet. A right-click on a keyword in the graph blacklists it too.
          </Text>
        ) : (
          <Table.Root size="sm" tableLayout="fixed" borderWidth="1px" rounded="md">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>Entry</Table.ColumnHeader>
                <Table.ColumnHeader w="48">Added</Table.ColumnHeader>
                <Table.ColumnHeader w="36" textAlign="end">
                  Removed
                </Table.ColumnHeader>
                <Table.ColumnHeader w="12" />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {list.map((e) => (
                <Table.Row key={e.entry}>
                  <Table.Cell wordBreak="break-word">{e.entry}</Table.Cell>
                  <Table.Cell color="fg.muted">{e.addedAt ? timestampDate(e.addedAt).toLocaleString() : ''}</Table.Cell>
                  <Table.Cell textAlign="end" color="fg.muted">
                    {plural(e.removed, 'keyword')}
                  </Table.Cell>
                  <Table.Cell textAlign="end">
                    <IconButton
                      size="2xs"
                      variant="ghost"
                      aria-label={`Remove '${e.entry}' from the blacklist`}
                      title="Remove from the blacklist (the keywords it removed do not come back)"
                      onClick={() => void remove(e.entry)}
                    >
                      <Trash2Icon />
                    </IconButton>
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        )}
        <Text textStyle="xs" color="fg.muted">
          Removing an entry does not bring back the keywords it removed: a later lookup may find them again.
        </Text>
      </Stack>
    </Stack>
  );
}
