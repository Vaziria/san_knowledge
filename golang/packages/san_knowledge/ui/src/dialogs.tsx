import { Box, Button, Dialog, Field as ChakraField, Input, Portal, Text, Textarea } from '@chakra-ui/react';
import { useRef, useState, type FormEvent, type ReactNode, type RefObject } from 'react';
import { api, errorText, q, type KNode } from './api';
import { locOf, splitKeywords, typeInfo } from './model';
import { Mono, toast, useApp, type DialogRequest } from './ui';

function Field({ label, help, children }: { label: string; help?: string; children: ReactNode }) {
  return (
    <ChakraField.Root mb="12px">
      <ChakraField.Label fontSize="12px" color="fg.muted">{label}</ChakraField.Label>
      {children}
      {help && <ChakraField.HelperText fontSize="11px" color="fg.subtle">{help}</ChakraField.HelperText>}
    </ChakraField.Root>
  );
}

// A form in a modal: shows the error from onSubmit, closes when it succeeds.
function FormDialog({ title, submitText, danger, focus, onSubmit, onClose, children }: {
  title: string;
  submitText: string;
  danger?: boolean;
  focus?: RefObject<HTMLElement | null>;
  onSubmit: () => Promise<void>;
  onClose: () => void;
  children: ReactNode;
}) {
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setPending(true);
    try {
      await onSubmit();
      onClose();
    } catch (err) {
      setError(errorText(err));
      setPending(false);
    }
  };
  return (
    <Dialog.Root open placement="center" onOpenChange={(e) => { if (!e.open) onClose(); }} initialFocusEl={focus ? () => focus.current : undefined}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content maxW="520px" mx="16px">
            <form onSubmit={submit}>
              <Dialog.Header>
                <Dialog.Title fontSize="16px">{title}</Dialog.Title>
              </Dialog.Header>
              <Dialog.Body>
                {error && <Text color="fg.error" fontSize="13px" mb="10px">{error}</Text>}
                {children}
              </Dialog.Body>
              <Dialog.Footer>
                <Button variant="ghost" size="sm" onClick={onClose}>Cancel</Button>
                <Button type="submit" size="sm" colorPalette={danger ? 'red' : 'accent'} loading={pending}>{submitText}</Button>
              </Dialog.Footer>
            </form>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

function EditDialog({ node, onClose }: { node: KNode; onClose: () => void }) {
  const { reload, select } = useApp();
  const isDomain = node.node_type === 'domain';
  const [title, setTitle] = useState(node.title || '');
  const [summary, setSummary] = useState(node.summary || '');
  const [keyword, setKeyword] = useState((node.keyword ?? []).join(', '));
  const summaryRef = useRef<HTMLTextAreaElement>(null);
  const save = async () => {
    const body: { summary: string; keyword: string[]; title?: string } = { summary: summary.trim(), keyword: splitKeywords(keyword) };
    if (isDomain && title.trim() !== node.title) body.title = title.trim();
    await api('PUT', `/api/node?key=${q(node.key)}`, body);
    toast('Saved');
    await reload();
    select(node.key); // a domain keeps its key when retitled
  };
  return (
    <FormDialog title={`Edit ${typeInfo(node.node_type).label}`} submitText="Save" focus={summaryRef} onSubmit={save} onClose={onClose}>
      {isDomain ? (
        <Field label="Title"><Input required autoComplete="off" value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
      ) : (
        <Box mb="12px"><Text fontWeight="600">{node.title}</Text><Mono>{locOf(node)}</Mono></Box>
      )}
      <Field label="Summary" help="1–3 sentences. Saving marks it as written by you, so the AI will not overwrite it.">
        <Textarea ref={summaryRef} rows={4} value={summary} onChange={(e) => setSummary(e.target.value)} />
      </Field>
      <Field label="Keywords">
        <Input autoComplete="off" placeholder="comma separated" value={keyword} onChange={(e) => setKeyword(e.target.value)} />
      </Field>
    </FormDialog>
  );
}

function DomainDialog({ onClose }: { onClose: () => void }) {
  const { reload, select } = useApp();
  const [title, setTitle] = useState('');
  const [summary, setSummary] = useState('');
  const [keyword, setKeyword] = useState('');
  const titleRef = useRef<HTMLInputElement>(null);
  const create = async () => {
    const res = await api<{ created: boolean; node: KNode }>('POST', '/api/domains', { title: title.trim(), summary: summary.trim(), keyword: splitKeywords(keyword) });
    toast(res.created ? 'Domain created' : 'Domain already existed; updated');
    await reload();
    select(res.node.key, true);
  };
  return (
    <FormDialog title="New domain" submitText="Create" focus={titleRef} onSubmit={create} onClose={onClose}>
      <Field label="Title"><Input ref={titleRef} required autoComplete="off" placeholder="e.g. Internet Marketing" value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
      <Field label="Summary"><Textarea rows={3} value={summary} onChange={(e) => setSummary(e.target.value)} /></Field>
      <Field label="Keywords"><Input autoComplete="off" placeholder="comma separated" value={keyword} onChange={(e) => setKeyword(e.target.value)} /></Field>
    </FormDialog>
  );
}

function AssignDialog({ nodeKey, onClose }: { nodeKey: string; onClose: () => void }) {
  const { graph, reload, select } = useApp();
  const [domain, setDomain] = useState('');
  const domainRef = useRef<HTMLInputElement>(null);
  const assign = async () => {
    const res = await api<{ domain: KNode; domain_created: boolean }>('POST', '/api/assign', { key: nodeKey, domain: domain.trim() });
    toast(res.domain_created ? `Created and assigned “${res.domain.title}”` : `Assigned “${res.domain.title}”`);
    await reload();
    select(nodeKey);
  };
  return (
    <FormDialog title={`Assign domain to “${graph.byKey.get(nodeKey)?.title || nodeKey}”`} submitText="Assign" focus={domainRef} onSubmit={assign} onClose={onClose}>
      <Field label="Domain" help="Pick an existing domain or type a new title to create it.">
        <Input ref={domainRef} required list="domain-list" autoComplete="off" placeholder="existing or new domain" value={domain} onChange={(e) => setDomain(e.target.value)} />
      </Field>
      <datalist id="domain-list">{graph.domains.map((d) => <option key={d.key} value={d.title} />)}</datalist>
    </FormDialog>
  );
}

function DeleteDialog({ node, members, onClose }: { node: KNode; members: number; onClose: () => void }) {
  const { reload, select } = useApp();
  const remove = async () => {
    await api('DELETE', `/api/node?key=${q(node.key)}`);
    toast('Domain deleted');
    select(null);
    await reload();
  };
  return (
    <FormDialog title={`Delete domain “${node.title}”?`} submitText="Delete" danger onSubmit={remove} onClose={onClose}>
      <Text color="fg.muted">This removes the domain and its {members} domain_of edge{members === 1 ? '' : 's'}. Docs and sections are kept.</Text>
    </FormDialog>
  );
}

function FetchDialog({ onClose }: { onClose: () => void }) {
  const { runFetch } = useApp();
  const [url, setUrl] = useState('');
  const urlRef = useRef<HTMLInputElement>(null);
  // The fetch runs after the dialog closes, under the busy indicator.
  const start = async () => { runFetch(url.trim()); };
  return (
    <FormDialog title="Fetch web page" submitText="Fetch" focus={urlRef} onSubmit={start} onClose={onClose}>
      <Field label="URL" help="The page’s main content is saved as markdown in docs/external_sources/web and synced. Fetching a saved URL again refreshes it.">
        <Input ref={urlRef} type="url" required autoComplete="off" placeholder="https://…" value={url} onChange={(e) => setUrl(e.target.value)} />
      </Field>
    </FormDialog>
  );
}

export function Dialogs({ request, onClose }: { request: DialogRequest | null; onClose: () => void }) {
  switch (request?.kind) {
    case 'edit': return <EditDialog node={request.node} onClose={onClose} />;
    case 'domain': return <DomainDialog onClose={onClose} />;
    case 'assign': return <AssignDialog nodeKey={request.key} onClose={onClose} />;
    case 'delete': return <DeleteDialog node={request.node} members={request.members} onClose={onClose} />;
    case 'fetch': return <FetchDialog onClose={onClose} />;
    default: return null;
  }
}
