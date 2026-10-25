# san_tower

The tower: one place to watch the whole project. `san_tower serve` runs, on
http://127.0.0.1:7477:
- TowerService (Connect RPC), which says the tower is up;
- WorkflowService: the workflows (graphs of steps, like n8n's but the
  user's own) in the tower's database (SQLite through GORM,
  `<data>/tower.db`), and the engine that runs them, by hand or from their
  triggers (see Workflows below);
- ConnectionService: the connections to other programs that workflow steps
  use, first San Remote (see Connections below);
- StorageService: the storages, named base folders that workflow steps use
  (see Storage below);
- the San Message broker ([golang/packages/san_message](../../packages/san_message/README.md))
  with its MessageService, so apps publish and subscribe at the tower's
  address: embedded in the tower, or a separate `san_message server` at a
  URI, on this machine or another, whose calls the tower passes on (see
  Message broker below);
- the tower's UI ([typescripts/san_tower_frontend](../../../typescripts/san_tower_frontend),
  built into the binary). Home shows the tower's status; Message Broker
  shows the broker's status and manages its topics and subscribers; Workflow
  lists, creates, edits, runs, switches on and deletes the workflows;
  Connections manages the connections; Storage the storages; Browser Remote Server switches on
  the San Remote server inside the tower (see below); Settings holds the
  tower's settings
  (which broker: inside the tower with its data directory, or a separate
  server's URI).

What else the tower watches is still to be decided.

Module `github.com/wargasipil/san_tower`, Go 1.26. It uses san_message, and
san_remote's generated clients, from this repository (`replace` in go.mod).

```powershell
pwsh build.ps1                          # UI build + vet + test + bin/san_tower.exe and bin/san_tower
bin\san_tower.exe serve                 # then open http://127.0.0.1:7477
go run .\cmd\san_tower dev              # tower + the UI's Vite dev server; open Vite's address (http://localhost:5173)
bin\san_tower.exe status                # asks the running tower (--server, $env:SAN_TOWER_SERVER)
# after editing a proto: Go and TS for TowerService, then TS for san_message's MessageService
buf generate; buf generate --template buf.gen.san_message.yaml
```

The broker's CLI works against the tower too:
`go run ../../packages/san_message/cmd/san_message --server http://127.0.0.1:7477 topic list`.

## Workflows

A workflow is a graph of steps joined by links between typed sockets:
`event` links say which step runs next, `object` (a google.protobuf.Value)
and `text` (a StringValue) links carry data. The engine (`internal/engine`)
defines the kinds of step, each with a one-sentence description for the
editor's Add menu (a test checks every kind has one) and, for a step of one
site or service, a group, its sub-menu in its category (Social › Threads,
Social › Telegram); the UI asks for them
(ListStepKinds). All of them run: Topic Event, Schedule and Telegram
Message (triggers), Filter, Prefix Filter, Regex Filter, Pick Greater, Template, Format
Text, Storage, File Path, Read File, Glob, UUID, MD5 Hash, Stringify, Now, Datetime,
Datetime Operation, String to Datetime, HTTP Request, Publish to Topic, Download File, Delete File, Delete Folder and Log,
and
the San Remote and Telegram steps (Social, below).

- A run starts at a trigger step, which fires its event outputs (Topic
  Event: Event, and Payload is the event's data, JSON as it is, other text
  as a string). A step runs when an event link from a fired output reaches
  it, then fires its own. A data input takes the linked output's value: from
  a step that already ran, from a pure step (no event input, such as
  Template) run then, or the value typed in the step for an unlinked field
  input. The first step that fails ends the run; a run stops after 1000
  steps, against a loop.
- A loop (Glob's Each, 2026-10-25) fires one event output once per item:
  each time, the steps it links to run to the end of their branch before
  the next item, reading the item from the step's outputs (Item). Pure
  steps that read it (also through other pure steps) are worked out again
  for each item; the others keep their once-per-run value. A step in the
  branch that fails ends the run as anywhere else: the looping step's
  Failed is for its own failure. The editor shows each looped step's last
  run.
- Every step with a Done output (HTTP Request, Publish to Topic, the San
  Remote and Telegram steps) also has In Progress, Failed and Error (the
  user, 2026-10-25, for easy wiring). In Progress fires as the step starts:
  the steps it links to run, to the end of their branch, before the step
  does. When the step fails and Failed is linked, Failed fires instead of
  Done, with Error (text) the error, and the run goes on (it ends Succeeded
  unless a later step fails); the step is still recorded with its error.
  With Failed not linked, the run fails as before.
- Expressions are CEL (`cel.dev/cel-go`, pure Go): Filter's If is a
  condition over `payload` (`payload.total > 1000000`), Template's Text puts
  `{{ expression }}` values in text. Numbers from JSON are doubles and
  compare with integers.
- Format Text puts its Text input (e.g. a Telegram Message's Text) into
  Format, where `{{ text }}` stands for it: `pesan "{{text}}"`. The
  `{{ }}` are CEL too, over the variable text (`{{ text.size() }}`,
  `{{ text.startsWith("/") ? "command" : "chat" }}`); Template's are over
  payload.
- Prefix Filter and Regex Filter test a text input (e.g. a Telegram
  Message's Text) and go on by Match or Else, as Filter does. Prefix Filter:
  the text starts with Prefix, as typed (case and a trailing space count:
  "/say "). Regex Filter: the text matches Pattern, a Go regular expression
  (RE2: `^/(start|help)\b`; `(?i)` ignores case), found anywhere unless
  anchored. An empty Prefix or Pattern, or a Pattern that does not compile,
  fails the step.
- Pick Greater (Logic, pure; `pick.go`; the user's ask, 2026-10-25) gives
  the greater of its A and B field inputs as Value, as it came: two numbers
  as numbers (3 and 10 give 10, not "3"), or two times, the later one (a
  Datetime in any zone, a Date, a time alone for today). An empty one gives
  the other, so a Last Scheduled Post that found nothing leaves Now; a tie
  gives A. Anything else, such as a number and a date, fails the step.
- Storage, UUID and MD5 Hash (Data, the user's ask, 2026-10-25) are pure
  steps: they run when a step takes one of their outputs, once per run.
  Storage gives the storage its Storage setting picks (Storage below): Path
  as set on the Storage page, and Absolute Path, where it is (e.g. for
  Vidara Download's Save To, which san_remote reads from its own folder). A
  storage that is gone fails the step. UUID makes a random version 4 UUID
  (crypto/rand), so every step that takes it in a run gets the same one and
  the next run a new one. MD5 Hash gives the MD5 of its Text field input,
  as 32 lowercase hexadecimal digits.
- File Path (Data, pure) is a file's path from its File Name (a field
  input, e.g. a UUID through Format Text: `{{ text }}.mp4`): Path and
  Absolute Path, with / as storages have. With a Storage picked, the name
  is inside it (`tmp/videos` + `test.mp4`: `tmp/videos/test.mp4` and
  `<data>/tmp/videos/test.mp4`), and a name that leaves it (`../`, an
  absolute path) fails the step. With none ("No storage: the data
  folder"), a relative name is in the tower's data folder and an absolute
  one is itself. It only makes the path: it creates and checks nothing.
- Read File (Data, pure; `filepath.go`; the user's ask, 2026-10-25) gives
  the text of the file at Path as Text. Path is placed as File Path places
  a name (Storage setting, else the data folder, or absolute). UTF-8 text
  up to 10 MB: a byte order mark is dropped and Windows line ends become
  `\n`. A file that is not there, a folder, a bigger file or a binary one
  fails the step. Linked from Glob's Item, it reads each file in turn.
- Glob (Data; `glob.go`; the user's ask, 2026-10-25) finds the files
  matching Pattern, a field input placed as File Path places a name (in
  the picked storage, the data folder, or absolute): `articles/*/*.md`;
  `*`, `?` and `[ab]` match within one name and `**` any number of folders
  (`videos/**/*.mp4`). Files only, sorted, absolute with /. A pattern
  without them is that one file when it is there; a folder before the first
  pattern that is not there fails the step. It is a loop: Each fires once
  per file with Item its path, then Done fires once. Files (a list), First
  and Count are set before the loop.
- Download File (Action; `download.go`; the user's ask, 2026-10-25) GETs
  URL (http or https) into Save To, placed the same way: a file, or a
  folder (one that is there, a path ending in /, or empty for the storage or
  the data folder itself), where the file takes the server's name
  (Content-Disposition) or the URL's last part. Folders are made, a file
  there is replaced, and it is written to a `.part` file beside it first,
  so a failed download leaves nothing. A 4xx or 5xx fails the step. File
  is where it went; it waits a minute at most for the answer to start, and
  as long as the file takes.
- The datetime steps (category Datetime, their own group in the Add
  menu; pure; `datetime.go`; the user's ask,
  2026-10-25) give a time three ways, in the tower's local time: Datetime
  in RFC 3339 (`2026-10-25T09:00:00+07:00`, which the post steps' At and
  CEL's `timestamp()` take), Date (`2026-10-25`) and Time (`09:00`). Now is
  the time the run asks for it. Datetime puts a Date and a Time together
  (no date: today; no time: midnight). Datetime Operation adds an Amount
  (a whole number) of Minutes, Hours, Days, Weeks, Months or Years to a
  Datetime, or subtracts it (days and longer by the calendar: 09:00 stays
  09:00). Their datetime, date and time inputs are pickers in the editor
  (Socket.picker: `datetime` 2026-10-25T09:00, `date`, `time` 09:00) but
  text sockets all the same, so they link to and from any text; a
  datetime input also takes RFC 3339, a space for the T, seconds, or a
  date alone.
- String to Datetime (Datetime, pure) reads a time from its Text and gives it
  as Value, in the form To picks: Datetime (RFC 3339), Date or Time. With
  no Format it reads what the other datetime steps take, a time alone
  (today at it), and Unix seconds or milliseconds (all digits, as a
  Telegram message's date). A Format reads anything else, in tokens:
  YYYY, YY, MMMM (January), MMM (Jan), MM, M, DD, D, HH (24-hour), hh and
  h (12-hour, with A for AM/PM), mm, ss; other characters are taken as
  they are, so `DD/MM/YYYY HH:mm` reads 20/11/2026 09:00. Month names are
  English.
- Claude Ask (CLI) (AI › Claude; the user's ask, 2026-10-25) asks Claude
  with this computer's Claude Code: `claude -p --output-format json --tools
  "" --strict-mcp-config --no-session-persistence` with the Prompt on stdin,
  in the temporary folder, as the knowledge tool's AI step runs it, or in
  Directory, an absolute folder, when it is set (the user's ask: a base
  project directory, the working directory of the call), so that project's
  CLAUDE.md and settings come in. Claude
  gets no tools or MCP servers (a prompt may come from a message
  anyone sent), and it is the user's own Claude Code login. Model is
  Default, Sonnet, Opus or Haiku; Wait bounds the answer (default 10m).
  Answer is its reply; an error Claude Code reports (a usage limit, not
  signed in) fails the step with it.
- Stringify (Data, pure) writes its Object input as JSON text: Compact on
  one line or Pretty, indented by two spaces; keys sorted, whole numbers
  without a decimal point, `< > &` as they are (it is for messages), and
  null when nothing is linked. Template's `{{ payload }}` gives the compact
  form too.
- Delete File (Action) deletes the file at Path (a field input, e.g. File
  Path's Absolute Path; a relative path is in the tower's data folder) and
  goes on by Done. A file that is not there counts as done too, since it
  is gone either way (the user's choice), with a log line saying so ("...
  was not there"). Files only: a folder is refused.
- Delete Folder (Action; the user's ask, 2026-10-25) deletes the folder at
  Path with everything in it. Path is placed as File Path places a name:
  in the picked Storage (and inside it), else in the data folder, or
  absolute. A folder not there counts as done, with a log line, as for
  Delete File; a deleted one logs how many files it held. Refused, so a
  slip cannot wipe the tower or a disk: a file (Delete File's), a drive's
  root, the tower's data folder or any folder holding it, and the picked
  storage's own folder (give a folder inside it).
- HTTP Request sends its Body (when linked) as JSON with its Method to its
  URL, whose `{{ expressions }}` read Body as `payload`; GET sends no body.
  Response is the answer: status, headers and body (JSON when it is JSON).
  A 4xx or 5xx fails the step, with its Response still in the run; it waits
  30 s at most.
- Publish to Topic publishes Payload on a topic of the tower's broker: an
  object as JSON, text as it is, nothing when unlinked. Result is the topic
  and the event's id. A workflow that publishes to its own trigger's topic
  runs again on each event it publishes.
- Social steps (category Social, and AI for ChatGPT, Gemini and Claude)
  work on the user's Threads, X, Facebook, Vidara, Medium, LinkedIn, TikTok,
  ChatGPT, Gemini and claude.ai through
  `san_remote api`, the San Remote connection their Connection setting names:
  - Threads Post, X Post, Facebook Post schedule a post: Text (lines that
    are only `---` split a thread into posts; Facebook takes one) and At.
    At is a datetime picker on every post step (the user, 2026-10-25), or a
    linked time (Now, Datetime Operation; RFC 3339 or local) or delay from
    the run ("+90m", "+1d"). It is rounded up onto the times the site
    offers: a whole minute (Threads, X, Medium), a 5-minute mark (Facebook,
    TikTok), a quarter hour (LinkedIn), since a time from Now has seconds
    that X refused ("not on a whole minute"). They only schedule (the
    user's choice, 2026-10-24); an empty At fails the step. A post over
    the site's length (X: 280 in its count without Premium) fails the step
    too, before anything is posted: it is neither split into a thread nor
    cut (the user's choice, 2026-10-25). Threads takes a
    Topic for the first post; Facebook a To (profile, or `page: <name>`)
    and an Audience. Result is when the site will publish it.
  - Format and Source (the user's ask, 2026-10-25), on Threads, X, Facebook
    and TikTok Post: Format is Plain (the text as it is, the default) or
    Markdown, and Source is the text input (Text, TikTok's Caption) or File
    (an absolute path, such as File Path's), which the editor shows instead.
    A plain File is read by the step. Markdown goes to San Remote, which
    reads it as its post files: frontmatter (`at:`, Threads' `topic:`,
    Facebook's `to:` and `audience:`, TikTok's `video:`), `---` between
    posts, `![](image)` attached to its post (relative to the file; text
    needs absolute paths), and the rest written as plain text, since the
    sites have no markup: `**bold**` is bold, `# Title` is Title,
    `[text](url)` is "text (url)", lists keep their markers. So an AI step's
    markdown Answer can be posted as it is. With Markdown, At (and TikTok's
    Video) may be left empty for the frontmatter's; what the step gives
    wins.
  - Vidara Upload uploads File (a path on this computer) with its Title.
  - X Post Download (Social › X; `xdownload.go`; the user's ask,
    2026-10-25) reads anyone's post from its Link (or id) through San
    Remote, in a background tab, and saves its images and videos (each
    video's best mp4), and an X Article's cover and images, in Save To: a
    folder placed as File Path places a name (Storage setting; empty is the
    storage or data folder itself). Caption is the post's text, Text an X
    Article as markdown (images pointing at the saved files) or else the
    caption, Title the article's title, Tags the hashtags as `#a #b`, Media
    the saved files (a list), First the first of them, Post all of it.
    Markdown (the user's ask, 2026-10-25) is the post as markdown: an
    article's Text, or the caption with each saved image and video under it
    as `![alt](file)` (absolute paths, in `<>` when they have a space), so a
    Threads, X or Facebook Post with Format Markdown posts it again with its
    media. First links into TikTok Post's Video; Text or Markdown into a
    Medium or LinkedIn Article with Source = Text.
  - Vidara Download saves the video Link names (https://vidara.to/v/...,
    or its code) to Save To, a folder on this computer, where the file is
    named after the video's title, or a file; it must be an absolute path,
    and an existing file is not overwritten. File is the path written (an
    .mp4 when ffmpeg is on the PATH, else a .ts), Result also has its title,
    stream ("1280x720, 3.0 Mbps") and size. It is Vidara's own encoding of
    the video, read through its player in a background tab, and takes as
    long as the stream does.
  - Medium Article (Social › Medium) writes File, a markdown article (an
    absolute path), or with Source Text the Article input (markdown whose
    images need absolute paths), into a Medium draft, images included; with At it is
    scheduled too, with Topics ("Programming, Golang", as Medium suggests
    them). Without At it stays a private draft. Link is the draft's.
  - LinkedIn Article (Social › LinkedIn) does the same in LinkedIn's article
    editor (File, or the Article text), with an optional Cover image
    (absolute path); with At (on a
    quarter hour) it is scheduled, shared with Text.
  - TikTok Post (Social › TikTok) schedules Video (an absolute path) with
    Caption for Everyone at At, on a 5-minute step up to 30 days ahead;
    `#hashtags` are picked from TikTok's suggestions.
  - ChatGPT Ask (AI › ChatGPT) asks ChatGPT the Prompt in a Temporary chat,
    which ChatGPT does not keep (the user's choice, 2026-10-25); Answer is
    its reply as markdown. Wait bounds a long answer (default 10m).
  - Gemini Ask (AI › Gemini) does the same on Gemini, in a Temporary chat;
    it brings Chrome to the front while it asks, since Gemini sends a prompt
    only from a shown page.
  - Claude Ask (Browser) (AI › Claude) does the same on claude.ai, in an
    incognito chat kept out of the history, in a background tab; Answer is
    markdown. Social Status reads ChatGPT, Gemini and Claude too.
  - Chat, on those three (the user's ask, 2026-10-25): Leave, the default,
    closes the tab when the ask is over; Stay leaves it open on the chat,
    to read on or go on in it. A Temporary or incognito chat is gone once
    that tab is closed.
  - Temporary, on those three (the user's ask, 2026-10-25): Yes, the
    default, asks in a Temporary chat (incognito on claude.ai) that the site
    does not keep; No asks in a normal chat, kept in the history.
  - Social Status says who is signed in on a Site (ChatGPT too). Social Posts
    lists a Site's Scheduled or Published posts, or Drafts (Facebook, Medium,
    LinkedIn); Vidara lists its videos, and TikTok's scheduled posts are the
    ones still ahead in Studio's list. Last is the last scheduled one (the
    latest time) or the newest published one, or for Facebook, drafts and
    Vidara the first listed.
  - Last Scheduled Post finds a Site's (Threads, X, Facebook, Medium,
    LinkedIn or TikTok; Text is an article's title or a video's caption) scheduled
    post with the latest time and goes on by Found, or by None when
    nothing is scheduled, and then by Done either way. Post is it, Text its text, and Datetime, Date
    and Time its time as the datetime steps give it (Datetime in RFC 3339,
    local, which a post step's At takes; an At output before them, the
    user's ask, 2026-10-25). Facebook's list has no times, so there it is
    the first listed and the three are empty. For a post the day after:
    Datetime Operation (Add 1 Days) on Datetime, into the post step's At.
  - A post or an upload brings Chrome to the front for a minute or two.
- Telegram steps work as a bot, through the Telegram Bot connection their
  Connection setting names (the Bot API, over plain net/http,
  `internal/telegram`):
  - Telegram Message (a trigger) starts a run for each message the bot
    gets, in any chat or in the one its Chat names (a chat's id or
    @username). Message is the message as Telegram sent it (`payload.text`,
    `payload.chat.id`, `payload.from.username`), Text its text (or a
    photo's caption), Chat its chat's id, which a Telegram Send can take to
    reply. A run's trigger reads "telegram, chat @heri, message 1010".
  - Telegram Send (Social › Telegram) sends Text to Chat (a chat's id, or a
    channel's @username), Plain, HTML or Markdown (Telegram's legacy
    Markdown), as a reply when Reply To has a message id. Message is the
    message sent.
  - Telegram Messages (Social › Telegram) lists the newest messages the bot
    got, in every chat or in Chat; Last is the newest. The Bot API cannot
    read a chat's history, so these are the messages the tower kept: what
    the bot received since its connection was made, the newest 1000 per
    connection.
  - The tower reads each bot connection's updates all the time it runs,
    used by a workflow or not (getUpdates, long polling, 50 s at a time),
    keeps each message, then starts the runs; an update is confirmed once
    that is done. Telegram keeps a bot's updates for 24 hours, so messages
    sent while the tower is stopped run when it is back, if within a day;
    one sent again after a restart is kept and run once. A bot has one
    reader: another program reading the same bot, or a webhook set on it,
    makes Telegram answer 409 Conflict, which the trigger's state shows. In
    a group the bot gets every message only with its privacy mode off
    (@BotFather, /setprivacy); otherwise commands, replies and mentions.
- Runs go on in the background under the engine's own context, so a step
  that takes minutes is not cut off by whatever started it; stopping the
  tower cuts them short and records them as interrupted.
- By hand: RunWorkflow (the editor's Run) gives a trigger step data and
  waits for the run.
- From triggers: a workflow switched on (SetWorkflowActive) makes each of
  its Topic Event steps a pull subscriber, `workflow-<id>-<step id>`, on its
  topic in the broker the tower uses, and the engine keeps a Subscribe
  stream open for each: to the tower's own address when the broker is
  inside it, or straight to the separate server, which so needs no way back
  to the tower (the tower only listens on loopback). An event is acked once
  its run is recorded as started, and the run follows; a failed run is
  still a done event. An event for a workflow or step that is gone or
  switched off is acked and skipped. Only a failure to record the run, or a
  tower that is stopping, leaves it unacked: the stream ends, is opened
  again after a wait (1 s doubling to 30 s), and the broker sends the event
  again. The subscribers live in the broker, so events that come while the
  tower is down run once it is back. The triggers follow every save of an
  active workflow, are rebuilt on start and when the tower switches
  brokers, and say whether they listen (Workflow.triggers: "topic orders
  does not exist", "not connected to the broker: ...").
- Schedule steps of a workflow switched on are timers in the engine
  (`schedule.go`), with no broker. Every is a time between runs, a Go
  duration that may start with days ("5m", "90m", "1d", "7d"), at whole
  multiples of it in local time whenever the tower started (one that
  divides a day from midnight: "5m" at :00, :05, ...; whole days at
  midnight, "7d" on Mondays); or a cron line in local time (robfig/cron:
  five fields, "0 9 * * *", "*/15 9-17 * * 1-5", or "@daily", "@hourly").
  Runs are at least a minute apart. Payload is the time the run was due,
  `{"time": "2026-10-25T09:00:00+07:00"}` (now, when run by hand), and the
  run's trigger reads "schedule, every 5m". A time missed while the tower
  was stopped, or the computer slept, is skipped, not made up; a time that
  comes while the run before it is still going is skipped too, so a post
  slower than its Every is not made twice at once. An edit that keeps
  Every keeps the timer; the step's state says when it runs next
  (TriggerState.next) or why it cannot ("Every \"soon\": give a time between
  runs ...").
- Every run is kept with what each step did (its error, the event outputs
  it fired, its log lines, its data outputs as google.protobuf.Any): the
  newest 100 per workflow. A run still going when the tower stops is
  marked interrupted on the next start.

## Connections

A connection is how workflow steps reach another program, managed on the
Connections page (ConnectionService) and kept in `tower.db`: a name (unique,
ignoring case), a type, and the type's settings (ConnectionSettings, one
field per type, stored as protojson). A step picks one in a setting with a
connection type; its value is the connection's id, and a step whose
connection is gone fails, saying so.

- **San Remote**: the URL of `san_remote api` (default
  http://127.0.0.1:7478; golang/apps/san_remote), which posts through the
  user's own browser and needs `san_remote serve`. Test (TestConnection)
  lists the browsers attached behind it, without opening anything in them.
- **Telegram Bot**: a bot's token from @BotFather (`/newbot`), checked for
  its shape (`123456:ABC-DEF1234`). Test asks the Bot API which bot it is
  (getMe), which reads no messages. Deleting the connection deletes the
  messages it kept. The token is a secret: it is stored and listed as it
  is (the tower only answers on loopback), and kept out of errors.

## Storage

A storage is a named base folder on this computer, which workflow steps use
(the user, 2026-10-25), managed on the Storage page (StorageService) and kept
in `tower.db`: a name (unique, ignoring case) and a base path, absolute
(`D:/videos`) or relative. A relative path is inside the tower's data folder,
as the broker's data directory setting is (`videos` is `<data>/videos`);
ListStorages says where that is, and every storage comes with its absolute
path. Paths use `/` between folders, on Windows too (the user, 2026-10-25):
a `\` typed is saved as `/`, one saved earlier is given out with `/`, and the
absolute path and the data folder come with `/` (Windows takes both). The
tower does not create, check or delete the folder: deleting a
storage leaves it as it is (Vidara Download makes the folders it saves in). A step picks a storage in a setting with
`storage` set; its value is the storage's id.

## Data

`--data` (default `data`, relative to where the tower runs; `$env:SAN_TOWER_DATA`)
is the tower's folder, git-ignored. It holds:
- `tower.json`, the settings changed from the UI. A broken file stops the
  tower from starting rather than falling back to the defaults.
- `tower.db` (with its `-wal` and `-shm` files), the tower's SQLite
  database: the workflows, their runs, the connections, the storages and
  the messages the Telegram bots got. Its tables are brought up to date
  when the tower starts (GORM's AutoMigrate).
- the topic and subscriber metadata and event logs of the broker inside
  the tower, in `<data>/san_message` unless the broker's data directory
  setting names another folder (Settings > Message Broker). A relative
  setting is inside `<data>`; an absolute one can be anywhere whose parent
  folder exists.

## Browser Remote Server

For when nothing else runs san_remote on this machine, the tower can run it
(the user, 2026-10-25): the hub the San Remote browser extension attaches to,
on `127.0.0.1:7476`, and the site services (Threads, X, Facebook, Vidara,
Medium, LinkedIn, TikTok, ChatGPT, Gemini, Claude), on
`127.0.0.1:7478`, as `san_remote serve` and `san_remote api` run them
(through san_remote's `server` package), so the extension, the CLI, the MCP
server and a San Remote connection find it at the usual addresses. The
Browser Remote Server page switches it on and off
(BrowserRemoteService), saved in `tower.json` as `browser_remote.active`, so
it starts with the tower; it shows the server URI to paste in the
extension's Server field, the API URI, and the attached browsers. When an
address is in use (san_remote already runs), it does not start and the page
says why. While it does not run in the tower, the page asks both addresses
whether a san_remote run outside the tower answers there, and shows the
browsers attached to that one (the user, 2026-10-25: it showed Off and no
browser while `san_remote serve` ran with Chrome attached). A fix to a site's steps needs the tower rebuilt, as the site code
is in the tower's binary.

## Message broker

The tower uses one broker at a time (Settings > Message Broker, saved in
`tower.json` as `broker.remote`, `broker.uri` and `broker.data_dir`, all
kept whichever is in use):
- Inside the tower (the default): its own san_message broker, in the data
  directory.
- A separate server: a `san_message server` at a URI, http or https, such
  as `http://10.8.0.5:7070`. On another machine it must listen on an
  address this one reaches (`san_message server --listen 10.8.0.5:7070`,
  its VPN address); it has no access control of its own. The tower passes
  the MessageService calls made to it on to the server one by one (a
  Subscribe stream both ways), so the UI and the apps keep using the
  tower's address; apps may also go to the server directly. Its status
  shows the server's address instead of a size on disk.

Switching happens at once. The new broker is opened, or asked for its topics
(5 s), first; if it cannot open or does not answer, nothing changes. Then
MessageService goes to it, the broker inside the tower that was in use
closes (its pull subscribers are disconnected and reconnect), and the
workflows' triggers are made again in the new broker, at the end of its
topics. Nothing is moved or deleted: the topics and subscribers are those
of the broker in use, none in a new folder or server. A separate server
that stops answering does not stop the tower: the calls that reach it fail
with Unavailable, the status says Not answering, and the triggers' streams
keep trying.

San Message does not lock its folder, so point one tower at each broker
folder. Two towers on the same `--listen` cannot both start, since the
second fails to listen before it opens the broker.

## Files

- `protos/san/tower/v1/`: `tower.proto` (TowerService), `broker.proto`
  (BrokerService: the broker's settings, inside or a separate server, and
  its status: since when it is in use, its size on disk or the server's
  URI, topics and events published, subscribers, pull ones connected,
  those behind and those retrying) and `workflow.proto`
  (WorkflowService: the step kinds; list, create, get, update, switch on,
  delete; run, and list and get runs, one workflow's or, for the Workflow
  page's Logs, every workflow's or the active ones', and of any status or
  some, each then with its workflow's name and its log lines and their
  types (LogLevel, the Log step's Type: info, success, warning, failed,
  debug); a workflow's steps and links, and the
  Graph and RunSteps messages the database stores; a Setting with `storage`
  picks a storage), `connection.proto`
  (ConnectionService: list, create, update, delete, test; ConnectionSettings
  per type: San Remote, Telegram Bot) and `storage.proto` (StorageService:
  list, with the tower's data folder, create, update, delete; a storage's
  absolute path). `buf.gen.yaml` makes their Go
  code (`gen/`) and the UI's TS client. `buf.gen.san_message.yaml` makes
  the UI's TS client for san_message's proto; the Go side uses san_message's
  own generated code.
- `cmd/san_tower/main.go`: the CLI (`serve`, `dev`, `status`).
  - `serve` and `dev` open the database and the broker, then serve the
    tower with `san_message.ConfigureServer` (h2c, for the broker's
    Subscribe stream, and pings), and once it listens start the engine with
    the tower's own address (where the triggers stream from the broker
    inside it). On stop the engine closes first (the triggers' streams,
    then the runs), then the broker inside the tower, which ends the
    Subscribe streams still open.
  - `dev` serves the tower on `--listen` and runs Vite (`node
    node_modules/vite/bin/vite.js`, not npm, so stopping it leaves nothing
    behind) in the frontend's folder, found from the current folder up
    (`--frontend` to give it). `npm ci` runs first if `node_modules` is
    missing. Vite's proxy goes to this tower through `SAN_TOWER_SERVER`.
    When either one stops, so does the other; Ctrl+C stops both. A UI
    change reloads by itself; a Go change needs a restart.
- `internal/server`: the HTTP server. It serves TowerService, BrokerService,
  WorkflowService (`workflow.go` the database's half, `engine.go` the
  engine's), ConnectionService (`connection.go`), BrowserRemoteService
  (`browser_remote.go`), StorageService (`storage.go`), MessageService and the UI
  behind `Guard`, which only accepts a loopback Host and loopback pages.
- `internal/engine`: `kinds.go` the kinds of step and how each runs,
  `actions.go` HTTP Request and Publish to Topic, `social.go` the San Remote
  steps (clients of `san_remote api`, the At times), `sites.go` the Medium,
  LinkedIn, TikTok, ChatGPT, Gemini and Claude (Browser) steps, `claude.go`
  Claude Ask (CLI), `run.go` running a
  graph (`Execute`, with an `Env` for connections, storages and publishing), `expr.go`
  CEL, `engine.go` runs (recorded in the store), the triggers' pull
  subscribers and their streams, and `publish` for Publish to Topic,
  `schedule.go` the Schedule steps' timers and their Every, `telegram.go`
  the Telegram steps and the bots' pollers (`SyncConnections`, after a
  connection changes).
- `internal/telegram`: the Bot API client (getMe, getUpdates,
  sendMessage); `telegramtest` is a fake Bot API for tests, which reach no
  Telegram.
- `internal/broker`: the broker the tower uses, inside it or a separate
  server. Its MessageService handler goes to the broker in use at each call,
  so `Set` can switch while the tower runs; for a separate server it is a
  relay (each call made to the server, a Subscribe stream relayed both
  ways). `NewClient` is a MessageService client that speaks h2c, as the
  streams need. `Publish` and `Status` work on either; `Status` counts from
  the topics and subscribers: a subscriber is behind when its pointer is not
  at its topic's end, retrying when it has a retry time.
- `internal/browserremote`: the San Remote server the tower runs when it is
  switched on: both addresses are taken before it serves, so one in use is
  an error rather than a server half up; off, it ends the browsers' streams
  and shuts down.
- `internal/settings`: `tower.json`, written whole through a temporary file;
  `Update` serializes the changes the parts of the tower save.
- `internal/store`: the database, `tower.db`. GORM with glebarez/sqlite (on
  modernc.org/sqlite, pure Go, so the build stays without cgo), WAL, one
  connection. A workflow is a row: id, name (unique, ignoring case, at most
  100 characters), its steps and links as one column (the proto Graph as
  protojson, through the `protojson` serializer in `protojson.go`; graphs
  saved earlier as snake_case JSON still load), active, created and updated
  times. An update checks that steps have an id of their own and a kind,
  and that links join steps that are there. A run (`run.go`) is a row too,
  its steps one protojson column (RunSteps). A connection
  (`connection.go`) is a row: name, type (a column, for lists of one type)
  and its ConnectionSettings as protojson. A storage (`storage.go`) is a
  row: name and path as given; the store knows its own folder (the data
  folder, made absolute), so it fills in each storage's absolute path
  (`AbsPath`) as it reads it. A Telegram message
  (`telegram.go`) is a row: connection, update id (kept once), chat id and
  @username, date, and the message's JSON. Errors wrap `ErrNotFound`,
  `ErrInvalid` or `ErrExists`, which the server turns into Connect codes.
- `internal/web`: the embedded UI. `dist/` is the Vite build (git-ignored but
  for `.gitkeep`). Paths without a file extension get `index.html`, so the
  UI's routes can be reloaded and linked to. Before the first UI build the
  page answers 503.
