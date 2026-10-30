import { Code, ConnectError } from '@connectrpc/connect';
import { connections, workflows } from '../api';
import type { Workflow } from '../gen/san/tower/v1/workflow_pb';

// createSyncWorkflow makes the sample sync workflow for a knowledge (the
// user, 2026-10-29: "Add button create sync workflow in knowledge detail for
// sample"), as the "sync knowledge" workflow: run by hand, Knowledge Sync
// reads the files again, then Knowledge Summarize writes the summaries of up
// to 3 docs a run with Gemini Ask, through the first San Remote connection,
// or with Claude Code itself when there is none. Its name is "sync <name>",
// with a number when that is taken.
export async function createSyncWorkflow(knowledgeId: number, knowledgeName: string): Promise<Workflow> {
  const remote = (await connections.listConnections({ type: 'san_remote' })).connections[0];
  const description =
    `Run it to bring ${knowledgeName} up to date: Knowledge Sync reads its files again, then Knowledge Summarize has ` +
    (remote ? 'Gemini' : 'Claude Code') +
    ' write the summaries, keywords and domains of up to 3 docs that need them (the rest wait for the next run). ' +
    'Swap Manual for a Schedule, and switch it on, to run it by itself.';
  let created: Workflow | undefined;
  for (let n = 1; !created; n++) {
    const name = n === 1 ? `sync ${knowledgeName}` : `sync ${knowledgeName} ${n}`;
    try {
      created = (await workflows.createWorkflow({ name, description })).workflow;
    } catch (err) {
      if (!(err instanceof ConnectError && err.code === Code.AlreadyExists) || n >= 20) throw err;
    }
  }
  const step = (id: string, kind: string, x: number, y: number, values: Record<string, string> = {}) => ({
    id,
    kind,
    x,
    y,
    values,
  });
  const link = (source: string, sourceSocket: string, target: string, targetSocket: string) => ({
    source,
    sourceSocket,
    target,
    targetSocket,
  });
  const steps = [
    step('run', 'manual', 0, 200),
    step('kb', 'knowledge-base', 0, 440, { Knowledge: String(knowledgeId), Sync: 'No' }),
    step('sync', 'knowledge-sync', 280, 220),
    step('synced', 'log', 580, 40, { Type: 'info' }),
    step('syncFailed', 'log', 580, 520, { Type: 'failed' }),
    step('sum', 'knowledge-summarize', 580, 240, { Limit: '3' }),
    step('done', 'log', 900, 100, { Type: 'success' }),
    step('sumFailed', 'log', 900, 240, { Type: 'failed' }),
  ];
  const links = [
    link('run', 'Event', 'sync', 'Event'),
    link('kb', 'Knowledge', 'sync', 'Knowledge'),
    link('kb', 'Knowledge', 'sum', 'Knowledge'),
    link('sync', 'Done', 'synced', 'Event'),
    link('sync', 'Text', 'synced', 'Message'),
    link('sync', 'Failed', 'syncFailed', 'Event'),
    link('sync', 'Error', 'syncFailed', 'Message'),
    link('sync', 'Done', 'sum', 'Event'),
    link('sum', 'Done', 'done', 'Event'),
    link('sum', 'Text', 'done', 'Message'),
    link('sum', 'Failed', 'sumFailed', 'Event'),
    link('sum', 'Error', 'sumFailed', 'Message'),
  ];
  if (remote) {
    // Each doc's Prompt goes to Gemini, and its answer back into Answer; a
    // failed ask is a warning, the doc staying for the next run.
    steps.push(
      step('gemini', 'gemini-ask', 900, 420, { Connection: String(remote.id), Wait: '10m', Temporary: 'Yes', Chat: 'Leave' }),
      step('geminiFailed', 'log', 1200, 560, { Type: 'warning' }),
    );
    links.push(
      link('sum', 'Each', 'gemini', 'Event'),
      link('sum', 'Prompt', 'gemini', 'Prompt'),
      link('gemini', 'Answer', 'sum', 'Answer'),
      link('gemini', 'Failed', 'geminiFailed', 'Event'),
      link('gemini', 'Error', 'geminiFailed', 'Message'),
    );
  }
  const { workflow } = await workflows.updateWorkflow({
    workflow: { id: created.id, name: created.name, description: created.description, folderId: created.folderId, steps, links },
  });
  return workflow ?? created;
}
