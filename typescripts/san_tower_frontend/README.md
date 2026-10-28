# San Tower frontend

The UI of the tower,
[golang/apps/san_tower](../../golang/apps/san_tower/README.md). React +
TypeScript + Chakra UI v3, with react-router for the pages. Vite builds it
into `golang/apps/san_tower/internal/web/dist`, which the binary embeds, so
rebuild the binary (`build.ps1` there) to see a change in `san_tower serve`.

The easiest way to work on it is `go run .\cmd\san_tower dev` in
`golang/apps/san_tower`, which runs the tower and this dev server together.

```powershell
npm install
npm run dev        # hot reload; TowerService is proxied to $env:SAN_TOWER_SERVER, default a `san_tower serve` on :7477
npm run build      # typecheck + build into the Go app
npm run gen        # regenerate src/gen from the protos (runs buf in golang/apps/san_tower)
```

`logo-candidates/` holds five logo ideas for the tower (open its `index.html`
to compare them at 16 to 128 px). The user chose 1, the watchtower
(2026-10-24): it is `public/logo.svg`, the sidebar's brand and the favicon.

## Files

- `src/routes.tsx`: the pages and the router. A new page is one entry in
  `pages`, which also puts it in the menu, with its icon (from
  `lucide-react`, the icon set for the tower).
- `src/Layout.tsx`: the menu in a sidebar on the left, and the current page
  beside it. The button by the title collapses the sidebar to its icons (the
  label shows on hover); the browser remembers it (localStorage
  `san-tower-sidebar-collapsed`).
- `src/pages/`:
  - `Home.tsx`: the tower's status.
  - `Chat.tsx` (`/chat`, after Home; the user, 2026-10-28): the tower's
    own chat (ChatService). The chats on the left, the newest message
    first, each with its workflow (a green dot when it is on) and its last
    message, asked for every 5 s; New chat picks a workflow with a Tower
    Chat step and a name (empty: the workflow's). The chat shown is in the
    URL (`?c=3`; none: the newest). On the right the conversation: the
    user's messages on the right, the workflow's on the left, as Markdown
    when sent so (`react-markdown` with `remark-gfm`, never raw HTML; links
    open in a new tab), each with its time. Under the user's, how the runs
    it started are doing: Running with the last line logged, or Failed or
    Interrupted with the error and a link to the workflow's Runs. It asks
    every second for what came after the newest message and for the runs
    still going (`usePoll`), keeps the newest in view while the reader is
    at the bottom, and Older messages loads the ones before. The header has
    the workflow (a link to its editor), its On/Off switch, and Rename and
    Delete. Enter sends, Shift+Enter is a new line; a workflow switched off
    or gone shows why instead of the box.
  - `MessageBroker.tsx`: the San Message broker the tower uses. A status
    bar on top (BrokerService.GetBrokerStatus): Running or Not answering,
    inside the tower or a separate server and since when, topics and events
    published, subscribers and pull ones connected, how many are behind
    (orange) and retrying (red), and the size on disk and data folder, or
    the separate server's address. The Topics tab lists topics (last event, where
    the log ends, subscriber count), creates them, and deletes those without
    subscribers. The Subscribers tab lists subscribers (type, webhook URI or
    whether a pull subscriber is connected, where it is in the log, failed
    attempts and the next retry), filtered by topic, and creates and deletes
    them; a new one starts at the last offset (only new events) or from the
    start of the topic's log. The tab and the topic are in the URL
    (`?tab=subscribers&topic=orders`). It reloads after each change and on
    Refresh.
  - `Connections.tsx` (`/connections`): the connections workflow steps use
    (ConnectionService), in a table (name, type, address, last change).
    Tabs (the user, 2026-10-28): All, then one per type with its count,
    which lists that type's connections (without the Type column) and says
    what it is; the tab is in the URL (`?type=telegramBot`), and its New
    connection starts the dialog on that type. New
    connection, or a row, opens a dialog: the type (San Remote,
    Telegram Bot, WhatsApp or Binance; a saved connection keeps its type), a name, and the URL
    of `san_remote api` (default http://127.0.0.1:7478) or the bot's token
    (a password field), with Test, which shows what the other side answers
    before anything is saved. A bot's address in the table is its id, the
    token's part before the colon. A row's menu has Edit, Test (as a
    toast) and Delete (asks first). A change makes the editor's Connection
    selects ask again.
  - `Storage.tsx` (`/storage`, after Connections; the user's ask,
    2026-10-25): the storages (StorageService), named base folders that
    workflow steps use (the Storage step). A table (name, base path, last
    change; a relative path has its absolute path under it, and long paths
    wrap). The line on top says where a relative path goes, the tower's
    data folder (ListStorages' `data_dir`). New storage, or a row, opens a
    dialog: a name and a base path (saved with /; a \ typed becomes /);
    saving toasts the absolute path. A
    row's menu has Edit and Delete (asks first, saying the folder stays). A
    change makes the editor's Storage selects ask again.
  - `BrowserRemoteServer.tsx` (`/browser-remote`): the San Remote server
    the tower can run (BrowserRemoteService). An Active switch starts or
    stops it (a failure, such as san_remote already running, is a toast); its
    status (Running in the tower since, Running outside the tower with which
    of serve and api answer, Off, or Not running with why); the server URI for
    the extension's Server field and the API URI for a San Remote
    connection, each with a Copy button; and the attached browsers, the
    tower's or those of the san_remote serve outside it, asked for every 3 s.
    Listen on (the user, 2026-10-28) is This computer only or All networks
    (SetBrowserRemoteNetwork); All networks asks first, then shows a warning
    and the URIs at this computer's local network addresses.
  - `Settings.tsx` (`/settings`, last in the menu): the tower's settings, a
    section per part. Message Broker (BrokerService; the user moved it here
    from the Message Broker page, 2026-10-24): which broker the tower uses,
    Inside the tower (its data directory) or Separate server (a
    san_message server's URI, on this machine or another; the user's
    choice, 2026-10-25). Both values are kept. Saving asks first, saying
    what the switch does, then switches at once; a server that does not
    answer, or a folder that cannot open, is refused in the dialog and
    nothing changes.
  - `WorkflowList.tsx` (`/workflow`): two tabs. Workflows: the workflows
    (WorkflowService, in the tower's database) in a table (an Active
    switch, green when on, name, step count, last change) with New workflow,
    20 a page. The search box (by name, ignoring case) and the page are in
    the URL (`?q=order&page=2`); both run on the server (ListWorkflows'
    query, offset and limit). A row, or a new workflow, opens its editor;
    the three dots at a row's end open its menu, with Rename, Export and
    Delete (asks first). Rename waits for an editor's last save, then sends the
    stored graph with the new name (UpdateWorkflow replaces both). Import
    and Export all sit by New workflow (the user's ask, 2026-10-26;
    `src/workflow/transfer.tsx`): Export, here, in a row's menu or in the
    editor's header (which saves first), has the tower write the file
    (ExportWorkflows) and saves it as a download; Import reads the .json
    files picked, one ImportWorkflows each (one refused does not stop the
    others), and a dialog lists the workflows made, as links to their
    editors, the notes to check, and the files refused. Logs
    (`?tab=logs`, the user's ask, 2026-10-25): `src/workflow/RunLog.tsx`.
  - `WorkflowEditor.tsx` (`/workflow/<id>`, a page of its own; the nav keeps
    Workflow marked): one workflow, with a link back to the list, the pencil
    by its name to rename it (saved in line with the edits, which then go
    with the new name), and Delete (asks first, then goes back to the list).
    The editor is React Flow
    (`@xyflow/react`) made to look and handle like Blender's node editors
    (the user's choice, 2026-10-24):
    Shift+A opens the Add menu at the pointer (the Add button opens it under
    itself, and adds in the middle of the canvas), a link goes from an output
    socket to an input of the same type and an input takes one link,
    right-clicking a step (or the selection) offers Duplicate, Disconnect
    links and Delete, and right-clicking a link Delete link. Dragging on empty
    space pans, with any button (the user's ask, 2026-10-25); Shift+drag
    box-selects. While steps are selected, their links are bolder and
    glow a little, and the other links fade (the user's ask, 2026-10-25;
    a class on the edges drawn, not saved). Undo and Redo (the arrows in the header, Ctrl+Z, and
    Ctrl+Shift+Z or Ctrl+Y; the user's ask, 2026-10-25) step through the
    changes to steps and links: adding, deleting, moving (a whole drag is
    one step), linking and the values typed in a step (a burst of typing is
    one step). A field being typed in keeps Ctrl+Z for its own text. While
    a step's text box has the focus it shows what is typed in it, and the
    step's value again once it has not (`useTyping` in StepNode.tsx): the
    value comes back through React Flow's store, and a box set straight
    from it moved the cursor to the end, so typing in the middle went to
    the end (the user's report, 2026-10-25). A field input with a picker
    (Socket.picker: `datetime`, `date` or `time`; the datetime steps, and
    every post step's At since 2026-10-25) is
    the browser's own picker (`<input type="datetime-local">`, `date`,
    `time`) with its name before it, rather than a text box. An input
    with a when (Socket.when, "Source=File": the post steps' Text and File,
    2026-10-25) shows only while that setting has that value (its first
    option when unset), or while it is linked, so a link keeps its end. The
    name, the Active switch and selection are not part of it, and the
    history ends with the editor. A change is saved
    0.7 s after the editing pauses (and when the editor closes); the header
    says Saved, Unsaved changes, Saving... or why it was not saved. The
    steps are drawn from the kinds the tower has (ListStepKinds); one the
    engine cannot run yet says "soon". A step's Connection setting is a
    select of the connections of its type, and its Storage setting a select
    of the storages (whose empty choice is the setting's placeholder when it
    has one: File Path's "No storage: the data folder"); one that is gone
    shows as such. Run gives a trigger step data typed
    in a dialog (JSON, or any text; a Schedule step takes none, as if its
    time came now) and runs the workflow; the Active switch turns its
    triggers on, and the line under the header says whether each listens,
    or for a Schedule step when it runs next (asked again once that time
    has come). Both save first, since the tower runs the saved steps. The
    editor has three tabs under its header (the user, 2026-10-25, in place
    of a Runs button and panel; Logs added the same day): Workflow, the
    canvas, Runs (`?tab=runs`): the runs, newest first (checked every 3 s), and beside
    them the picked one step by step (error, fired outputs, log lines, data
    outputs). The picked run, or the one Run just made, is also marked on
    the canvas (a badge on each step that ran), with a chip whose ✕ stops
    it. Logs (`?tab=logs`) is the Workflow page's Logs (`RunLog`) for this
    workflow alone: its runs as they come, switched on or not, with their
    log lines, without the workflow's name on each. The canvas stays
    mounted while Runs or Logs shows, so nothing typed or undoable is
    lost; there Undo, Redo, Add and the canvas's keys are off.
  - `NotFound.tsx`.
- `src/workflow/`: `catalog.ts`, the step kinds (loaded once from the
  tower), the connections a step can pick (asked once per type, forgotten
  when the Connections page changes one) and the storages (asked once,
  forgotten when the Storage page changes one), the socket colours and category
  palettes (Datetime, the datetime steps, is teal; Social, the San Remote
  steps, is pink; AI, ChatGPT Ask, is yellow), and the conversion
  between a stored workflow and React Flow's nodes and edges (a handle's id
  is `<type>:<socket>`); `StepNode.tsx`, how a step is drawn; `history.ts`,
  the editor's undo and redo (`useHistory`): the graph as saved (JSON)
  after each change, recorded once the editing pauses for 0.5 s and not
  during a drag, up to 200 back; `runs.tsx`,
  run statuses, durations and output values, log lines in their type's
  colour (`LogLine`), and the context that marks the picked run on the
  canvas; `RunsPanel.tsx`, the editor's Runs tab and
  `RunDetail` (what each step of a run did), which the Logs share;
  `RunLog.tsx`, the Workflow page's Logs: the runs of every workflow as
  they come, newest first, asked for every 2 s (ListRuns with no workflow,
  which gives each run its workflow's name and the lines its steps logged;
  with a `workflowId`, the editor's Logs, ListRuns with that workflow and
  `logs`, and no "Active workflows only").
  "Active workflows only" (on by default) keeps to the workflows switched
  on; All / Succeeded / Failed keeps to runs of that status (the tower
  filters; Failed takes in interrupted runs). A run shows its time, status, workflow (a link to its editor),
  trigger and duration, with its log lines under it and, failed, its
  error; a run that just came in is marked for 3 s. Clicking a run opens
  `RunDetail` (fetched again when it finishes). Pause stops asking, so
  the list stands still to read; "Live" says when it last asked.
  `RenameDialog.tsx`, the rename dialog the list and the editor share;
  `AddPalette.tsx`, the Add menu as Blender's Shift+A (the user's choice,
  2026-10-25), a large popup (up to 70% of the window's height, 420 px
  columns; the rows in it stay compact, as the user asked): a search field
  that has the keys at once, and menus
  that open in columns beside each other, category › group › step
  (Social › Threads › Threads Post; a StepKind's `group`, from the tower;
  a step with none is right in its category, after the groups). A menu
  row shows its icon, name and how many steps it holds; a step row its
  icon in its category's colour, name and description (StepKind's).
  Hovering a menu, or moving to it, shows what it holds in the next
  column. Typing searches every step (title, description, category, group;
  titles that start with it first), each result saying where it is. ↑↓
  move, → or Enter open a menu, ← goes back, Enter adds a step, Esc or a
  click outside closes. The pointer selects only once it moves, so a menu
  that opens, or moves in to stay on screen, under a still pointer leaves
  the keys in charge. Icons: `categoryIcons`, `groupIcons` and `kindIcons`
  (a kind or group without one shows its category's).
- `src/ui.tsx`: the toaster, and `FormDialog`, a form in a modal that shows
  the server's error and closes when it succeeds.
- `src/api.ts`: the TowerService, BrokerService, WorkflowService,
  ConnectionService, BrowserRemoteService, StorageService, ChatService and MessageService clients, same origin as the page,
  and `errorText`. Their
  JSON reading knows the types a run's `Any` outputs hold (Value,
  StringValue).
- `src/gen/`: generated from the protos; do not edit.
