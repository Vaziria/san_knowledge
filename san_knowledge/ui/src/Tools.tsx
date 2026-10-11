import { Button, Flex, Grid, HStack, NativeSelect, Stack, Text } from '@chakra-ui/react';
import type { ReactNode } from 'react';
import { communityColor, communityNames, communityOf, toggled, TYPE_ORDER, typeInfo, typeSwatchColor, type ColorBy, type Filters } from './model';
import type { ViewMode } from './TopBar';
import { Segmented, SectionTitle, Swatch, useApp } from './ui';

// The panel's Tools tab: the actions that change the knowledge, then the
// filters and view controls for the graph and table.

function Chip({ pressed, title, onClick, children }: { pressed: boolean; title: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button size="xs" variant="outline" rounded="full" ps="6px" pe="10px" gap="6px" fontWeight="400" maxW="100%"
      aria-pressed={pressed} title={title} onClick={onClick} color={pressed ? 'fg' : 'fg.subtle'}
      css={{ '& svg': { opacity: pressed ? 1 : 0.35 }, '@media (forced-colors: active)': { textDecoration: pressed ? 'none' : 'line-through' } }}>
      {children}
    </Button>
  );
}

const Count = ({ n }: { n: number }) => <Text as="span" color="fg.subtle" fontVariantNumeric="tabular-nums">{n}</Text>;

function LegendNote({ icon, text }: { icon: ReactNode; text: string }) {
  return <HStack gap="5px" color="fg.subtle" fontSize="12px">{icon}{text}</HStack>;
}

function Select({ label, value, onChange, children }: { label: string; value: string; onChange: (v: string) => void; children: ReactNode }) {
  return (
    <Grid as="label" templateColumns="64px 1fr" alignItems="center" gap="8px" color="fg.muted" fontSize="12px">
      <span>{label}</span>
      <NativeSelect.Root size="xs">
        <NativeSelect.Field value={value} onChange={(e) => onChange(e.target.value)} bg="bg.panel">{children}</NativeSelect.Field>
        <NativeSelect.Indicator />
      </NativeSelect.Root>
    </Grid>
  );
}

function Action({ label, help, disabled, onClick }: { label: string; help: string; disabled?: boolean; onClick: () => void }) {
  return (
    <>
      <Button size="xs" variant="outline" justifyContent="flex-start" disabled={disabled} onClick={onClick}>{label}</Button>
      <Text fontSize="12px" color="fg.subtle" alignSelf="center">{help}</Text>
    </>
  );
}

export function Tools({ busy, onSync, onAI, onFetch, onDomain, filters, setFilters, setColorBy, mode, hasExplain, onClearExplain, onRelayout, onFit, onCollapseAll, onExpandAll }: {
  busy: boolean;
  onSync: () => void;
  onAI: () => void;
  onFetch: () => void;
  onDomain: () => void;
  filters: Filters;
  setFilters: (update: (f: Filters) => Filters) => void;
  setColorBy: (c: ColorBy) => void;
  mode: ViewMode;
  hasExplain: boolean;
  onClearExplain: () => void;
  onRelayout: () => void;
  onFit: () => void;
  onCollapseAll: () => void;
  onExpandAll?: () => void; // absent when nothing is collapsed
}) {
  const { graph, pal, colorBy, ai } = useApp();
  const typeCounts = new Map<string, number>();
  const communityCounts = new Map<number, number>();
  for (const n of graph.nodes) {
    typeCounts.set(n.node_type, (typeCounts.get(n.node_type) ?? 0) + 1);
    communityCounts.set(communityOf(graph, n.key), (communityCounts.get(communityOf(graph, n.key)) ?? 0) + 1);
  }
  const names = communityNames(graph);
  const hasData = graph.nodes.length > 0;

  return (
    <>
      <SectionTitle>Actions</SectionTitle>
      <Grid templateColumns="auto 1fr" columnGap="10px" rowGap="8px">
        <Action label="Sync docs" help="Update docs and sections from ./docs and the tracked folders." disabled={busy} onClick={onSync} />
        {ai && <Action label="Summarize with AI" help="Write missing summaries, keywords and domains with Claude." disabled={busy} onClick={onAI} />}
        <Action label="Fetch URL" help="Save a web page as a doc in docs/external_sources/web." disabled={busy} onClick={onFetch} />
        <Action label="+ Domain" help="Create a domain to group docs and sections." onClick={onDomain} />
      </Grid>

      {hasData && (
        <>
          <SectionTitle>Show</SectionTitle>
          <Flex wrap="wrap" gap="6px" aria-label="Node types (click to show or hide)">
            {TYPE_ORDER.map((t) => {
              const shown = !filters.hiddenTypes.has(t);
              return (
                <Chip key={t} pressed={shown} title={`${shown ? 'Hide' : 'Show'} ${typeInfo(t).label}s`}
                  onClick={() => setFilters((f) => ({ ...f, hiddenTypes: toggled(f.hiddenTypes, t) }))}>
                  <Swatch type={t} color={typeSwatchColor(pal, colorBy, t)} />{typeInfo(t).label}<Count n={typeCounts.get(t) ?? 0} />
                </Chip>
              );
            })}
          </Flex>
          <Flex wrap="wrap" columnGap="14px" rowGap="4px" mt="8px">
            <LegendNote text="needs summary" icon={<svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><circle cx="7" cy="7" r="5.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="2.5 2" /></svg>} />
            <LegendNote text="reference" icon={<svg width="22" height="8" viewBox="0 0 22 8" aria-hidden="true"><line x1="1" y1="4" x2="21" y2="4" stroke="currentColor" strokeWidth="2" strokeDasharray="1 3" strokeLinecap="round" /></svg>} />
            <LegendNote text="domain" icon={<svg width="22" height="8" viewBox="0 0 22 8" aria-hidden="true"><line x1="1" y1="4" x2="21" y2="4" stroke="currentColor" strokeWidth="1.5" strokeDasharray="5 3" /></svg>} />
          </Flex>
          <Stack gap="8px" mt="12px">
            <Select label="Domain" value={filters.domain} onChange={(domain) => setFilters((f) => ({ ...f, domain }))}>
              <option value="">All domains</option>
              {graph.domains.map((d) => <option key={d.key} value={d.key}>{d.title}</option>)}
            </Select>
            <Select label="Keyword" value={filters.keyword} onChange={(keyword) => setFilters((f) => ({ ...f, keyword }))}>
              <option value="">All keywords</option>
              {graph.keywords.map((k) => <option key={k} value={k}>{k}</option>)}
            </Select>
          </Stack>

          <SectionTitle>Colour by</SectionTitle>
          <Segmented label="Colour nodes by" size="xs" value={colorBy} onChange={setColorBy}
            options={[{ value: 'community', text: 'Community', title: 'Colour by Leiden community' }, { value: 'type', text: 'Type', title: 'Colour by node type' }]} />
          {colorBy === 'community' && graph.communities.size > 0 && (
            <Flex wrap="wrap" gap="6px" mt="10px" aria-label="Leiden communities (click to show or hide)">
              {[...communityCounts.keys()].sort((a, b) => a - b).map((c) => {
                const shown = !filters.hiddenCommunities.has(c);
                return (
                  <Chip key={c} pressed={shown} title={`${shown ? 'Hide' : 'Show'} community ${c + 1}: ${names.get(c)}`}
                    onClick={() => setFilters((f) => ({ ...f, hiddenCommunities: toggled(f.hiddenCommunities, c) }))}>
                    <Swatch type="doc_section" color={communityColor(pal, c)} />
                    <Text as="span" truncate>{names.get(c)}</Text>
                    <Count n={communityCounts.get(c) ?? 0} />
                  </Chip>
                );
              })}
            </Flex>
          )}

          <SectionTitle>View</SectionTitle>
          <HStack gap="6px" wrap="wrap">
            <Button size="xs" variant="outline" disabled={mode !== 'graph'} title="Arrange the graph again, grouped by community" onClick={onRelayout}>Re-layout</Button>
            <Button size="xs" variant="outline" disabled={mode !== 'graph'} title="Fit the visible nodes in view" onClick={onFit}>Fit</Button>
            {hasExplain && <Button size="xs" variant="ghost" onClick={onClearExplain}>Clear highlight</Button>}
          </HStack>
          <HStack gap="6px" wrap="wrap" mt="8px">
            <Button size="xs" variant="outline" title="Hide every doc's sections, leaving domains and docs" onClick={onCollapseAll}>Collapse all docs</Button>
            <Button size="xs" variant="outline" disabled={!onExpandAll} onClick={onExpandAll}>Expand all</Button>
          </HStack>
          <Text fontSize="12px" color="fg.subtle" mt="8px">
            Double-click a domain, doc or section to collapse or expand what is under it: a domain hides its docs, a doc or section its sections.
          </Text>
        </>
      )}
    </>
  );
}
