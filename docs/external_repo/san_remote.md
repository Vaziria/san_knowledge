# San Remote

San Remote runs JavaScript in the user's own browser, the everyday Chrome
they are signed in to. Code is sent from Go: the CLI, or Claude through
MCP. A Manifest V3 extension in the browser holds a Connect RPC stream to a
small Go server on this computer. The server hands it code and tab commands
down that stream and waits for the replies.

It is the extension that
[san_gpt.md](san_gpt.md#alternative-a-chrome-extension) weighed against CDP,
built as a general tool rather than for one site.

How to install and use it is in
[golang/apps/san_remote/docs/readme.md](../../golang/apps/san_remote/docs/readme.md).
This document is the design and the reasons behind it.

## General.
1. The server and the CLI are written in Go: package
   `github.com/wargasipil/san_remote`, Go 1.26, in
   `./golang/apps/san_remote`, an ordinary folder (not a submodule).
2. The extension lives in `./typescripts/san_remote_extension`:
   TypeScript, React + Chakra UI for its popup, built by Vite, Manifest V3,
   Chrome or Edge 135+.
3. The extension and the server talk Connect RPC (connect-go, connect-es
   v2) with buf. The proto is in `golang/apps/san_remote/protos`, and one
   `buf generate` writes both the Go code and the TypeScript client.
4. JavaScript runs through `chrome.userScripts.execute`.
5. Commands come from the CLI and from an MCP server (`san_remote mcp`,
   registered in `.mcp.json` as `san_remote`).
6. The browser is the user's real one, wherever the extension is installed,
   not a `browser_profiles/` profile.
7. It is local only for now: the server listens on `127.0.0.1:7476`.
8. The CLI uses urfave/cli v3. The MCP server is hand-written (JSON-RPC 2.0
   on stdio), like autotrade's.
9. Site services sit on top of the generic commands. The first is Threads
   (`internal/threads`, the user, 2026-10-23: "add service for threads that
   handle post scheduling and other like san_medium"). It replaces the
   chromedp start `golang/packages/san_threads`, which was removed.

## Layout.

```
golang/apps/san_remote/
  protos/san/remote/v1/remote.proto   BrowserService (extension) + RemoteService (CLI, MCP)
  protos/san/remote/threads/v1/       the Threads service's data (messages only, no RPCs)
  gen/                                buf output (protoc-gen-go, protoc-gen-connect-go)
  internal/hub/    hub.go             browsers, pending commands, Attach loop, the guard
                   service.go         the generated handlers, pass-through to the hub
                   client.go          RemoteService client, "not running" error
  internal/hubtest/                   fake browsers that attach over Connect, for tests
  internal/threads/ thread.go         the thread markdown, its checks, the request
                   threads.js         what is known of Threads' pages (isolated world)
                   session.go         one tab: call into threads.js, waits, pace, front, screenshot
                   service.go         status, post, scheduled, reschedule, unschedule, posts
                   remote.go          the browser, through RemoteService
  internal/mcp/                       MCP server: remote_* and threads_* tools
  cmd/san_remote/                     urfave/cli v3 tree: serve, browsers, tabs, exec, open,
                                      navigate, close, focus, screenshot, threads, mcp
  e2e/                                tests against the real browser (SAN_REMOTE_E2E=1)
  build.sh / build.ps1                vet, test, build both binaries

typescripts/san_remote_extension/
  public/manifest.json                MV3; `key` pins the extension id
  src/background/  index.ts           service worker: Attach loop, keepalive, reattach, logging
                   commands.ts        tabs through chrome.tabs, code through userScripts
  src/popup/                          React + Chakra: status, warnings, settings, opens the panel
  src/sidepanel/                      React + Chakra: the debug log of commands and replies
  src/log.ts                          the command log in chrome.storage.session
  src/settings.ts                     settings, the status the worker writes
  src/ui.tsx                          mounting a page in Chakra, the status badge
  src/gen/                            protoc-gen-es output
```

## Design decisions.

**An extension, not chromedp.** The other san_* tools start their own
Chrome with a debugging port on a `browser_profiles/` profile. That brings
a separate login per profile, and an automation surface: sites notice a
debugging port, which is why `chromedp-undetected` is in their go.mod.

An extension runs in the browser the user already uses, with every sign-in
it has, and has nothing extra to detect. What it costs:
- an unpacked install
- a service worker Chrome likes to stop
- JavaScript-made events that are untrusted (see Limits in the usage doc)

**The extension connects to the server, not the other way round.** With
Chrome native messaging, Chrome would start the Go process itself, and the
CLI and MCP would need a second way in to reach it. Instead, the server is
its own process. The extension is one client of it, and the CLI and MCP are
others.

**Commands travel down a stream.** In a browser, Connect only has unary and
server-streaming calls; `fetch` cannot stream a request. So the extension
opens `Attach`, a server stream, and the server sends each command down it.
The extension answers each command with a separate unary `Reply`, matched
by `command_id`.

The commands are the RemoteService request messages themselves, carried in
a `oneof` of `AttachResponse`. The answers are the response messages, in a
`oneof` of `ReplyRequest`. So there is no second set of types, and the hub
is a pass-through.

**One server, many senders.** `san_remote mcp` is a client, not the
server: every Claude Code session starts its own MCP process, and several
servers would fight over the port and the browser. `serve` runs on its own,
and the MCP server says plainly when it isn't running.

**`chrome.userScripts.execute` to run code.** Manifest V3 forbids `eval`
in an extension, so the question was how to run a code string:

| Way | Problem |
|---|---|
| inject a `<script>` or `eval` into the page | the page's CSP blocks it on most big sites |
| `chrome.scripting.executeScript` | only runs functions that ship inside the extension |
| `chrome.debugger` + `Runtime.evaluate` | Chrome shows a "started debugging this browser" bar the whole time |
| `chrome.userScripts.execute` (Chrome 135+) | needs the user to turn on **Allow User Scripts** once; unpacked only |

The last one won. It runs in an isolated world (the default) or in the
page's own world, and the browser injects the code, so the page's CSP and
Trusted Types do not apply. That was checked on accounts.google.com and
YouTube.

**One shape for results.** The code is wrapped in an async function, and
the wrapper calls `JSON.stringify` in the page. Every value comes back as
JSON text, and an exception comes back as its stack. DOM nodes could not
cross anyway.

**Syntax errors.** When the code does not parse, Chrome runs nothing and
reports no error. The wrapper always returns an object, so an empty result
means exactly that. The extension then compiles the code without running
it, in a separate user-script world (`san_remote_syntax`) whose CSP allows
`new Function`, and reports the real `SyntaxError`.

**Keepalive.** Chrome stops an idle extension service worker after 30s. A
`fetch` stream alone does not count as activity, but any extension API call
resets the timer. So:
- The server sends a ping first, which also tells the extension it is
  attached, then every 20s.
- The worker writes its status to `chrome.storage.session` on each ping.
- If the worker is stopped anyway, or the server is down, a 30s
  `chrome.alarms` alarm wakes it and it attaches again.

It held 75s idle on one stream in testing.

**Binary protobuf between extension and server.** Connect-ES's JSON parser
refuses fields it does not know, so a server with a new field would break
an extension that has not been reloaded yet. Binary keeps unknown fields
quietly. This was found when `focus` was added to OpenTab (2026-10-23).

**Local only, and no token.** Whoever can send commands can act as the user
on every site they are signed in to. A local process running as the user
could read the browser's cookies anyway, so the thing to keep out is web
pages and other extensions:
- `serve` refuses a non-loopback listen address.
- Every request needs a loopback `Host` header, against DNS rebinding.
- BrowserService only accepts `Origin: chrome-extension://<the San Remote
  id>`, so no page or other extension can attach and receive the commands.
- RemoteService refuses any request with an `Origin`. Browsers always send
  one on a POST, and the CLI and MCP never do.

**A pinned extension id.** An unpacked extension's id normally comes from
its folder path. The public key in `manifest.json` (`key`) fixes it to
`kmlmiiljeiakjbilabmpdiegmoagkmkg` in every install, Chrome or Edge, so the
server can check it. The private key was not kept, because nothing is ever
packed into a `.crx`.

## Protocol.

```
CLI / MCP                    serve (hub)                        extension
   │  Exec(tab, code) ──────▶ pick browser, new command_id
   │                          pending[id] = waiter
   │                          Attach stream ── AttachResponse{id, exec} ──▶ userScripts.execute
   │                                                                          in the tab
   │                          ◀── Reply(ReplyRequest{id, exec | error}) ───
   │ ◀── ExecResponse ─────── waiter gets it
```

- **Picking a browser.** `browser` is an id or a name (case ignored). It
  may be empty when exactly one browser is attached.
- **Waiting.** The server waits for the command's own timeout (default 30s)
  plus 5s, so the browser's timeout message arrives before the server's.
- **Reattaching.** A browser that attaches again under the same id replaces
  its old stream. A browser that goes away fails its waiting commands at
  once.

Errors use Connect codes:

| Code | When |
|---|---|
| `Unavailable` | no browser attached, or it went away before answering |
| `NotFound` | no browser by that name; or a `Reply` for a command no longer waiting |
| `FailedPrecondition` | several browsers attached and none picked |
| `Aborted` | the browser answered with an error: an exception with its stack, a `SyntaxError`, a tab that does not exist, a page extensions cannot touch |
| `DeadlineExceeded` | no answer in time |
| `PermissionDenied` | the guard refused the request (HTTP 403) |

## Extension.

**Service worker.**
- **Start.** Every time it starts, it attaches.
- **On each ping.** It saves `{state: connected, server, since}`.
- **On a dropped stream.** It saves `disconnected` with the error and
  retries after 1s, doubling up to 30s. The alarm covers a worker that was
  stopped in between.
- **Settings change.** Changing the server or the name in the popup
  aborts the stream and attaches again.
- **Popup open.** Opening the popup sends `attach`, so its status is fresh.
- **Browser id.** It is a random UUID kept in `chrome.storage.local`, so
  a restarted worker is the same browser to the server.

**Commands.**
- `tab_id` 0 means the active tab of the last focused window, except for
  `close`, which needs an id.
- `open` and `navigate` start listening for the tab's load before they
  create or update it, so a fast load is not missed. `open` with `focus`
  also brings the window to the front.
- `exec` checks Allow User Scripts first. Without it the command fails with
  a message pointing at the extension's details page.

**Popup.** It shows:
- the connection badge and the error
- a warning with a button to the details page when Allow User Scripts is off
- **Open debug panel**
- the server URL and the browser name

**Debug panel** (the user, 2026-10-23: "extension page right of web that
show what send from server for debugging"). It is a Chrome side panel, so
it stays to the right of the page while the user browses.

The worker records each command when it arrives and again when it is
answered, in `chrome.storage.session`:
- **an entry**: the kind, a one-line summary (tab, world, url), the code
  or the command as JSON, the result or error, the state and the time it
  took
- **why that storage**: it outlives a stopped worker, is empty again after
  a browser restart, and the panel can read it later and follow it live
  through `storage.onChanged`
- **limits**: the last 200 entries, each code or result cut at 20,000
  characters; pings are left out
- **one writer**: only the worker writes the log, from the copy it holds in
  memory, so concurrent commands cannot lose entries; **Clear** asks the
  worker to empty it

The panel's header shows the last ping, which shows whether the keepalive
is working.

**Two more commands came with Threads.**
- `focus` makes a tab the active one and brings its window to the front.
- `screenshot` captures what a tab shows, which a failed Threads step saves
  next to its error.

`open` also gained `focus` and `new_window`.

## Threads.

**The composer, not the API.** Threads' API needs a registered Meta app and
a token renewed every 60 days. It takes media only from a public URL, and
it cannot schedule. The composer on threads.com has none of these limits,
so the service does what a person does there. Threads publishes a scheduled
thread by itself.

**Where it runs.** `threads.Service` runs in the process of the CLI or the
MCP server. It reaches the browser through `serve`'s RemoteService
(`threads.Remote`).
- **First design.** It started as a Connect `ThreadsService` inside `serve`.
- **Why it moved.** Every fix during the first live runs then needed the
  user to restart `serve`, three times in one evening (2026-10-24).
- **Now.** `serve` and the extension stay generic and rarely change. Site
  knowledge changes with a rebuild of the CLI, or `/mcp` for the MCP server.
- **What it costs.** Two processes do not wait for each other, so only one
  Threads command should run at a time. The proto keeps the service's data
  (`PostRequest`, `ScheduledPost`, …) as plain messages.

**Page knowledge and pace.**
- **The split.** `threads.js` holds everything known of Threads' pages. It
  runs in the extension's isolated world, where the page cannot see it, and
  is set up once per page. The steps, waits and pace are in Go (`session.go`).
- **Finding controls.** Threads' class names are generated, so controls are
  found by role and by the names Threads gives them: an aria-label, an SVG
  `<title>` ("Attach media", "More"), or the text.
- **A person's pace.** A session starts at the home page and looks at it for
  a few seconds, and every click waits 0.6–1.5 s first.

**Found on threads.com, 2026-10-23 and 24** (the interface in English, the
account @archetipe__):
- **A shown page.** A page that is not shown stalls Threads.
  - The composer's More menu never opened in a hidden tab: animation frames
    stop, and timers drop to one a minute after about 5 minutes hidden.
  - So clicking sessions use a tab of their own in the current window,
    brought to the front, and `front()` brings it back if the user switches
    away.
  - A window of its own was tried first. On Windows,
    `chrome.windows.create({state: 'maximized'})` gave a 0×0 window that
    stayed hidden even when focused. The extension now copies the current
    window's size for `new_window`, but the service does not use it.
- **Text.** The editor is Lexical. One `paste` event per post keeps the line
  breaks; `execCommand('insertParagraph')` and `insertLineBreak` did not.
- **Topic.**
  - "Community or topic" is typed key by key, and Threads lists
    suggestions.
  - A click on a suggestion does not choose it, nor do pointer events. The
    down arrow until the wanted suggestion is `aria-selected`, then Enter,
    does: the same as Medium's topics in san_medium.
  - The list can show the last query's suggestions first, so it must hold
    still before the arrows walk it.
  - The topic box keeps typed text whether or not it was chosen, so the box
    cannot prove a choice. Its own React state can, but that is too fragile
    to rely on.
- **Media.** The composer's file input takes files set through
  `DataTransfer`; the bytes are sent in 1 MB base64 pieces. A preview (a
  `blob:` image) inside the post's own box is the proof.
- **Scheduling.**
  - More → "Schedule..." opens a picker: month arrows, day cells labelled
    "Sunday 22 November 2026", text boxes `hh` and `mm` in 24 hours, and
    Done.
  - The composer then shows "Posting on Sun 22 Nov at 09:00 GMT+7", and
    Post becomes Schedule. Closer days read "Posting today at …", "Posting
    tomorrow at …", and within the week "Posting on Monday at …" (found
    2026-10-24, when the tool first refused to schedule a thread it could
    not read back).
  - That line is read back and must be the time asked for, or nothing is
    scheduled.
- **Scheduled threads.**
  - They are listed in the composer's Drafts, which opens in the composer's
    own dialog with the editors hidden.
  - A row is: the time line, the handle, the topic if any, the age, the
    first post's text, and "1 reply" for a thread.
  - Its More menu has "Edit schedule", which opens the same picker with
    Update, and "Delete draft", which deletes at once without asking.
  - So a scheduled thread is named by its text and time, which must match
    exactly one row.
- **Profile.** Each post's link `/@user/post/<code>` holds a
  `<time datetime>`. Its text is read from the post's box.

**Bugs the first live runs found** (2026-10-24, each now covered):
- **A topic click that did nothing.** It left the suggestion list open over
  the composer.
- **The Go side sent the staged files without JSON tags.** As `{ID, Name,
  Type}`, `threads.js` read `undefined` and attached an empty file. A unit
  test now checks the names `threads.js` reads.
- **Drafts went unrecognised, and rows were misread.** Drafts was not
  recognised because it holds the hidden composer, and a topic shifted a
  row's lines.
- **A failure after Schedule skipped the test's cleanup.** One run left a
  test thread scheduled, deleted by hand. The cleanup now runs before the
  post.

## Verified.

- **2026-10-23, `go test ./...`.** The hub, CLI and MCP server against fake
  browsers that attach over Connect: the guard (host, origin, other
  extension), picking a browser, browser errors, timeouts, detaching while
  a command waits, and reattaching.
- **2026-10-23, a throwaway headless Chrome 155.** It used a temp profile,
  with `Extensions.loadUnpacked` over `--remote-debugging-pipe
  --enable-unsafe-extension-debugging`, and Allow User Scripts turned on
  through `chrome.developerPrivate` on `chrome://extensions`. Checked:
  - the pinned id, attach, tabs, open, navigate, close
  - exec in both worlds, including that isolated code cannot see a global
    set in the main world
  - exceptions, syntax errors in both worlds, timeouts
  - 75s idle on one stream
  - the main world on accounts.google.com (Trusted Types) and YouTube

  GitHub and Tokopedia showed Chrome's error page to the headless browser,
  so they are untested.
- **2026-10-23, the user's real Chrome.** `e2e.TestGoogleSearch` passed:
  open google.com, type "cute cat" key by key, submit, read the results.
  The first run worked but showed nothing. The tab opened in a Chrome window
  behind VS Code and was closed when the test ended. That led to `focus` on
  `open` and to leaving the tab open. Whether Windows let Chrome come to the
  front on the second run is not confirmed yet: Windows can refuse and only
  flash the taskbar button.
- **2026-10-23, the debug panel**, in a throwaway headless Chrome on its own
  server (`127.0.0.1:7499`, so it did not attach to the user's). A few
  commands were run, then `sidepanel.html` and `popup.html` were
  screenshotted. The panel listed each command, newest first, with its code,
  result, error stack, timing and the last ping. Opening it from the popup
  button inside Chrome's real side panel has not been tried yet.
- **2026-10-24, Threads, on @archetipe__, in the user's Chrome.**
  - `e2e.TestThreadsSchedule` passed (132 s) and left nothing scheduled.
    The steps:
    1. Scheduled a two-post test thread for 23 Nov 09:00, with the topic
       Programming and an image.
    2. Found it among the scheduled threads.
    3. Moved it to 24 Nov.
    4. Deleted it, and confirmed it was gone.

    The user switched away once, and `front()` brought the tab back.
  - `threads status` read "signed in as @archetipe__".
  - The first real thread was scheduled with `threads post`: VPN Bagian 1,
    eight posts, topic VPN, two diagrams, for 2026-10-25 19:00 WIB. Threads
    showed "Posting tomorrow at 19:00 GMT+7", and the check found it in
    Drafts.
  - The VPN series recap followed: eight posts, for 2026-10-26 19:00 WIB.
    The first try stopped before Schedule on the unread "Posting on Monday
    at 19:00 GMT+7"; after the fix it was scheduled and found in Drafts.
    `threads scheduled` then listed both threads, a day apart.
  - Not run yet: `threads posts` and `--now`.

## Open questions.

- **From another machine** (the Pi, heri-office-2 over VPN): this needs a
  token and TLS. The Origin check only protects against browsers.
- **Trusted input.** Sites that check `isTrusted` would need
  `chrome.debugger` with `Input.dispatchMouseEvent` / `dispatchKeyEvent`,
  per site, with Chrome's debugging bar showing meanwhile.
- **A Go client package**, so `san_tiktok`, `san_youtube` and the others
  could move off chromedp onto the real browser.
- **Frames.** Only a tab's top frame runs code.
- **More of Threads.** The composer's GIF, sticker, poll, quote, location,
  music and reply settings. "Add AI label" stays off for the user's own
  articles (the user, 2026-10-24).
- **One Threads command at a time across processes.** It could become a
  lock file, if the CLI and Claude ever post at the same time.
