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
- ChatService: the tower's own chat, chats with workflows that have a Tower
  Chat step (see Chat below);
- KnowledgeService: the knowledge, each with a storage of its own, in
  folders (see Knowledge below);
- the San Message broker ([golang/packages/san_message](../../packages/san_message/README.md))
  with its MessageService, so apps publish and subscribe at the tower's
  address: embedded in the tower, or a separate `san_message server` at a
  URI, on this machine or another, whose calls the tower passes on (see
  Message broker below);
- the tower's UI ([typescripts/san_tower_frontend](../../../typescripts/san_tower_frontend),
  built into the binary). Home shows the tower's status; Chat, after it,
  the chats with workflows; Knowledge, after Chat, lists, creates, moves
  and deletes the knowledge and their folders; Message Broker shows the broker's status and manages its topics and subscribers;
  Workflow lists, creates, edits, runs, switches on and deletes the workflows;
  Connections manages the connections; Storage the storages; Browser Remote Server switches on
  the San Remote server inside the tower (see below); Settings holds the
  tower's settings
  (which broker: inside the tower with its data directory, or a separate
  server's URI).

What else the tower watches is still to be decided.

Module `github.com/wargasipil/san_tower`, Go 1.26. It uses san_message,
san_remote's generated clients, and san_knowledge's `graph` package (the
knowledge tool's store, sync and AI step), from this repository (`replace`
in go.mod).

```powershell
pwsh build.ps1                          # UI build + vet + test + bin/san_tower.exe, bin/san_tower, bin/san_tower-linux-arm64
bin\san_tower.exe serve                 # then open http://127.0.0.1:7477
bin\san_tower.exe serve --listen 0.0.0.0:7477 --lan   # on the network too (no login: see Network)
go run .\cmd\san_tower dev              # tower + the UI's Vite dev server; open Vite's address (http://localhost:5173)
go run .\cmd\san_tower dev --watch      # the same, and a Go change rebuilds and restarts the tower once no run is in progress
bin\san_tower.exe status                # asks the running tower (--server, $env:SAN_TOWER_SERVER)
bin\san_tower.exe update                # install the newest release and restart the tower into it (see Updating)
bin\san_tower.exe restart               # restart the running tower, once no run is in progress
# after editing a proto: Go and TS for TowerService, then TS for san_message's MessageService
buf generate; buf generate --template buf.gen.san_message.yaml
```

The broker's CLI works against the tower too:
`go run ../../packages/san_message/cmd/san_message --server http://127.0.0.1:7477 topic list`.

## Release

Pushing a version tag releases the tower and the San Remote extension
(the user, 2026-10-25; `.github/workflows/release.yml`):

```powershell
git tag v1.0.0; git push origin v1.0.0
```

A Windows job runs `build.ps1` with `SAN_TOWER_VERSION` set to the tag (the
binaries' `--version`; `build.sh` takes it too, and a local build without it
is stamped with the date), so the UI is built and the tower vetted and
tested, then builds the extension. A Linux job packages them and makes the
GitHub release of the tag, with:
- `san_tower-<tag>-windows-amd64.zip`: `san_tower.exe`, `start.bat`, the
  extension and `README.txt`, the setup for whoever gets it
  (`release/` holds both files);
- `san_tower-<tag>-linux-amd64.tar.gz`: the same with the Linux binary;
- `san_tower-<tag>-linux-arm64.tar.gz`: the same for 64-bit ARM Linux, the
  Raspberry Pi (2026-10-26);
- `san_remote_extension-<tag>.zip`: the extension alone.

The extension's manifest gets the tag's version (v1.2.3-rc1 is 1.2.3, its
`version_name` the whole tag). Since 2026-10-25 the workflow's jobs do not
start (the GitHub account is locked over billing), so a release is built
the same way on this computer (`SAN_TOWER_VERSION=<tag>` build.ps1, the
extension's `npm run build`), packaged as the workflow does, and published
with `gh release create`.

## Updating

`san_tower update` (the user's ask, 2026-10-26: a command run by hand, not
a background updater; `internal/update`, `cmd/san_tower/update.go`) asks
GitHub for the latest release of Vaziria/san_knowledge (public: no token),
and when it is newer than this binary's version downloads its bundle for
this system and processor (`san_tower-<tag>-<os>-<arch>`, .zip on Windows,
else .tar.gz). It unpacks it beside the running binary, checks that the new
san_tower runs here and says it is that tag (`--version`), then puts the
files in place: README.txt, start.bat and the extension's folder (whole,
through a `.old`), then the binary last (Linux: renamed over it; Windows,
where a running file can be renamed but not overwritten: the old one to
`san_tower.exe.old`, removed when the tower next starts). A local build
(stamped with the date, not a version) is not updated without `--force`,
which also installs the same version again. `--check` only says whether a
newer release is out.

Then, when the tower at `--server` runs from the file just replaced
(GetStatus's `executable`), the update asks it to restart (TowerService's
Restart) and waits until it answers as the new version. A tower with runs
in progress refuses (`san_tower restart` once they end, or `--force`, which
cuts them short; they are marked interrupted). Restart answers, closes the
tower as a stop does, then starts it again from its executable: on Linux in
the same process (`syscall.Exec`), so a supervisor (the Pi's `run.sh` loop,
systemd) sees no exit; on Windows as a new process in the same console
(start.bat's window stays: it pauses after the first one ends). The new one
retries its address for 10 s (`SAN_TOWER_RESTARTED`), while the old one
lets go of it. `dev` cannot restart itself; `dev --watch` restarts the
tower when its source changes, once no run is in progress.

## Network

The tower has no login, so `serve` listens on loopback only. `--lan` (the
user's choice, 2026-10-26, for the Raspberry Pi opened from the PC) lets
`--listen` take any address (`0.0.0.0:7477`), and the log warns that anyone
on the network can use it. Behind it, `GuardLAN` still keeps out other
sites and DNS rebinding: Host must be loopback, an address of a private
network (10/8, 172.16/12, 192.168/16, fc00::/7, link-local, 100.64/10 as
VPNs give out) or this machine's name (`raspi`, `raspi.local`, `raspi.lan`),
and a browser call must come from the tower's own page (its Origin's host
is the Host) or a loopback one. The Browser Remote Server's addresses stay
on loopback unless its own Listen on setting says All networks (see Browser
Remote Server below). Listening on every address, the tower reaches itself (its
triggers' broker streams) at 127.0.0.1 (`selfURL`), never at `[::]`.

## Raspberry Pi

The user's Pi (`ssh raspi`, 192.168.1.5, 64-bit Debian, no sudo) runs the
linux-arm64 release in `~/san_tower`, open on the LAN at
http://192.168.1.5:7477 (2026-10-26). `deploy/raspi/run.sh` keeps it
running as san_radio's run.sh does: crontab `@reboot`, one copy (`flock`),
started again 10 s after it stops, `serve --listen 0.0.0.0:7477 --lan --data
data`, the log in `san_tower.log` (under 5 MB), and san_radio's unpacked
ffmpeg on the PATH for the video steps. Installed with:

```sh
curl -fsSL -o /tmp/st.tgz https://github.com/Vaziria/san_knowledge/releases/download/<tag>/san_tower-<tag>-linux-arm64.tar.gz
tar -xzf /tmp/st.tgz -C ~                         # ~/san_tower/...
cp run.sh ~/san_tower/; (crontab -l; echo "@reboot /home/pi/san_tower/run.sh") | crontab -
setsid -f ~/san_tower/run.sh                     # start it now
```

Then `~/san_tower/san_tower update` there brings it to each new release.
Steps that drive a browser need the San Remote extension in a browser on
the Pi, which has none: there it runs the other steps (Telegram, schedules,
HTTP, files, video).

## Workflows

A workflow is a graph of steps joined by links between typed sockets:
`event` links say which step runs next, `object` (a google.protobuf.Value)
and `text` (a StringValue) links carry data. The engine (`internal/engine`)
defines the kinds of step, each with a one-sentence description for the
editor's Add menu (a test checks every kind has one) and, for a step of one
site or service, a group, its sub-menu in its category (Social › Threads,
Messengers › Telegram); the UI asks for them
(ListStepKinds). The categories, in the Add menu's order, are Trigger,
Logic, Data, Storage (the user, 2026-10-27: Storage, File Path, Read File
and Glob, moved out of Data), Datetime, Action, Messengers (the user,
2026-10-27: Telegram's and WhatsApp's steps, their triggers too, each
messenger a sub-menu), Social, Finance (the user, 2026-10-27: Binance),
Knowledge (the user, 2026-10-28) and AI; the UI gives each a colour and an
icon (`catalog.ts`, `AddPalette.tsx`). A kind says whether it is a trigger
(StepKind's `trigger`), which the UI reads rather than the category, since
a messenger's trigger is in Messengers. All of them run: Manual, Topic Event, Schedule, Log Event, Tower Restart, Tower Chat and
Telegram Message (triggers), Filter, Prefix Filter, Regex Filter, Pick Greater, Template, Format
Text, Storage, File Path, Read File, Glob, UUID, MD5 Hash, Stringify, Now, Datetime,
Datetime Operation, String to Datetime, Wait, HTTP Request, Bash, Publish to Topic, Download File, Cut Video, Crop Video, Delete File, Delete Folder and Log,
and
the Tower Chat Send, San Remote, Telegram and WhatsApp steps (Social and Messengers, below).

- Manual (the user, 2026-10-27: "What if I need have workflow, but I just
  run it manually") is a trigger that only the editor's Run button (or
  RunWorkflow) starts: nothing else fires it, and the Active switch does
  not change that, so a workflow meant to be run by hand cannot start on
  its own. Payload is the data typed in Run (JSON, text, or nothing); the
  Run dialog picks a Manual step first, with no data.
- A run starts at a trigger step, which fires its event outputs (Topic
  Event: Event, and Payload is the event's data, JSON as it is, other text
  as a string). A step runs when an event link from a fired output reaches
  it, then fires its own. A data input takes the linked output's value: from
  a step that already ran, from a pure step (no event input, such as
  Template) run then, or the value typed in the step for an unlinked field
  input. The first step that fails ends the run; a run stops after 1000
  steps, against a loop.
- Log Event (Trigger; `logevent.go`; the user's ask, 2026-10-25) starts a
  run for each line another workflow's run logs: a Log step's, with its
  Type, or any other step's (Download File's, Cut Video's...), as info. A
  run that fails is a failed line too, with its error, so "tell me when
  anything fails" needs no Log steps. Type is Any (the default), info,
  success, warning, failed or debug; Workflow is empty for every workflow,
  or one's name (case aside) or id. The lines are those the editor's Logs
  show. Message is the line, Type and Workflow its, and Log all of it
  (`message`, `type`, `workflow`, `workflow_id`, `run_id`, `step_id`,
  `step_kind`, `at`); a run by hand with text is an info line of it. A
  run's trigger reads "log, Backup run 12". Lines go out as each step is
  recorded (`Env.Logged`, from `execution.record`), so a long run's early
  lines start their runs before it ends. Against loops: a workflow never
  hears its own lines, and a run that Log Event started hands on none, so
  lines never start runs in a chain (two workflows listening to each other
  would otherwise run forever). It listens while its workflow is on and
  needs no broker; an unknown Type shows as its trigger's error.
- Tower Restart (Trigger; `towerrestart.go`; the user, 2026-10-26: a chat
  before and after a restart, to be aware) fires Before once the tower has
  agreed to restart (TowerService's Restart: `san_tower restart`, `san_tower
  update`, `dev --watch`), with no other run in progress; the tower waits
  for these runs to end, up to a minute, then stops (`engine.BeforeRestart`,
  from the server's Restart). It leaves `restarting.json` (its version and
  the time) in the data folder; the tower started next finds it on Start,
  removes it, and fires After. Version is the tower's (the old one for
  Before, the new one for After), Previous the one it restarted from, Down
  For how long it was away ("4s"). A stop that is no restart (Ctrl+C, a
  crash) fires neither. Its runs read "tower, before restart" and "tower,
  after restart"; by hand, text "before" fires Before, else After. It has
  no trigger state: it needs nothing to listen to. The workflow "Tower
  Restart Notice" uses it to send the user a Telegram chat (Developer Bot)
  before and after.
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
  With Failed not linked, the run fails as before. Fail Run (the user,
  2026-10-27) at the end of a Failed branch that cleans up ends the run as
  failed after all: with its Message, or when that is empty with the error
  of the last step whose Failed fired ("Fail Run: X Post Download: …").
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
- Knowledge Base, Knowledge Search and Knowledge Explain (Knowledge;
  `engine/knowledge.go`; the user, 2026-10-28) look in a knowledge of the
  Knowledge page (Knowledge below) with the knowledge tool's own search and
  explain. All three are pure. Knowledge Base's Knowledge setting picks one
  (its value the knowledge's id, carried by name in an export, as a
  storage is); its output is a socket of a type of its own, `knowledge`
  (the id), which only the other two take. With Sync Yes it reads the
  knowledge's files again first, so a file added since is found. Knowledge
  Search gives the nodes that hold every word of Query, best first, up to
  Limit (10); Knowledge Explain the nodes that match any word of Question
  (English and Indonesian common words aside), up to Limit (5), each with
  its domains and the nodes Depth (1, at most 3) links away. Both have AI
  Format (the user's ask): Yes makes Text a context block for an AI's
  prompt (headed with the knowledge's name, each node with its kind, key,
  file:line to cite, domains, keywords, and its summary, or the start of
  its text while it has none; Explain's is the knowledge tool's own
  explain block), No a list for a person (title, file:line, summary;
  Explain's with domains and "See also"). Explain's starts at Yes, Search's
  at No. Results (Search) and Matches (Explain) are the same nodes as an
  object list, and Count how many matched. They read the graph as last
  synced: the knowledge's page, its Sync, or Knowledge Base's Sync.
- Storage (Storage), UUID and MD5 Hash (Data; the user's ask, 2026-10-25) are pure
  steps: they run when a step takes one of their outputs, once per run.
  Storage gives the storage its Storage setting picks (Storage below): Path
  as set on the Storage page, and Absolute Path, where it is (e.g. for
  Vidara Download's Save To, which san_remote reads from its own folder). A
  storage that is gone fails the step. UUID makes a random version 4 UUID
  (crypto/rand), so every step that takes it in a run gets the same one and
  the next run a new one. MD5 Hash gives the MD5 of its Text field input,
  as 32 lowercase hexadecimal digits.
- File Path (Storage, pure) is a file's path from its File Name (a field
  input, e.g. a UUID through Format Text: `{{ text }}.mp4`): Path and
  Absolute Path, with / as storages have. With a Storage picked, the name
  is inside it (`tmp/videos` + `test.mp4`: `tmp/videos/test.mp4` and
  `<data>/tmp/videos/test.mp4`), and a name that leaves it (`../`, an
  absolute path) fails the step. With none ("No storage: the data
  folder"), a relative name is in the tower's data folder and an absolute
  one is itself. It only makes the path: it creates and checks nothing.
- Read File (Storage, pure; `filepath.go`; the user's ask, 2026-10-25) gives
  the text of the file at Path as Text. Path is placed as File Path places
  a name (Storage setting, else the data folder, or absolute). UTF-8 text
  up to 10 MB: a byte order mark is dropped and Windows line ends become
  `\n`. A file that is not there, a folder, a bigger file or a binary one
  fails the step. Linked from Glob's Item, it reads each file in turn.
- Glob (Storage; `glob.go`; the user's ask, 2026-10-25) finds the files
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
- Cut Video (Action; `video.go`; the user's ask, 2026-10-25) cuts the part
  of the video File from Start to End, in seconds (`12.5`, or `12,5`),
  with this computer's ffmpeg (on PATH; `winget install Gyan.FFmpeg`). An
  empty Start is 0, an empty End the video's end. File is taken as it is
  when absolute (a Download File's File links in), else placed as File
  Path places a name. The part is encoded again (`ffmpeg -ss Start -i File
  -t End-Start`), so it starts and ends at those seconds, not at the
  nearest keyframes, with the codecs ffmpeg picks for the extension (H.264
  and AAC for .mp4). Save To is a file or a folder, placed the same way;
  empty, the cut goes beside File as `clip_10s-25s.mp4` (`clip_10s-end.mp4`
  with no End). It is written to a `.part` file beside its place first and
  moved there once whole. A Start past the video's end fails the step; an
  End past it cuts to the end, which the log says. File is the cut
  video, e.g. for Telegram Send's File.
- Crop Video (Action; `video.go`; the user's ask, 2026-10-25) crops the
  picture of the video File with ffmpeg's crop filter, the whole length.
  Aspect 9:16 (the default), 1:1, 4:5 or 16:9 keeps the largest middle
  part of that shape (`crop=w='trunc(min(iw,ih*9/16)/2)*2':h=...`: a
  1920x1080 video at 9:16 keeps 606x1080 from its middle); Custom shows
  Width, Height, X and Y and keeps Width x Height pixels at X, Y from the
  top left, centred on an axis whose X or Y is empty. Sizes are made even,
  as H.264 needs (a Width of 33 is 32); a crop larger than the picture
  fails with ffmpeg's message. File, Save To, the codecs and the `.part`
  file are as Cut Video's (shared in `video.go`: `videoIn`, `videoOut`,
  `ffmpegTo`); beside File it is `clip_9x16.mp4` or `clip_crop_32x20.mp4`.
  The log says the sizes ("from 1920x1080 to 606x1080").
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
- Wait (Datetime; the user, 2026-10-27) holds the run For a while, typed
  or linked (`30s`, `1m`, `2h`, `1d`, `1d12h`), then goes on by Done. A run
  stopped while it waits (the tower stopping) ends there. While it waits,
  the run is in progress, so `dev --watch` and Restart wait for it too.
- Claude Ask (CLI) (AI › Claude; the user's ask, 2026-10-25) asks Claude
  with this computer's Claude Code: `claude -p --output-format json --tools
  "" --strict-mcp-config --no-session-persistence` with the Prompt on stdin,
  in the temporary folder, as the knowledge tool's AI step runs it, or in
  Directory, an absolute folder, when it is set (the user's ask: a base
  project directory, the working directory of the call), so that project's
  CLAUDE.md and settings come in. Directory is a field input (2026-10-26),
  typed in the step or linked from any text, as Prompt is. It is the
  user's own Claude Code login.
  - Model (2026-10-26, the user's ask: a model and its version, as Claude
    Code's /model lists them) is Default, Fable 5.1, Opus 5.5, Opus 5.5
    (1M), Sonnet 5.5, Sonnet 5.5 (1M) or Haiku 5.5, sent as their full
    names (`claude-fable-5-1`, `claude-opus-5-5`, `claude-opus-5-5[1m]`,
    …). Opus, Sonnet and Haiku, the choices before versions, still run, as
    claude's aliases for the latest of each, and the editor still shows
    them in a step set to one (a select shows a value it no longer offers).
    A model the account cannot use fails the step with what Claude Code
    says about it. Effort is Default, Low, Medium, High, XHigh or Max
    (`--effort`). Default leaves either to Claude Code.
  - Tools (2026-10-26, the user's ask) is None (the default: `--tools ""
    --strict-mcp-config`, no tools and no MCP servers, for a prompt that
    may come from a message anyone sent), Read Only (`--tools
    Read,Grep,Glob`, still no MCP: Claude reads and searches the files of
    Directory), or Full (no tool flags: all of Claude Code's tools, the
    MCP servers of the project and the user, and the skills). claude -p
    cannot ask for permission, so a tool runs only when Permission Mode or
    the user's Claude Code settings (the `allow` rules of
    `~/.claude/settings.json` and the project's `.claude/settings.json`)
    allow it; anything else Claude is refused, and says so.
  - Permission Mode (2026-10-26, the user's ask) is `--permission-mode`:
    Default (none sent: the settings' `defaultMode`, else Manual), Manual
    (only what the settings allow), Accept Edits (file edits in Directory
    too), Auto (Claude Code's safety check allows what is safe and refuses
    what is risky, as in an auto-mode session), Plan (looks and plans,
    changes nothing), Don't Ask (only what the settings allow, never
    asking), or Bypass Permissions (everything, nothing checked). Bypass
    Permissions is only for prompts the user writes: a Telegram message or
    a failure line could then run anything on this computer. Auto is the
    one for a workflow that should act on its own.
  - Wait bounds the answer (default 10m).
  Answer is its reply; an error Claude Code reports (a usage limit, not
  signed in) fails the step with it. With a Session ID (a field input,
  2026-10-26: the user's ask, so that several steps attach the same
  session) it keeps the conversation: no `--no-session-persistence`, but
  `--resume <id>`, and when Claude Code has no such conversation yet
  ("No conversation found"), `--session-id <id>`, which starts it. Session
  ID must be a UUID, as claude takes.
  - A session's questions go to one live process while it answers
    (`claudelive.go`; the user, 2026-10-26: the Telegram chat as
    interactive as Claude Code itself, built by hand to learn how): claude
    -p in stream-json both ways (`--input-format stream-json
    --output-format stream-json --verbose --replay-user-messages`), each
    question a JSON line on stdin with a uuid of its own. Experiments with
    claude 2.1.306 showed what it does, which the fake claude in
    `claudelive_test.go` copies (and `TestClaudeLiveReal`, with
    `SAN_TOWER_CLAUDE_LIVE=1`, checks against the real one): a question sent
    while Claude works is queued by Claude Code and answered next, in the
    same process; questions queued when a turn begins are read as one turn
    and answered once; each question is echoed back with its uuid as it is
    taken into a turn, so the turn's answer (its `result`) goes to the run
    of the last question taken, and the runs of the others end firing
    nothing, logging that they were read with it; `--json-schema` still
    shapes each answer; a `--resume` of a conversation Claude Code does not
    have yet is a `result` with "No conversation found" before any `init`,
    and the process is started again with `--session-id` and the questions
    sent again. Once no question waits, stdin is closed and the process
    ends: nothing runs, or takes memory, between conversations, and the
    next question starts it again (one arriving while it ends waits for
    that). The process's settings (Model, Tools...) are its first
    question's. The tool calls and what Claude says become the step's log
    lines.
  - Claude Interrupt (AI › Claude) stops the turn going on in a Session ID
    as Esc does: a `control_request` of subtype interrupt. Claude Code
    notes "[Request interrupted by user]", the turn ends without an answer
    (`error_during_execution`), the Claude Ask (CLI) waiting on it fires
    Stopped, and the conversation and the questions queued behind it go
    on. It fires Stopped, or Idle when nothing was going on. A question
    whose Wait runs out is interrupted the same way; a tower that stops
    kills the process.
  - With Status Bot (a Telegram Bot connection) and Status Chat, the step
    sends a Telegram message that shows what Claude does, as Claude Code
    does on screen: "Queued" while another question is answered, then
    "Working · 1m 20s · 5 steps" with the last six steps ("Read
    claude.go", the Bash step's description, "“Let me look…”"), edited at
    most every 3 s (Telegram allows about one message a second in a chat),
    and at the end "Done", "Stopped", "Failed" or "Read together with your
    next message". It is plain text, edited with editMessageText.
  - The Dev Workflow uses it: a Prefix Filter sends "/stop" to Claude
    Interrupt (replying "Stopped." or "Nothing is running."), everything
    else to Claude Ask (CLI) with the live status in the same chat. Claude says
  whether it answered or needs feedback (2026-10-26, the user's ask: tell
  the two apart): the step asks for its reply in a shape (`--json-schema`,
  `{needs_feedback, text}`), and when Claude cannot answer without more
  information or a decision from the person, Needs Feedback fires instead
  of Done, with Question its question. When Needs Feedback links nowhere,
  a question goes out by Done as Answer (and Question), so a workflow that
  only reads Answer still gets it; a reply not in the shape is an answer.
  The step does not wait for the reply: it comes in a later run, to a
  Claude Ask (CLI) with the same Session ID (a Telegram Message trigger
  in the same workflow, say), and Claude goes on from its question.
- Claude Chat Session (AI › Claude, pure, 2026-10-26) gives a Session ID
  for Claude Ask (CLI) steps: linked to several, they ask in one
  conversation, which goes on from run to run. It is a UUID (version 5)
  made from the workflow's id and the step's, so every run gives the same
  one, and another Claude Chat Session step, or this one in an imported
  copy of the workflow, gives another. Its conversation is a Claude Code
  session like any other: `claude --resume <id>` opens it, and it is in
  `/resume` of the folder its first question ran in.
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
- Bash (the user, 2026-10-27: "Add node bash for executing shell and it can
  connect to storage as working directory"; `engine/bash.go`) runs Script
  with bash in the folder of the storage its Storage setting picks, or in
  the tower's data folder when none is, with Input on its standard input.
  Stdout and Stderr (each its first 1 MB, the last newline trimmed) and
  Exit Code are its outputs, and its log line says the exit code, how long
  it took and where it ran. An exit code other than 0 fails the step
  (Failed, with Error the code and stderr's last lines) unless Non-zero
  Exit is Goes On. After Timeout (5m, up to 24h), or when the tower stops,
  it is stopped with everything it started: a process group on Linux, a
  job object of its own on Windows, where it starts suspended, joins the
  job and then runs (msys's fork loses the parent links `taskkill /T`
  follows: a `sleep 30 &` outlived the script, 2026-10-27). What a script
  leaves running when it ends by itself goes on, as in a terminal. The
  script goes through a temporary file, not `-c`. On Windows it is Git
  Bash's bash (beside git, or where Git for Windows installs it), never the
  `bash.exe` of System32 or WindowsApps, which starts WSL and which a
  cmd.exe's PATH finds first; `SAN_TOWER_BASH` names another.
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
  YouTube, Instagram, ChatGPT, Gemini and claude.ai through
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
  - X Post with Plain (the user's ask, 2026-10-25) takes Caption (instead of
    Text, or a File), Image and Video, shown while Format is Plain (a
    Socket's when can join conditions with commas, "Format=Plain,Source=Text"):
    Image is up to 4 files, one per line, Video one video or GIF, since X
    takes one or the other in a post; they go on the first post (in San
    Remote's API, `Post.media`, absolute paths). A relative path is in the
    tower's data folder; a file not there fails the step before anything is
    sent, as do 5 images or both. The Caption may be empty when there is
    media. A step saved before Caption, whose Text is linked (Underground
    Flow's Gemini Answer), still posts its Text: Caption wins when it has
    any. With Markdown, Text is shown as before and Image and Video are not
    used: its `![](file)` images go on their posts.
  - Vidara Upload uploads File (a path on this computer) with its Title,
    into Folder (the user's ask, 2026-10-25): one of the account's folders
    by name, or `Promo/2026` for one in a folder, picked in the Upload
    page's Folder list; a top-level folder that is not there is made
    (-Create new-), one in a folder must be there; empty is the root.
    Link is the uploaded video's link (the user's ask, 2026-10-25), as
    My videos shows it, or https://vidara.to/v/<code> when its row has
    none; Video is all My videos lists of it.
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
  - Instagram Post (Social › Instagram; the user, 2026-10-29: an Instagram
    node, photos, carousels and Reels, "both, chosen by At") posts through
    San Remote's InstagramService. With Format Plain,
    Media is the files, one per line (up to 20, photos and videos mixed; a
    relative path is in the tower's data folder, and a file not there fails
    the step), with Caption or a File. With Markdown, Text or a File is a
    post file, whose `![alt](file)` images are the media with their alt
    text, so X Post Download's Markdown is a post as it is. One video is a
    Reel, anything else a feed post. With At (on the minute, 15 minutes to
    29 days ahead) it is scheduled in Meta Business Suite, where
    @archetipe__, a creator account, is signed in: up to 10 files, no AI
    label, and alt text is not set. **An empty At shares it at once** on
    instagram.com, which has no schedule: Instagram publishes the post as
    soon as Share is pressed. This is the one post step that does not only
    schedule. AI Label Yes turns on Add AI label there. Share to Threads and
    Facebook are always off. Post is the response (`scheduled` and `at` for
    a scheduled one), and Link the post's (https://www.instagram.com/p/<code>/)
    when shared now, found on the profile after Share (empty when the
    profile does not show it yet, and for a scheduled post). Social
    Status reads Instagram, and Social Posts lists its Published posts (the
    latest 10, pinned ones included; Last is the newest by time) or its
    Scheduled ones (Business Suite's Dijadwalkan), which Last Scheduled Post
    reads too; Post Count leaves it out.
  - Instagram Delete (Social › Instagram; the user, 2026-10-29: "dont
    forget instagram delete node") deletes a post. List Published: Post is
    its link or code (an Instagram Post's Link), deleted from its page on
    instagram.com, which keeps it in Recently deleted for 30 days. List
    Scheduled: the one post scheduled in Business Suite whose caption starts
    with Caption and, when given, goes out at At, such as a Last Scheduled
    Post's Text and Datetime; it cannot be undone. Several or no posts
    matching fail the step. Post is the post as it was listed.
  - YouTube Upload (Social › YouTube; the user, 2026-10-26: YouTube in San
    Remote, Shorts too) uploads Video (an absolute path) through YouTube
    Studio (San Remote's YouTubeService) with Title, Description and Tags
    (commas between them), not made for kids, and schedules it at At
    (rounded up to a quarter hour, Studio's only times; at least 15 minutes
    ahead), or without At saves it as Private:
    never public at once. Kind Short is for a Short, which YouTube makes of
    a vertical or square video of at most 3 minutes by itself: the step
    fails when Studio lists it under Videos (it is saved all the same).
    Video is the response (the video's id, link, visibility, date, Studio
    link, channel), Link its link (youtube.com/shorts/<id> for a Short).
    Social Status reads who is signed in on YouTube; Social Posts lists
    Studio's Videos and Shorts: Scheduled, Published (Public), or Drafts
    (Private and Unlisted), each at its day (Studio's date column has no
    time), so Last Scheduled Post gives the last day one is scheduled.
  - YouTube Make Private (Social › YouTube) makes Video (its id or a link,
    such as a YouTube Upload's Link) Private in Studio's content list; a
    scheduled one loses its schedule. YouTube Delete deletes one for good
    (Delete forever), but never a Public video (the user, 2026-10-26: a
    workflow has nobody to ask; make it Private first when that is meant):
    it sends `allow_public` false. It cannot be undone.
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
  - Chat Link, on those three (the user, 2026-10-27: sessions as Claude Ask
    (CLI) has; `browserchat.go`): an input that goes on in a kept chat, a
    chat's link (https://chatgpt.com/c/..., https://gemini.google.com/app/...,
    https://claude.ai/chat/...) or a Browser Chat Session's; and an output,
    the chat's link (empty for a Temporary chat). A chat that goes on is
    always a kept one: Temporary does not apply, and the step's log says so.
  - Browser Chat Session (AI) is a chat that goes on from run to run: link
    its Chat Link to the asks' Chat Link. The first ask on a site starts a
    kept chat and the session keeps its link (the tower's database,
    `chat_sessions`, by session and site; a deleted workflow's go with it),
    so each later ask, in this run or another, goes on in it. One session
    holds a chat per site, so the same session can go to ChatGPT, Gemini
    and Claude. San Remote opens the chat by its link, as a person opens one
    they kept. A session's chat deleted on the site (the user, 2026-10-28: "can it refresh when I delete session"; san_remote says NotFound, "the chat is not there") is started again: the ask goes to a new kept chat, the session keeps it, and the step logs a warning; Remote Shell does so for the user's message at the start of a run, the new chat getting the protocol. A chat's link typed in, not a session's, still fails.
  - Remote Shell (AI; the user, 2026-10-27: "Implement
    remote_shell_protocol.md. it to be node"; `engine/remoteshell.go`) is
    the Web Machine Shell Protocol of
    [remote_shell_protocol.md](../san_remote/docs/remote_shell_protocol.md):
    a web chat on Site (ChatGPT, the default, Gemini or Claude) talks with
    the user and the machine, a bash shell on this computer, in one chat.
    Each message to the chat is a WebchatRequest and each answer a
    WebchatResponse (`protos/san/tower/v1/remote_shell.proto`), as JSON with
    the proto's field names. Message, the user's, goes first, from
    `FROM_TYPE_USER`; a new chat gets the protocol in that same message (who
    talks, the two JSON shapes, the shell and its folder). Each answer's
    `chat` fires Chat, with Reply its text, so a Telegram Send on Chat
    tells the user; its `execute` runs as Bash runs a script (in Storage's
    folder, else the data folder; Timeout, 5m, each), and its exit code,
    stdout and stderr (each 8000 characters at most, the middle left out)
    go back to the chat from `FROM_TYPE_EXECUTE`. An answer with nothing to
    execute ends the step (Done): the chat waits for the user's next
    Message, in a later run. San Remote reads the answer with the site's
    Copy button, which gives markdown, and JSON written as text does not
    come back as it was: Gemini's copy adds markdown's escapes
    (`\*\*bold\*\*`, `https\://`; 2026-10-27), and ChatGPT writes its
    backslashes doubled for markdown (`\\"$f\\"` for `\"$f\"`, which ends
    the string early; 2026-10-28). So the protocol, its reminder and the
    ask again ask for the JSON in a ```` ```json ```` code block, escaped as
    JSON only, which neither touches. The answer's first code block is read, its fences as markdown reads them (a longer fence, four backticks, around JSON whose chat holds a code block; a ``` in the one-line JSON's strings ends nothing; 2026-10-28),
    when it has one, else, or when it holds no WebchatResponse, the whole answer, from its first `{` to its last `}`. One
    that is not JSON as it is is read again with markdown's added escapes
    dropped (and a raw line break in a string escaped), then as markdown
    shows it (a backslash before punctuation dropped once). One that is
    still not a
    WebchatResponse is asked for again once, then fails the step. The chat
    is always a kept one, in one tab (the user, 2026-10-27: a tab opened
    and closed for each turn flickered): San Remote keeps the tab open on
    the chat (`idle_seconds`, its `page.Kept`), each turn and each later run
    goes on in it, and it is closed once 5 minutes go by without an ask; a
    tab the user closed is opened again by the chat's link. Chat Link goes
    on in a chat, a chat's link or a
    Browser Chat Session's, which keeps the conversation from run to run,
    and the output is the chat's link. The protocol goes only in a new
    chat's first message; in a chat that goes on, the user's Message comes
    after a one-line reminder of it (the user's choice, 2026-10-27: a chat
    picked up hours later may have drifted), while a script's result, seconds
    after the last answer, comes without one. Commands counts the scripts
    run. There is no limit on them unless Max Commands sets one (the user,
    2026-10-27: long jobs, such as making a video or setting up a project,
    take many scripts); a script asked for past it fails the step. The step
    fails too when the chat looks stuck: the same script with the same
    result 3 times in a row (a script polling something whose result
    changes goes on). A script that only prints text it holds (echo or printf of plain words, no variable, substitution, redirection or pipe) is not run and ends the chat's turn (Done): a chat with nothing left to do ran echo OK, echo Selesai, echo Siap... one after another without end (2026-10-28); the protocol and its reminder say execute is only for the machine's shell, for work the user asked for, never for conversation, which goes in chat; a command the user asked how to run themselves is explained in chat, not run (2026-10-28: "how do I preview HyperFrames" ran `npx hyperframes preview`), and a command that does not end by itself (a preview or dev server, a watcher) is started in the background, never in the foreground. The step logs, as it goes (see the Logs below), the chat's answers, each script, what it prints and how it ended; each script runs in a new bash in the storage's folder (a cd does not carry over), which the protocol tells the chat, with the Timeout. The web chat runs any script it wants with the tower's rights
    (the protocol asks it to ask the user first before what cannot be
    undone, which it may not do): give it a storage to work in.
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
  - Post Count (the user, 2026-10-27: "add node for getting post count at
    input date in X"; `postcount.go`) counts a Site's (X or Threads, whose
    lists tell each post's time) posts on Date: a day in local time
    (2026-10-28, a datetime counts its day, empty is today). List is
    Both, Scheduled or Published. Count is the total, Scheduled and
    Published each list's count (empty when not counted), Posts the posts
    counted with their `list`, Date the day. Published reads the latest 100
    posts from the profile; when all 100 are on or after the day, the log
    says older ones were not counted.
  - A post brings Chrome to the front for a minute or two. A Vidara
    upload opens its tab but does not wait for Chrome to be in front: it
    goes on behind other windows, as long as the file takes.
- Telegram steps work as a bot, through the Telegram Bot connection their
  Connection setting names (the Bot API, over plain net/http,
  `internal/telegram`):
  - Telegram Message (Messengers › Telegram, a trigger) starts a run for each message the bot
    gets, in any chat or in the one its Chat names (a chat's id or
    @username). Message is the message as Telegram sent it (`payload.text`,
    `payload.chat.id`, `payload.from.username`), Text its text (or a
    photo's caption), Chat its chat's id, which a Telegram Send can take to
    reply. A run's trigger reads "telegram, chat @heri, message 1010".
  - Telegram Send (Messengers › Telegram) sends Text to Chat (a chat's id, a
    group's such as -1001234567890, or a channel's @username), Plain, HTML
    or Markdown, as a reply when Reply To has a message id. Message is the
    message sent.
  - Long Text (2026-10-28: a web chat's long answer made Telegram refuse a
    message, "message is too long"; the user asked for it as a choice):
    Split, the default, sends a Text over a message's 4096 characters as
    several messages of at most 4000, counted as Telegram counts (UTF-16,
    an emoji two), each ending between lines (a longer line between words,
    a longer word anywhere), each formatted on its own, only the first a
    reply; the step logs how many, and Message is the last. Fail sends it
    whole, and Telegram refuses it. A Media caption is never split.
  - Markdown (2026-10-26, after Telegram refused a Claude answer with
    "can't parse entities") is Markdown as people and Claude write it
    (`**bold**`, `` `code` `` in bold, lists, fenced code, links), turned
    into Telegram's HTML by `telegram.MarkdownHTML`: bold, italic, strike,
    code, code blocks, quotes and http(s) links as tags, headings bold,
    lists with • or their numbers, the rest as escaped text. Telegram's own
    "Markdown" parse mode reads only its old dialect (`*bold*`, nothing
    nested) and refuses the whole message otherwise. When Telegram still
    cannot read a message's formatting (HTML typed by hand, too), the step
    sends the text as it is, without parse_mode, and logs a warning.
  - Its Media (the user's ask, 2026-10-25) sends a Photo, Video, Audio,
    Voice, Document or Animation instead, with Text as its caption
    (optional, up to 1024 characters). The File input, shown once Media is
    not None, is one of: an http(s) link, which Telegram downloads (up to
    5 MB for a photo, 20 MB otherwise); a file on this computer, absolute
    or in the tower's data folder, uploaded as multipart/form-data and
    streamed (up to 10 MB for a photo, 50 MB otherwise, checked before
    the upload); or, when no such file is there, the file_id of a file
    Telegram has, such as a received photo's (`payload.photo`).
  - A send is tried up to three times (the user's ask, 2026-10-25, after
    one over IPv6 got no answer: `wsarecv ... did not properly respond`),
    2 s and then 5 s apart (`telegram.DefaultRetry`), when another try may
    get through: no answer (the next tries reach Telegram over IPv4, since
    a path that breaks mid-call is often IPv6's), Telegram failing itself
    (5xx), or 429 Too Many Requests (after its `retry_after`, up to 30 s).
    A mistake (chat not found, bad request) is not tried again, and a
    media file is uploaded afresh each try. Each new try is a warning line
    in the run ("try 2 in 2s, after: ... (now over IPv4)"). A send whose
    answer was lost after Telegram took it is sent twice: the Bot API has
    no way to tell.
  - Telegram Messages (Messengers › Telegram) lists the newest messages the bot
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
- Telegram account steps (the user, 2026-10-27: "In telegram add node
  create bot, delete bot, create group, delete group, add user to group,
  remove user to group", then "We use San remote, not telegram API") work
  as your own Telegram account, in Telegram Web in your browser, through
  the San Remote connection their Connection setting names
  (`san.remote.telegram.v1.TelegramService`, `engine/telegramweb.go`; the
  steps themselves are San Remote's `internal/telegram`). Each brings
  Chrome to the front with a tab of its own for a minute or so: Telegram
  Web does not start in a hidden tab. All in Messengers › Telegram:
  - Telegram Create Bot asks @BotFather for a new bot with Name and
    Username (ending in bot) and adds it as a Telegram Bot connection named
    after the bot (the user's choice), or after the bot and its @username
    when that name is taken, so Telegram Message and Telegram Send can use
    it at once (its poller starts as on the Connections page). Bot is the
    bot without its token, Username and Link (t.me) its own, Connection ID
    the new connection's id. The token goes into the connection only, never
    into an output, which a run keeps and shows. @BotFather's refusal (a
    username taken) fails the step with what it said.
  - Telegram Delete Bot deletes one of your bots through @BotFather
    (/deletebot, with its confirmation). It cannot be undone. The bot's
    connection stays, and no longer works.
  - Telegram Create Group makes a group titled Title with no other members
    yet. Group ID is its chat id (-5012345678), which a basic group shares
    with the Bot API, so a Telegram Send can post to it once its bot is a
    member.
  - Telegram Delete Group deletes one of your groups for all its members
    (its menu's Delete Group, with "Delete for all members"). Group is its
    id, or its title when only one of your groups has it. A group you do
    not own is refused, never left. It cannot be undone.
  - Telegram Add Member and Telegram Remove Member add a user to one of
    your groups, or remove one, by username (User, with or without the @);
    Group as for Delete Group. A user already in the group is left as they
    are; one whose privacy settings keep them out of groups, or who is not
    a member to remove, fails the step.
- Tower Chat steps (Messengers › Tower Chat; `engine/towerchat.go`; the
  user, 2026-10-28: "after menu home, create menu chat. and we have node
  Tower Chat") talk with the Chat page (see Chat below), as the Telegram
  steps talk with a bot (the user's choice: a trigger and a send):
  - Tower Chat (a trigger) starts a run for each message sent, on the Chat
    page, in a chat with its workflow, while the workflow is switched on.
    Message is the message (`id`, `chat`, `chat_name`, `from`, `text`,
    `at`), Text its text, Chat its chat's id, which a Tower Chat Send takes
    to answer. A run's trigger reads "chat Dev, message 12"; one by hand
    with text is a message of that text in no chat.
  - Tower Chat Send sends Text to Chat: a chat's id (a Tower Chat's Chat) or
    its name (case aside, when only one chat has it), so a Schedule
    workflow can post in a chat too. Format is Markdown (the default),
    which the page shows formatted (headings, lists, code, tables, links),
    or Plain. A text over 64K characters is cut, saying so. Message is the
    message sent; a run may send several.
- WhatsApp steps (the user, 2026-10-26: WhatsApp like Telegram, with
  `go.mau.fi/whatsmeow`) work as the WhatsApp account of the WhatsApp
  connection their Connection setting names, which is a device linked to
  the account, as WhatsApp Web is (`internal/whatsapp`, `engine/whatsapp.go`):
  - The connection is made by linking, never typed in (the user: scan the
    QR code, and connected first): the Connections page's New connection,
    type WhatsApp, a Storage (required), Link with QR code, calls
    ConnectionService's PairWhatsApp, a server stream. It sends a QR code (the code, and a 320 px PNG made by
    `skip2/go-qrcode`) every 20 s or so; the user scans one with the phone
    (WhatsApp › Settings › Linked devices › Link a device); once the device
    is linked and then connected (whatsmeow logs in again after the link),
    the connection is created, with the device's JID, the account's number,
    its name and the storage, and the stream ends with it. A storage
    missing or gone, or a name in use, fails before the first code. Closing the dialog ends the stream, which stops the
    linking and keeps nothing. Editing renames it; another account is
    another connection. Deleting it logs the device out first, so the phone
    drops it from Linked devices (a device that cannot connect is only
    forgotten here).
  - Each connection's device keys are in a database file of its own in
    the folder of its storage (the user, 2026-10-26: a base storage from
    those registered, required), never in tower.db or an export: named by
    the SHA-256 of the connection's name as it was made, in hex, with .db
    (`whatsapp.FileName`; the user, 2026-10-26), and saved in the
    connection's settings (`database`), so a rename keeps it. Only a
    WhatsApp connection's name can change (UpdateConnection keeps its
    settings), and the store takes only such a file name, never a path out
    of the storage. It is whatsmeow's sqlstore on the same pure-Go SQLite
    (`glebarez/go-sqlite`). `engine.WhatsAppStores` opens each file once,
    when first needed (making the folder), and closes them after the
    engine; a linking that fails removes the file it made, and deleting the
    connection deletes it (with its -wal and -shm). A file that is gone is
    not made again empty: the trigger's state says to link again. The
    storage is looked up at each connect, so one that is gone shows as the
    trigger's state until it is back. The tower keeps each WhatsApp connection's device
    connected all the time it runs, used by a workflow or not, as a Telegram
    bot's poller; whatsmeow connects again by itself when the connection
    drops, and a first connect that fails is tried again after 1 s doubling
    to 30 s. Test says whether it is connected. When the phone logs the
    device out, the state says so: delete the connection and link again.
  - WhatsApp Message (Messengers › WhatsApp, a trigger) starts a run for each message the account
    gets, in any chat or the one Chat names: a phone number with its
    country code (any spaces, +, - or dots aside), which takes the chat
    with that number and that number's messages in groups, or a chat's JID
    (a group's ends in @g.us). From says whose: Others (the default),
    Others and me, or Me: yours are those you send from the phone or
    another device, such as in the chat with yourself; the messages the
    tower sent never start runs (it remembers the last 500 it sent), and a
    message that comes twice runs once. Message is all of it (`id`, `chat`,
    `sender`, `phone`, `name`, `text`, `type`, `from_me`, `is_group`,
    `time`), Text its text or a photo's, video's or document's caption,
    Chat where to reply, Phone and Name the sender's number and own name.
    Reactions and protocol messages start nothing. A run's trigger reads
    "whatsapp, chat +628123456789, message 3EB0...". Messages that came
    while the tower was stopped arrive when it connects again.
  - WhatsApp Send (Messengers › WhatsApp) sends Text to Chat, a phone number
    with its country code or a JID (a WhatsApp Message's Chat, to reply);
    Message ID is the sent message's. With File (the user, 2026-10-27: a
    zip of the tower's build to a friend), a file on this computer,
    absolute or in the tower's data folder, up to 500 MB, goes as a
    document with Text as its caption (`whatsapp.Conn.SendFile`: encrypted
    and uploaded through a temporary file, not held in memory; its MIME
    type by its extension).
  - WhatsApp does not allow unofficial programs and may ban an account that
    uses one: the dialog says so; a number you can risk is safer.
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
- Folder Watch and File Watch (Trigger; `watch.go`; the user, 2026-10-27:
  "add node that can watch file add, edited, deleted in folder and
  subfolder with base storage. And add node file watch too") start a run
  for each file added, changed or deleted: Folder Watch in Folder of its
  Storage (the whole storage when empty) and every folder under it, folders
  made later too (their files count as added); File Watch for File in its
  Storage. A file's events (fsnotify) count once the file has been quiet
  for 2 s, so a copy or a save in parts is one run; a file that came and
  went in that time is none, and one replaced by a save is changed. Folders
  themselves are not told. The run's outputs are Path (in the storage, with
  /), Absolute Path, Change (added, changed or deleted), and Event with
  Added, Changed or Deleted fired; its trigger reads "folder watch, added
  inbox/a.mp4". A watch works only while its workflow is switched on, and
  says so in the editor ("folder Videos/inbox", or why not: no storage, a
  path outside it, a folder that is not there). Changes made while the
  tower is stopped are not told afterwards. A workflow that writes into the
  folder it watches starts itself again.
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
- What a run keeps is bounded (the user, 2026-10-27: "what if log contain
  many log"; `store/runlimit.go`), so neither tower.db nor the pages that
  show runs grow without end, however much a step logs (a loop, a long
  Claude session) or outputs (a whole file or page). The run itself has it
  all; what is stored is cut, with a note saying what was left out: a log
  line to 2000 characters, a step's lines to its last 500, an output to
  64 KB (a text keeps its start, any other value becomes a note), a run's
  steps to 2 MB in all (the steps after keep only their errors and what
  they fired). Each run also stores its summary (`RunSummary`): its last
  20 lines, each cut to 300 characters, and how many came before. The Logs
  pages, which ask every 2 s for up to 200 runs, get the summaries
  (`Run.more_logs` the count left out) and never read the steps; opening a
  run (GetRun) shows them. A run stored before summaries gets its summary
  the first time a list reads it.
- A running run's lines show as they come (the user, 2026-10-27: "When
  remote shell node execute command, can it log streamed to frontend log";
  `engine/live.go`). A run's steps are stored only when it ends, so a
  step that works long logs with `call.log` as well, which passes each line
  on at once (`Env.Live`): the engine keeps a running run's last 20 lines
  in memory, and the Logs pages get them, with the count before them, as
  the run's summary until it ends and its record has them. Remote Shell
  logs this way: the web chat's answers, each script, each line the script
  prints to stdout or stderr (debug; a `\r` ends a line, so a progress bar
  shows as it moves), and how it ended. A step logging without end keeps
  its last 500 lines as it runs, with a first line counting the ones that
  went. Opening a running run shows nothing yet.
- The pages that ask again and again (the Logs, the editor's Runs, the
  Browser Remote Server page) ask nothing while their browser tab is hidden,
  and again as soon as it is shown, and never twice at once
  (`usePoll`, `src/poll.ts`). The Logs keep a run that did not change as
  it was drawn.
- A description (the user, 2026-10-27), up to 2000 characters, says what
  a workflow is for: given when it is made (CreateWorkflow), shown under
  its name in the list and in the editor, edited in its settings (the
  pencil in the editor, Settings in the list's menu), found by the search
  as the name is, and carried by an export. Workflow.description is
  optional: UpdateWorkflow replaces it when it is sent and keeps it when
  not, as the editor's saves of the graph do.
- One run at a time (the user, 2026-10-27: X links sent to Underground
  Flow in a burst ran together, fought over the browser, scheduled their
  posts for the same minute, and failed together), a workflow setting
  beside the name and description (Workflow.one_at_a_time, optional as the
  description is, carried by an export). Its queue is in the message
  broker (the user: "I need undercover flow use our message broker";
  `engine/queue.go`):
  - Each run is published, as its event came, to the workflow's topic
    `tower-queue-<id>`, and waits as Running. The workflow's queue reader,
    the pull subscriber of that topic, takes the runs one at a time and
    runs each to its end before it takes the next. A run that waited a
    second or more says so in its trigger's log.
  - A run's event is acked when the run ends, succeeded or failed, or
    earlier by an Ack step. One not acked stays in the broker: after a
    restart it runs again as a new run, its trigger ending "(again: the
    tower stopped before it was done)". Put Ack after the step that must
    not happen twice (Underground Flow: after X Post); a run that ran to
    its end but whose ack was lost is not run twice.
  - The reader's stream asks San Message to hold each event for up to 6
    hours (Hello.ack_timeout_seconds); the broker's loop does not wait for
    a pull stream's ack, so a run of minutes holds up no other subscriber.
  - A run that cannot join its queue (the broker does not answer) is
    recorded as failed. Switching the setting off leaves a queue that
    still holds runs being read until it is empty; deleting the workflow
    deletes its queue. Runs of other workflows are not held up.
- Folders (the user, 2026-10-27: "In workflow, add folder like. So I can
  manage workflow easily"; `store/folder.go`): a workflow is in one folder
  or at the top, and folders nest (WorkflowService's ListFolders,
  CreateFolder, UpdateFolder, DeleteFolder, MoveWorkflows; ListWorkflows
  lists one folder's workflows, or with all_folders every folder's, as a
  search does). A folder's name is unique in the folder it is in, ignoring
  case; a folder cannot go into itself or a folder inside it. Deleting a
  folder deletes no workflow: what was in it moves up to its parent (a
  folder named like one there gets " (2)"). Workflow names stay unique
  across folders. The Workflow page shows one folder (`?folder=<id>`), its
  folders first with how much is in each, under a breadcrumb back to the
  top; New workflow and New folder make them there; a row's menu has Move
  to..., and a workflow dragged onto a folder, or onto a folder of the
  breadcrumb, moves there. A search looks in every folder and shows each
  workflow's folder. The editor's back link returns to the workflow's
  folder. An export does not carry folders yet: imported workflows land at
  the top.
- Export and import (the user's ask, 2026-10-26; `internal/server/transfer.go`):
  ExportWorkflows writes some workflows, or all, to a file for
  ImportWorkflows, here or in another tower: a `WorkflowFile` as JSON
  (`format` "san_tower.workflows", `version` 1, `exported_at`, `tower`,
  and each workflow's name, steps with their places and values, and
  links), indented two spaces (protojson varies its spacing, so it is
  indented again). Runs and whether a workflow is on stay behind. A step's
  Connection and Storage (the settings whose kind says `connection_type`
  or `storage`) go into the file's `connections` and `storages` with only
  a name and a connection's type, by their id here: never their settings,
  so a bot's token never leaves the tower. One workflow is
  `<name>.workflow.json`, several `san_tower-workflows-2026-10-26.json`.
  ImportWorkflows creates the file's workflows switched off, a name in use
  getting " (2)", " (3)"...; each Connection becomes this tower's of the
  same name (case aside) and type, else the only one of that type, else
  nothing to pick; each Storage the one of the same name, else nothing
  (a storage is a folder: never guessed). The notes say what did not carry
  over as it was (a name taken, a connection matched otherwise or missing,
  a storage missing, a kind of step this tower does not have). A file of
  another format, a newer version or no workflows is refused, and a
  workflow that cannot be saved (e.g. two steps with one id) takes back
  the ones made before it, so a refused file leaves nothing.

## Chat

The Chat page, after Home in the menu (the user, 2026-10-28), is the tower's
own chat, kept in `tower.db` (`store/towerchat.go`). A chat talks with one
workflow (the user's choice), picked when it is made among those with a
Tower Chat step (ChatService's ListChatWorkflows), and named after it unless
named otherwise:
- What the user sends (SendChatMessage) is kept and starts a run of each of
  the workflow's Tower Chat steps; the workflow answers with Tower Chat Send
  steps. A workflow switched off, gone, or without a Tower Chat step takes
  nothing (FailedPrecondition, and nothing is kept): the page says so
  instead of the box to write in, and the chat's header has the workflow's
  On/Off switch.
- The page asks every second, while its tab is shown, for the messages
  after its newest (ListChatMessages' `after_id`) and how the runs of the
  user's messages still going are doing (`watch_runs`): under each message,
  Running with the last line its steps logged as they went (the Logs' live
  lines), or Failed with the error and a link to the workflow's Runs. A run
  that succeeded shows nothing: its answer is in the chat.
- A chat keeps its newest 2000 messages; the page shows the newest 100 and
  Older messages pages back. The chats are listed with the newest message
  first; Rename and Delete are in a chat's menu. Deleting a workflow deletes
  its chats.

## Connections

A connection is how workflow steps reach another program, managed on the
Connections page (ConnectionService), with a tab per type after All (the
user, 2026-10-28), and kept in `tower.db`: a name (unique,
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
- **Binance** (the user, 2026-10-27, as autotrade reaches Binance): a USD-M
  futures account through an API key and its secret, on a network:
  testnet (testnet.binancefuture.com, the default), demo
  (demo-fapi.binance.com, Binance's Demo Trading) or live
  (fapi.binance.com: real money). Test reads the wallet with the key
  (nothing is traded) and says when the account is in hedge mode, which
  Binance Order cannot trade. The secret is stored and listed as a bot's
  token is. From this PC only the test network answered: fapi.binance.com
  and demo-fapi.binance.com timed out (2026-10-27, likely the ISP).

## Finance

The Finance steps (the user, 2026-10-27: "new node category ... finance.
And add binance. It can execute order (limit / market / stoploss and take
profit option), get the candle, get the Porto, get order history, get open
or close position"; `engine/finance.go`) work on a Binance connection, with
the client autotrade uses (`internal/binance`, adopted from
`golang/apps/autotrade/internal/binance`: signed requests on the server's
clock, stop losses and take profits through the Algo Order API). They are
in Finance › Binance. Every line they log starts with the network
("testnet: BUY 0.012 BTCUSDT MARKET at 80000.1, order 11 FILLED"), so a run
says whether it traded for real.

- **Binance Order** places a Market order, or a Limit one at Price (Time In
  Force: GTC, Post Only, IOC or FOK), to Buy (long) or Sell (short)
  Quantity of Symbol (BTCUSDT, or btc/usdt; rounded down to the symbol's
  step, a price to its tick), Reduce Only or not, with Leverage set first
  when given (1 to 125). A Stop Loss and a Take Profit are trigger prices
  on the mark price: reduce-only stop-market and take-profit-market orders
  for the order's quantity (a market order's fill, a limit order's whole
  quantity, which protects it from its first fill). Refused before
  anything is placed: a stop on the wrong side of the entry (a limit's
  price, a market order's best ask or bid), a quantity below the symbol's
  least, stops with Reduce Only, and an account in hedge mode (the steps
  trade in one-way mode, as autotrade does). When the exchange refuses a
  stop after the order went in, the order is undone, so no position stays
  without the protection asked for: the stops already placed are
  cancelled, a limit order cancelled, and what filled closed at market;
  the step fails saying what was refused and what was undone. Order is the
  order as Binance took it, Order ID its id, Stop Loss Order and Take
  Profit Order the conditional orders.
- **Binance Candles** reads Symbol's latest candles (klines) at Interval
  (1m to 1M), Limit of them (100, up to 1500), oldest first, from the
  public market data of Network: Testnet (the default, the market Testnet
  orders fill on), Live or Demo; no connection or key. Candles is the list
  (open and close times, open, high, low, close, volume, quote volume,
  trades), Last Closed the newest that has closed (the last one is usually
  still forming), Close its close price.
- **Binance Portfolio**: Account the totals in USD (wallet, available,
  margin balance, unrealized PnL), Assets each asset held, Positions the
  open positions.
- **Binance Orders** lists orders, newest first, by Show: History, the
  Symbol's latest orders of any status (Limit, 50, up to 1000; Binance
  keeps a cancelled order that never filled for a few days only); Open,
  the orders that can still fill, of Symbol or every symbol; Stop Orders,
  the stop losses and take profits waiting to trigger. Last is the newest.
- **Binance Positions** lists the open positions, of Symbol or every
  symbol, and goes on by Found or None, then Done. Position is the first:
  side (LONG or SHORT), amount (negative for a short), entry, break-even,
  mark and liquidation price, unrealized PnL, notional.
- **Binance Close Position** closes Symbol's position at market
  (reduce-only, its whole amount), then cancels the symbol's stop orders
  still waiting; None when there is no position. Order is the closing
  order, Position the position as it was.

Tested against a fake Binance (`finance_test.go`, and the client's own
tests from autotrade), and live on the test network on 2026-10-27: a
workflow of the six steps through the tower's Binance connection (candles,
portfolio, a market buy of 10 XRPUSDT at 5x with its stop loss and take
profit, the position Found, both stops listed, the position closed and
its stops cancelled, then None and the history), and
`SAN_TOWER_BINANCE_LIVE=1` with the testnet key (`finance_live_test.go`,
`binance/live_test.go`): a market buy and a limit sell that filled at once,
each with its stops and closed, a post-only order that would have taken and
a stop on the wrong side refused, nothing left open. XRPUSDT, because
autotrade trades BTC, ETH, BNB and BCH with the same key. The test network
answers a market order with an average price of 0; the order is read back
for its price.

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
`storage` set; its value is the storage's id. A knowledge's storage is
deleted only with the knowledge (Knowledge below); a storage comes with the
name of the knowledge it holds (`knowledge`, empty for one of its own),
which the Storage page shows as a tag.

## Knowledge

A knowledge (the user, 2026-10-28; `store/knowledge.go`,
`server/knowledge.go`, KnowledgeService) is a knowledge graph of the
markdown files in a folder, adopted from the knowledge tool
([golang/packages/san_knowledge](../../packages/san_knowledge), whose
`graph` package the tower uses: the same store, sync and AI step), on the
Knowledge page, after Chat. CreateKnowledge takes a name and a path, and
makes a storage of the same name at that path (absolute, or relative to the
data folder, as any storage), the folder there if it is not there, and the
graph's `knowledge_data` in it with a `config.json` that tracks every
`.md` file in the folder (one already there, as in a project the knowledge
tool keeps, stays as it is); a folder that cannot be made leaves nothing
behind. Names are
unique, ignoring case, among the knowledge and among the storages, since
the storage takes the knowledge's name. DeleteKnowledge deletes the
knowledge and its storage and leaves the folder and its files;
DeleteStorage refuses a knowledge's storage, saying to delete the
knowledge. Knowledge are in folders as workflows are (Folders under
Workflows), in a tree of their own (`knowledge_folders`; ListKnowledgeFolders,
CreateKnowledgeFolder, UpdateKnowledgeFolder, DeleteKnowledgeFolder,
MoveKnowledge; ListKnowledge lists one folder's).

The graph is the knowledge tool's: every `.md` file is a doc node (its
first `# ` heading the title), every other heading a section under its
parent, a markdown link to another file or heading a reference, and
frontmatter `domain:`, `keyword:` and `summary:` are applied as written.
SyncKnowledge brings it in line with the files (no AI; a changed section
keeps its summary and needs a new one), GetKnowledgeGraph gives every node
and link, and GetKnowledgeNode one node with its links and its text from
the file (6000 characters at most). SummarizeKnowledge runs the AI step:
`claude -p` on this computer's Claude Code login, one call per doc with a
node waiting or without a domain, three at a time, with a system prompt
for knowledge about anything (the knowledge tool's own is about its
marketing research); it answers once all are done, a doc that failed is
tried again next time, a summary a person or frontmatter wrote stays, and
one run per knowledge at a time. The graph is opened per call (bbolt lets
one process at a time; a call that meets it busy says to try again), so
the knowledge tool (`knowledge --data <folder>/knowledge_data`) can use the
same graph between calls.

A sample (2026-10-28): the knowledge General (`D:/tmp/general`) holds 8
docs explaining San Tower (overview, workflows, chat, connections and
storage, message broker, browser remote, knowledge, deploying), with
frontmatter domains and summaries and links between them; its sections
wait for Summarize.

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

Listen on (the user, 2026-10-28: "add setting for only localhost or all
network"; BrowserRemoteService's SetBrowserRemoteNetwork,
`browser_remote.network` in `tower.json`) is This computer only (127.0.0.1,
the default) or All networks:
- Both servers then listen on every address of this computer, at the same
  ports, behind san_remote's guard for the local network (`server.LANHandler`,
  `server.LANAPI`, `hub.GuardLAN`): Host may also be a private network's
  address or this machine's name, never another name, and the extension's
  Origin rule is the same. A browser on another computer can attach to it,
  or a tower elsewhere (the Raspberry Pi, which has no browser) can post
  through the browsers attached here, with a San Remote connection to
  `http://<this computer>:7478`. The site services still reach the hub at
  127.0.0.1.
- A change restarts a running server on its new addresses; when that fails,
  it goes on as it was and nothing is saved. A san_remote already answering
  on 127.0.0.1 at those ports is refused, as on loopback.
- The page asks before All networks, warns while it is on (there is no
  password: anyone on the network can run code in the attached browsers and
  post as the user), and lists the URIs at this computer's local network
  addresses. On Windows, the first listen on every address may bring up a
  Windows Firewall prompt for the tower's executable.

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
  page's Logs, every workflow's or the active ones' (or, with `logs`, for
  the editor's Logs, one workflow's), and of any status or
  some, each then with its workflow's name and its log lines and their
  types (LogLevel, the Log step's Type: info, success, warning, failed,
  debug); a workflow's steps and links, and the
  Graph and RunSteps messages the database stores; a Setting with `storage`
  picks a storage), `connection.proto`
  (ConnectionService: list, create, update, delete, test; ConnectionSettings
  per type: San Remote, Telegram Bot, WhatsApp, Binance), `storage.proto`
  (StorageService: list, with the tower's data folder, create, update,
  delete; a storage's absolute path), `chat.proto` (ChatService: the
  chats, the workflows they can talk with, their messages and how the runs
  are doing, sending) and `knowledge.proto` (KnowledgeService: list,
  create with its storage, delete, move, the knowledge's folders, and a
  knowledge's graph: sync, its nodes and links, a node, summarize). `buf.gen.yaml` makes their Go
  code (`gen/`) and the UI's TS client. `buf.gen.san_message.yaml` makes
  the UI's TS client for san_message's proto; the Go side uses san_message's
  own generated code.
- `cmd/san_tower/main.go`: the CLI (`serve`, `dev`, `status`; `dev --watch`
  in `devwatch.go`).
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
  - `dev --watch` (`devwatch.go`; the user, 2026-10-26: the tower refreshes
    itself in dev, but not aggressively while Claude is implementing) runs
    the tower as a process of its own instead: `serve` built with `go build`
    from the source (`golang/apps/san_tower`, found from the current folder
    up) into a temporary folder. Every 0.5 s it looks at the source's Go
    files, go.mod and go.sum, and those of the modules go.mod replaces with
    a folder (San Remote, `../san_remote`, which the tower serves in its
    process), with the .js files beside Go files (San Remote's site
    scripts, go:embed); not hidden folders, node_modules or the data
    folder. Once they changed and then stayed the same for 2 s, it builds
    again, so several saves give one build, but only once the tower has no
    run in progress (TowerService's GetStatus, `runs_in_progress`; the
    user, 2026-10-27, before the PC runs out of memory): a build takes a few
    hundred MB for a minute, and Claude editing the tower through one of its
    runs would otherwise make one at each pause, of which only the last is
    used. It logs once that a change waits. A failed build is logged and the
    running tower goes on. A build that passes still waits until no run is
    in progress, since one may have started while it built: dev asks the
    tower for TowerService's Restart, which refuses while runs go on (the
    rule `san_tower restart` uses), and asks again every 0.5 s, logging why
    it waits. So Claude editing the tower through
    one of its own runs (Telegram, Claude Ask) is not cut short: the run
    ends, its reply gets out, then the tower restarts once, into the newest
    build. A tower that agreed stops by itself once its Tower Restart
    runs (Before) end, which dev waits for, up to 90 s, before it stops it.
    A tower that does not answer is stopped as it is. The new one
    starts on the same address (`SAN_TOWER_RESTARTED`, so it waits for the
    old one to let go). The tower it runs has `SAN_TOWER_DEV_CHILD` set: it
    stops when its stdin closes, which is how dev stops it (Windows cannot
    send a child Ctrl+C), so it also stops when dev ends however dev ends;
    and it takes a Restart as a stop, after which dev starts it again (from
    the newest build, so the UI's or `san_tower restart`'s Restart works
    too). One that has not stopped in 15 s is killed. One that fails by
    itself (a panic) is logged, and the next change starts it again.
    On Windows dev puts itself in a job object that kills what is in it
    when dev ends, however it ends (Ctrl+C, a closed window, Task Manager):
    Vite, the tower, a build, and what the tower started (claude). Vite
    shares the console, so its stdin does not close with dev. The builds'
    folder (`san_tower-dev-*` in the temporary folder) holds `dev.pid`;
    dev removes it when it ends, and at the start removes those whose dev
    is gone (one was left from a dev that was killed, 2026-10-27).
- `internal/server`: the HTTP server. It serves TowerService, BrokerService,
  WorkflowService (`workflow.go` the database's half, `engine.go` the
  engine's), ConnectionService (`connection.go`), BrowserRemoteService
  (`browser_remote.go`), StorageService (`storage.go`), ChatService
  (`chat.go`), KnowledgeService (`knowledge.go`), MessageService and the UI
  behind `Guard`, which only accepts a loopback Host and loopback pages.
- `internal/engine`: `kinds.go` the kinds of step and how each runs,
  `actions.go` HTTP Request and Publish to Topic, `social.go` the San Remote
  steps (clients of `san_remote api`, the At times), `sites.go` the Medium,
  LinkedIn, TikTok, Instagram, YouTube, ChatGPT, Gemini and Claude (Browser)
  steps, `claude.go`
  Claude Ask (CLI), `run.go` running a
  graph (`Execute`, with an `Env` for connections, storages and publishing), `expr.go`
  CEL, `engine.go` runs (recorded in the store), the triggers' pull
  subscribers and their streams, and `publish` for Publish to Topic,
  `schedule.go` the Schedule steps' timers and their Every, `telegram.go`
  the Telegram steps and the bots' pollers (`SyncConnections`, after a
  connection changes), `towerchat.go` the Tower Chat steps and `Chat`, which
  keeps a message sent on the Chat page and starts its runs.
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
  @username, date, and the message's JSON. A chat of the Chat page
  (`towerchat.go`) is a row (name, workflow), and so is each of its
  messages (chat, from the user or the workflow, text, Markdown or not, the
  runs, time). A knowledge (`knowledge.go`) is a row: name, its storage and
  its folder. Folders (`folder.go`) are two trees, the workflows'
  (`folders`) and the knowledge's (`knowledge_folders`), with the same
  code (`folderTree`). Errors wrap `ErrNotFound`,
  `ErrInvalid` or `ErrExists`, which the server turns into Connect codes.
- `internal/web`: the embedded UI. `dist/` is the Vite build (git-ignored but
  for `.gitkeep`). Paths without a file extension get `index.html`, so the
  UI's routes can be reloaded and linked to. Before the first UI build the
  page answers 503.
