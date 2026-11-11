import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Badge, Button, HStack, Heading, Input, InputGroup, Stack, Table, Text, Textarea } from '@chakra-ui/react';
import { FolderPlusIcon, SearchIcon, TrendingUpIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { errorText, keywordResearch } from '../api';
import {
  draggable,
  FolderCrumbs,
  folderCounts,
  FolderDialog,
  folderPath,
  FolderRow,
  MoveDialog,
  pathText,
  RowMenu,
  type FolderApi,
} from '../folders';
import type { KeywordResearch, KeywordResearchFolder } from '../gen/san/tower/v1/keyword_research_pb';
import { active, statusOf } from '../keywordresearch/trends';
import { usePoll } from '../poll';
import { StreamIndicator, useStream } from '../stream';
import { FormDialog, FormField, toast } from '../ui';

// researchFolders are the folders of keyword researches, for folders.tsx.
const researchFolders: FolderApi = {
  top: 'All researches',
  create: async (name, parentId) => (await keywordResearch.createResearchFolder({ name, parentId })).folder!,
  update: async (id, name, parentId) => (await keywordResearch.updateResearchFolder({ id, name, parentId })).folder!,
};

// KeywordResearchList is the Keyword Research page (after Knowledge; the
// user, 2026-11-11): the tower's keyword researches (KeywordResearchService),
// each a seed keyword whose Google Trends related queries are mapped into a
// graph; a click opens it (KeywordResearchPage). Folders work as the
// Workflow and Knowledge pages' (../folders.tsx): one folder at a time
// (?folder=<id>), its folders first, a breadcrumb back up, New folder,
// rename, move, a research dragged onto a folder moves there, and a deleted
// folder's contents go up one level. The search (?q=) looks in every folder.
// The list is live (WatchResearches): each research's status, progress and
// counts as they change, and the whole list again when another page makes,
// moves or deletes researches or folders; while the stream is down it asks
// every 15 s.
export function KeywordResearchList() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const folder = Math.max(0, Number(params.get('folder')) || 0);
  const q = params.get('q') ?? '';
  const [text, setText] = useState(q);
  const [all, setAll] = useState<KeywordResearch[]>();
  const [folders, setFolders] = useState<KeywordResearchFolder[]>();
  const [error, setError] = useState('');
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<KeywordResearch>();
  const [moving, setMoving] = useState<KeywordResearch>();
  const [deleting, setDeleting] = useState<KeywordResearch>();
  const [folderEdit, setFolderEdit] = useState<KeywordResearchFolder | 'new'>();
  const [folderMove, setFolderMove] = useState<KeywordResearchFolder>();
  const [folderDelete, setFolderDelete] = useState<KeywordResearchFolder>();
  const openFolder = (id: number) => {
    setText('');
    setParams(id ? { folder: String(id) } : {});
  };

  // The search box follows the URL, and sets it once the typing pauses.
  useEffect(() => setText(q), [q]);
  useEffect(() => {
    const query = text.trim();
    if (query === q) return;
    const t = setTimeout(
      () =>
        setParams(
          (p) => {
            const next = new URLSearchParams(p);
            if (query) next.set('q', query);
            else next.delete('q');
            return next;
          },
          { replace: true },
        ),
      250,
    );
    return () => clearTimeout(t);
  }, [text, q]);

  // Every folder's researches and every folder, pushed by the tower as they
  // change.
  const live = useStream(
    (signal) => keywordResearch.watchResearches({}, { signal }),
    (m) => {
      const e = m.event;
      if (e.case === 'list') {
        setAll(e.value.researches);
        setFolders(e.value.folders);
        setError('');
      } else if (e.case === 'research') {
        const r = e.value;
        setAll((list) =>
          list?.some((k) => k.id === r.id) ? list.map((k) => (k.id === r.id ? r : k)) : [...(list ?? []), r],
        );
      }
    },
    [],
  );
  // load reads them all every 15 s while the stream is down, and after this
  // page's own changes then (live, the tower sends the list again itself,
  // and a read that crossed a newer change would show an old status).
  const load = async (stale = () => false) => {
    try {
      const [r, f] = await Promise.all([
        keywordResearch.listResearches({ allFolders: true }),
        keywordResearch.listResearchFolders({}),
      ]);
      if (stale()) return;
      setAll(r.researches);
      setFolders(f.folders);
      setError('');
    } catch (err) {
      if (!stale()) setError(errorText(err));
    }
  };
  const changed = () => live !== 'live' && void load();
  usePoll(load, 15000, [], live === 'reconnecting');

  // A folder that is gone shows the top.
  useEffect(() => {
    if (folders && folder && !folders.some((f) => f.id === folder)) openFolder(0);
  });

  const query = q.toLowerCase();
  const list = (all ?? []).filter((k) =>
    query
      ? [k.name, k.description, k.seed].some((s) => s.toLowerCase().includes(query))
      : k.folderId === folder,
  );
  const inside = query ? [] : (folders ?? []).filter((f) => f.parentId === folder);
  const folderName = (id: number) =>
    id ? (folderPath(folders ?? [], id).at(-1)?.name ?? 'a folder') : researchFolders.top;
  const nameOf = (id: number) => all?.find((k) => k.id === id)?.name;

  const move = async (id: number, to: number, name?: string) => {
    try {
      await keywordResearch.moveResearches({ ids: [id], folderId: to });
      toast(`Moved ${name ?? 'the research'} to ${folderName(to)}`);
      changed();
    } catch (err) {
      toast(`Not moved: ${errorText(err)}`, true);
    }
  };
  const duplicate = async (k: KeywordResearch) => {
    try {
      const { research } = await keywordResearch.duplicateResearch({ id: k.id });
      toast(`Duplicated ${k.name} as ${research!.name}, with its graph`);
      changed();
    } catch (err) {
      toast(`Not duplicated: ${errorText(err)}`, true);
    }
  };

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Keyword Research</Heading>
        <HStack gap="3">
          <StreamIndicator state={live} />
          <Button size="sm" variant="outline" onClick={() => setFolderEdit('new')}>
            <FolderPlusIcon />
            New folder
          </Button>
          <Button size="sm" colorPalette="blue" onClick={() => setCreating(true)}>
            New research
          </Button>
        </HStack>
      </HStack>
      <Text color="fg.muted" textStyle="sm">
        A research maps a seed keyword's related queries on Google Trends: its Top and Rising queries, then the Top
        ones' in turn, to its deep limit, through your Chrome (a San Remote connection). One crawl runs at a time.
      </Text>
      <InputGroup maxW="sm" startElement={<SearchIcon size="16" />}>
        <Input
          size="sm"
          value={text}
          onChange={(e) => setText(e.currentTarget.value)}
          placeholder="Search by name, description or seed, in every folder"
        />
      </InputGroup>
      {!q && folders && (
        <FolderCrumbs
          api={researchFolders}
          folders={folders}
          folder={folder}
          onOpen={openFolder}
          onDrop={(id, to) => void move(id, to, nameOf(id))}
        />
      )}
      {error ? (
        <Text color="fg.error">The tower does not answer: {error}</Text>
      ) : !all || !folders ? (
        <Text color="fg.muted">Loading...</Text>
      ) : list.length === 0 && inside.length === 0 ? (
        <Text color="fg.muted">
          {q
            ? `No research matches "${q}".`
            : folder
              ? 'This folder is empty: New research makes one here, and a research dragged onto a folder moves there.'
              : 'No research yet: New research starts one from a seed keyword.'}
        </Text>
      ) : (
        <Table.ScrollArea borderWidth="1px" rounded="md">
          <Table.Root size="sm" interactive tableLayout="fixed">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader w="12" />
                <Table.ColumnHeader>Name</Table.ColumnHeader>
                <Table.ColumnHeader w="48">Seed keyword</Table.ColumnHeader>
                <Table.ColumnHeader w="28">Status</Table.ColumnHeader>
                <Table.ColumnHeader w="24" textAlign="end">
                  Keywords
                </Table.ColumnHeader>
                <Table.ColumnHeader w="44">Last run</Table.ColumnHeader>
                <Table.ColumnHeader w="12" />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {inside.map((f) => (
                <FolderRow
                  key={`f${f.id}`}
                  folder={f}
                  counts={folderCounts(f.researchCount, 'research', 'researches', f.folderCount)}
                  align="start"
                  pad={2}
                  onOpen={() => openFolder(f.id)}
                  onDrop={(id) => void move(id, f.id, nameOf(id))}
                  onRename={() => setFolderEdit(f)}
                  onMove={() => setFolderMove(f)}
                  onDelete={() => setFolderDelete(f)}
                />
              ))}
              {list.map((k) => {
                const st = statusOf(k);
                return (
                  <Table.Row
                    key={k.id}
                    cursor="pointer"
                    onClick={() => navigate(`/keyword-research/${k.id}`)}
                    {...draggable(k.id)}
                  >
                    <Table.Cell color="fg.muted">
                      <TrendingUpIcon size="18" />
                    </Table.Cell>
                    <Table.Cell>
                      <Text fontWeight="semibold" truncate>
                        {k.name}
                      </Text>
                      {(k.description || (q && k.folderId > 0)) && (
                        <Text textStyle="xs" color="fg.muted" truncate>
                          {[q && k.folderId > 0 && `in ${pathText(folders, k.folderId)}`, k.description]
                            .filter(Boolean)
                            .join(' · ')}
                        </Text>
                      )}
                    </Table.Cell>
                    <Table.Cell>
                      <Text truncate>{k.seed || '-'}</Text>
                    </Table.Cell>
                    <Table.Cell>
                      <Badge colorPalette={st.color} title={k.message || undefined}>
                        {st.label}
                      </Badge>
                    </Table.Cell>
                    <Table.Cell textAlign="end">{k.nodeCount}</Table.Cell>
                    <Table.Cell color="fg.muted" whiteSpace="nowrap">
                      {k.lastRunAt ? timestampDate(k.lastRunAt).toLocaleString() : 'Never'}
                    </Table.Cell>
                    <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
                      <RowMenu
                        name={k.name}
                        items={{
                          rename: () => setEditing(k),
                          move: () => setMoving(k),
                          duplicate: () => void duplicate(k),
                          delete: () => setDeleting(k),
                        }}
                      />
                    </Table.Cell>
                  </Table.Row>
                );
              })}
            </Table.Body>
          </Table.Root>
        </Table.ScrollArea>
      )}
      {creating && (
        <ResearchDialog
          where={folder ? folderName(folder) : ''}
          onClose={() => setCreating(false)}
          onSaved={async (name, description, seed) => {
            const { research } = await keywordResearch.createResearch({ name, description, seed, folderId: folder });
            toast(`Created research ${research!.name}`);
            navigate(`/keyword-research/${research!.id}`);
          }}
        />
      )}
      {editing && (
        <ResearchDialog
          research={editing}
          onClose={() => setEditing(undefined)}
          onSaved={async (name, description) => {
            const k = editing;
            await keywordResearch.updateResearch({
              id: k.id,
              name,
              description,
              seed: k.seed,
              deepLimit: k.deepLimit,
              filters: k.filters,
              connectionId: k.connectionId,
            });
            toast(`Saved ${name}`);
            changed();
          }}
        />
      )}
      {moving && folders && (
        <MoveDialog
          api={researchFolders}
          name={moving.name}
          folders={folders}
          from={moving.folderId}
          onClose={() => setMoving(undefined)}
          onMove={(to) => move(moving.id, to, moving.name)}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete research ${deleting.name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await keywordResearch.deleteResearch({ id: deleting.id });
            toast(`Deleted research ${deleting.name}`);
            changed();
          }}
        >
          <Text>
            Its graph of {deleting.nodeCount} keywords is deleted with it.
            {active(deleting) && ' Its crawl stops first.'}
          </Text>
        </FormDialog>
      )}
      {folderEdit && (
        <FolderDialog
          api={researchFolders}
          folder={folderEdit === 'new' ? undefined : folderEdit}
          parent={folder}
          onClose={() => setFolderEdit(undefined)}
          onSaved={(f) => {
            toast(folderEdit === 'new' ? `Created folder ${f.name}` : `Renamed the folder to ${f.name}`);
            changed();
          }}
        />
      )}
      {folderMove && folders && (
        <MoveDialog
          api={researchFolders}
          name={`folder ${folderMove.name}`}
          folders={folders}
          from={folderMove.parentId}
          folder={folderMove}
          onClose={() => setFolderMove(undefined)}
          onMove={async (to) => {
            await researchFolders.update(folderMove.id, folderMove.name, to);
            toast(`Moved folder ${folderMove.name} to ${folderName(to)}`);
            changed();
          }}
        />
      )}
      {folderDelete && (
        <FormDialog
          title={`Delete folder ${folderDelete.name}?`}
          submitText="Delete folder"
          danger
          onClose={() => setFolderDelete(undefined)}
          onSubmit={async () => {
            await keywordResearch.deleteResearchFolder({ id: folderDelete.id });
            toast(`Deleted folder ${folderDelete.name}`);
            changed();
          }}
        >
          <Text>
            Its {folderDelete.researchCount} researches and {folderDelete.folderCount} folders move to{' '}
            {folderName(folderDelete.parentId)}. No research is deleted.
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}

// ResearchDialog makes a research (name, description, seed keyword; its
// filters start at their defaults and change on its page), or renames one.
function ResearchDialog({
  research,
  where,
  onClose,
  onSaved,
}: {
  research?: KeywordResearch;
  where?: string;
  onClose: () => void;
  onSaved: (name: string, description: string, seed: string) => Promise<void>;
}) {
  const [name, setName] = useState(research?.name ?? '');
  const [description, setDescription] = useState(research?.description ?? '');
  const [seed, setSeed] = useState(research?.seed ?? '');
  return (
    <FormDialog
      title={research ? `Rename ${research.name}` : where ? `New research in ${where}` : 'New research'}
      submitText={research ? 'Save' : 'Create'}
      onClose={onClose}
      onSubmit={() => onSaved(name.trim(), description.trim(), seed.trim())}
    >
      <FormField label="Name" help="Unique, at most 100 characters.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Aplikasi kasir" />
      </FormField>
      <FormField label="Description">
        <Textarea
          rows={2}
          value={description}
          onChange={(e) => setDescription(e.currentTarget.value)}
          placeholder="What it is for"
        />
      </FormField>
      {!research && (
        <FormField
          label="Seed keyword"
          help="Where the crawl starts. Its filters start at Indonesia, Past 12 months, All categories, Web Search and deep limit 2; change them on its page."
        >
          <Input value={seed} onChange={(e) => setSeed(e.currentTarget.value)} placeholder="aplikasi kasir" />
        </FormField>
      )}
    </FormDialog>
  );
}
