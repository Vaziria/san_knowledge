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
9. Site services sit on top of the generic commands.
   - **Threads** came first (`internal/threads`; the user, 2026-10-23: "add
     service for threads that handle post scheduling and other like
     san_medium"). It replaces the chromedp start
     `golang/packages/san_threads`, which was removed.
   - **Vidara** followed (`internal/vidara`; the user, 2026-10-24: "build
     ability to post video on that", then "add ability to download video
     from vidara link to").
   - **Facebook** came next (`internal/facebook`; the user, 2026-10-24:
     "add ability to remote facebook").
   - **X** followed (`internal/x`; the user, 2026-10-24: "can we add
     support to x").
   - **Medium, LinkedIn, TikTok and ChatGPT** came together
     (`internal/medium`, `internal/linkedin`, `internal/tiktok`,
     `internal/chatgpt`; the user, 2026-10-25: "make san_medium,
     san_linkedin, san_tiktok have browser remote version in san_remote and
     have node in tower", then "san_gpt too"). The chromedp packages stay.
   - **Gemini** followed the same day (`internal/gemini`; the user: "add
     gemini ask", through the browser like ChatGPT rather than an API key),
     then **Claude** on claude.ai (`internal/claude`; the user: "add node
     claude ask (cli) and claude ask (browser)"; the CLI one is the tower's
     own, running `claude -p`).
   - All of them work in a tab through `internal/page`.

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
  internal/pagetest/                  a fake page that keeps the files a service stages, for each
                                      site's TestStageWholeFile (a file over several pieces arrives whole)
  internal/page/   page.go            a site service's tab: call into the site's script, waits,
                                      pace, front, screenshot; the browser through RemoteService
  internal/plaintext/ plaintext.go    markdown written as a post's plain text (goldmark), for the
                                      Threads, X, Facebook and TikTok post files
  internal/threads/ thread.go         the thread markdown, its checks, the request
                   threads.js         what is known of Threads' pages (isolated world)
                   session.go         Threads' page state, its waits, the home page
                   service.go         status, post, scheduled, reschedule, unschedule, posts
  internal/vidara/ vidara.go          the upload's checks, links and codes, the types
                   vidara.js          what is known of Vidara's pages and its player (isolated world)
                   service.go         status, upload, videos, delete
                   download.go        download: the player's HLS stream, saved and remuxed
  internal/facebook/ facebook.go      the post markdown, its checks, Facebook's date and time labels
                   facebook.js        what is known of Facebook's and Business Suite's pages (isolated world)
                   service.go         status, post (profile, Page, group), posts, delete
  internal/x/      x.go               the thread markdown, X's weighted length, its checks
                   x.js               what is known of X's home page, composer and profiles (isolated world)
                   service.go         status, post (now or scheduled), scheduled, reschedule,
                                      unschedule, posts, delete
  internal/medium/ article.go, html.go the article markdown, cut as Medium's editor takes it (from san_medium)
                   medium.go          ids, Medium's time formats, the schedule's checks
                   medium.js          the editor, the publish page, the stories page (isolated world)
                   service.go         status, draft, schedule, scheduled, unschedule, drafts, get, delete
  internal/linkedin/ article.go, html.go the article markdown, with a cover (from san_linkedin)
                   linkedin.go        ids, LinkedIn's time formats, the schedule's checks
                   linkedin.js        the feed, the editor and its dialogs, the share box, Articles
                   service.go         status, draft, schedule, scheduled, unschedule, drafts, get, delete
  internal/tiktok/ tiktok.go          the post file, its checks, Studio's list rows
                   tiktok.js          tiktok.com's menu, Studio's upload page and Posts list
                   service.go         status, post (scheduled, or now for Only you), posts, delete
  internal/chatgpt/ chatgpt.go        the ask's checks, chat links
                   chatgpt.js         the composer, Temporary chat, the turns (isolated world)
                   copy.js            the answer's Copy button, caught in the page's own world
                   service.go         status, ask
  internal/gemini/ gemini.go          the ask's checks
                   gemini.js          the prompt box, Temporary chat, the answers (isolated world)
                   copy.js            the answer's Copy button, caught in the page's own world
                   service.go         status, ask (in a tab brought to the front)
  internal/claude/ claude.go          the ask's checks
                   claude.js          the prompt box, incognito, the answers (isolated world)
                   copy.js            the answer's Copy button, caught in the page's own world
                   service.go         status, ask
  internal/api/    api.go, sites.go,  `san_remote api`: the site services over Connect
                   gemini.go,
                   claude.go
  internal/mcp/                       MCP server: remote_*, threads_*, vidara_*, facebook_*, x_*,
                                      medium_*, linkedin_*, tiktok_*, chatgpt_*, gemini_* and
                                      claude_* tools
  cmd/san_remote/                     urfave/cli v3 tree: serve, browsers, tabs, exec, open,
                                      navigate, close, focus, screenshot, threads, vidara, facebook, x,
                                      medium, linkedin, tiktok, chatgpt, gemini, claude, mcp, api
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
(`page.Remote`).
- **First design.** It started as a Connect `ThreadsService` inside `serve`.
- **Why it moved.** Every fix during the first live runs then needed the
  user to restart `serve`, three times in one evening (2026-10-24).
- **Now.** `serve` and the extension stay generic and rarely change. Site
  knowledge changes with a rebuild of the CLI, or `/mcp` for the MCP server.
- **What it costs.** Two processes do not wait for each other, so only one
  Threads command should run at a time. The proto keeps the service's data
  (`PostRequest`, `ScheduledPost`, …) as plain messages.
- **Other programs (2026-10-24).** The tower's workflows needed the site
  services over gRPC. Putting them back in `serve` would bring back the
  restarts, so `san_remote api` is a process of its own, a client of
  `serve` like the CLI and MCP, with ThreadsService, XService,
  FacebookService and VidaraService, and RemoteService passed through. It
  only schedules posts, as the user chose for workflow steps. Vidara's
  download joined it on 2026-10-25 for the tower's Vidara Download step
  (DownloadVideo: a link or code, and an absolute path, since a relative one
  would land wherever the api was started). Missing folders on that path
  are made (the user, 2026-10-25, after a step's Save To named a folder
  that was not there yet). MediumService, LinkedInService, TikTokService and
  ChatGPTService joined it the same day, for the tower's steps of those
  sites, and GeminiService and ClaudeService after them; an article without a time stays a private draft, which is not a
  post.

**Page knowledge and pace.**
- **The split.** `threads.js` holds everything known of Threads' pages. It
  runs in the extension's isolated world, where the page cannot see it, and
  is set up once per page. The steps, waits and pace are in Go (`service.go`,
  on the tab of `internal/page`).
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

## One tab, shared.

`internal/page` is what a site service needs of a tab. When Vidara came,
it was moved out of `internal/threads` (2026-10-24) rather than copied.
- **What it does:**
  - open and close the tab
  - call the site's script with its global and its `ErrPage`
  - `Until`, `Pause`, `Look`, `Press` and `Front`
  - `Fail`'s screenshot, named after the site
- **What each site keeps:** its page state, the waits on that state, and
  its home page.
- **`Front`** asks the page's visibility itself, so a site's script does not
  have to report it.

## Vidara.

**The web uploader, not the API.** Vidara has an API with an `api_key`
(Settings → API): one multipart POST from disk, plus rename, move, list and
delete. The user chose the web uploader instead (2026-10-24), in the same
way as Threads' composer. The file therefore goes through the browser.

**Found on vidara.to, 2026-10-24** (the interface in English, the account
getresolved):
- **The site.** Pages are rendered on the server with Tailwind; every
  sidebar link loads a new page.
  - The dashboard is `/index`; `/` is the public landing page, even when
    signed in.
  - Signed in means a Logout link in the header.
  - The user's name is only on Settings: "Hello, getresolved!".
- **Exec during a page load.** In a quick test, `exec` kept answering while
  a click loaded the next page: first from the old page, then from the new
  one while it was still loading. One of five e2e runs still failed right
  after an upload, at a step the log did not show. So Vidara's waits treat a
  call the browser refuses (`Aborted`) as "not yet", and name it if the wait
  runs out. `Fail` also says why it has no screenshot.
- **Upload.**
  - The Upload page is Dropzone. Its hidden `input.dz-hidden-input` is what
    Browse files opens; files set on it go through the page's own flow.
  - That flow fetches an upload server (`/ajax/media`), then sends the file
    in 16 MB chunks (`resumable-upload.js`).
  - The page remembers an unfinished upload by the file's name, size and
    `lastModified`. The staged `File` gets the real file's time, so a stalled
    upload continues when it is run again.
  - The page takes 10 MB to 10 GB. It has no title box: the video is named
    after the file, without its extension. So `--title` names the staged
    `File`.
  - A finished upload submits a form to `/export`. That page holds the
    link (`#textarea_download`) and the title (in `#textarea_html`'s `<a>`).
  - The proof is the row in My videos, found by searching its code.
- **Moving the bytes.** The file goes into the tab in 4 MB base64 pieces,
  about 190 ms each (about 20 MB/s). Each piece becomes a `Blob` at once,
  so the bytes wait in Chrome's blob store, not in the page's memory. The
  page's own uploader then sends them at the user's upload speed (about
  1 MB/s that day).
- **My videos.**
  - A row's checkbox carries `data-filecode`. Its columns are read by the
    table's own names: Filename, Size, Views, Uploaded, Status.
  - The search box (`?search=`) finds titles and filecodes.
  - The row's Delete button opens `#modal-delete-confirm`, which names the
    video. "Delete Permanently" then posts and reloads the page.
  - With two-factor login on, Vidara asks for a code in a `prompt()`. The
    tool cannot answer that; 2FA was off.
- **Timers in a background tab.** Chrome runs them once a second there. A
  search typed key by key in a background tab took 19 s, so only a shown
  session waits between keys. The box has no key handlers, and the form
  sends only its final text.
- **A video's page and its player.**
  - `/v/<code>` embeds a JW Player from another host:
    `gaqghe.com/e/<code>?vt=…`.
  - Its "Download file" button is an ad: it sits in a box marked "Ad" and
    links to another site. There is no real download link.
  - Frames do not run code, so the tool loads the player's address in the
    tab itself.
  - The player gets an HLS stream (`/api/stream`, then `master.m3u8` on
    `virelios.live`). The stream is one variant: Vidara's encoding (720p at
    3 Mbps for a 720p upload at 6 Mbps). Its MPEG-TS segments are named
    `.css`, and there is no encryption.
  - The segments load from the player's page but answered 404 from a
    vidara.to tab. So the player tab fetches them, and the tool reads them
    back in 4 MB base64 pieces.
  - The stream's address is read from the page's resource timing, in the
    isolated world. JW Player's `getPlaylistItem()` has it too, but only in
    the page's own world.
  - ffmpeg remuxes the segments into an .mp4 (`-c copy`). The 16 s test
    clip came back as 16.02 s, h264 1280x720 with aac.
- **The dashboard's count.** It showed "Videos 161" while My videos listed
  none. That number is not used.

## Facebook.

**The composers, not the Graph API.** The Graph API needs a registered
Meta app with reviewed permissions, and it cannot post to a personal profile
at all. The user wanted all four (2026-10-24): post to the profile,
post or schedule on a Page, post to groups, and read and manage. So
`internal/facebook` goes through Facebook's own composers, as Threads does.

**Indonesian, not English.** The account's interface is Indonesian
(`lang="id"`), and controls are found by their names. Threads and Vidara
are read in English. Here the tool checks `lang` and stops with
`ErrLanguage` rather than look for English names.

**A Page through Business Suite.** "Buat postingan" on the Page asks to
switch the whole Facebook session to the Page ("Beralih profil"). That
would change what the user sees in every tab. Meta Business Suite's
composer posts as the Page without switching, so the tool opens
`business.facebook.com/latest/home?asset_id=<id>` and presses "Buat
postingan". The id is the first `"delegate_page_id"` in the Page's own
HTML, read after opening the Page as a person would. The composer's Page
box must name that Page before any text goes in.

**Found on facebook.com, 2026-10-24** (the account Santoso Heri, two Pages,
26 groups):
- **The profile composer.** "Apa yang Anda pikirkan?" opens a dialog with a
  contenteditable editor. Paste works as for Threads (a `ClipboardEvent`
  with a `DataTransfer`), and so does setting files on its file input.
- **Audience.** "Audiens postingan …" opens "Pilih audiens": radio inputs
  whose label is in an ancestor ("Publik", "Teman", "Hanya saya"), then
  "Selesai".
- **Scheduling.** "Opsi penjadwalan" is a panel, not a dialog, so its
  controls are found page-wide.
  - "Buka Pemilih Tanggal" opens a grid "Pilih Tanggal" with "Bulan
    Berikutnya" / "Bulan Sebelumnya". Its days are named "Senin, 16 November
    2026". Days past 29 ahead are disabled, which is where `MaxDays` comes
    from.
  - The date box then reads "16 Nov 2026". The time list is in 5-minute
    steps; today's starts about 20 minutes ahead (`MinLead`).
  - Text typed into either box does not stick, so the picker and the list
    are used. The date goes first: changing it resets the time.
  - A click on a day does nothing while the time list is open.
  - "Jadwalkan untuk nanti" confirms. The composer's button then reads "16
    Nov pada 09.00", and its submit button turns from "Posting" into
    "Jadwalkan Postingan".
- **The Content Library.** The profile's "Kelola postingan" link (an
  `<a role=link>`) opens `/professional_dashboard/content/content_library/`.
  - Tabs Diterbitkan, Terjadwal, Draf. A row is `[role=row]`: the text, then
    "Terjadwal •", then "16 Nov pukul 09.00" or "Hari ini pukul 12.18".
  - "Tindakan untuk postingan ini" opens a menu with "Hapus postingan".
    Facebook asks "Hapus postingan secara permanen?" (Hapus / Batalkan).
  - After Hapus, the first reload of the library still listed a deleted
    post; a minute later it was gone. So `delete` reloads up to four times.
- **Business Suite's home and composer.**
  - The home page has no `navigation` or `main` landmark, which the first
    sign-in check required. The Page run waited its minute and stopped there
    (2026-10-24). Signed in now also means Business Suite's "Buat postingan"
    button is shown.
  - "Buat postingan" loads the composer as a page of its own,
    `/latest/composer/?asset_id=…`, not a dialog.
  - The editor is named "Tulis ke dalam kotak dialog…" and takes a paste. The switch "Atur tanggal dan waktu" shows a
  date box (dd/mm/yyyy), where typing works, and spinbuttons "jam" and
  "menit". The buttons are "Jadwalkan" or "Terbitkan". Leaving asks "Keluar
  dari halaman?".
- **Groups.** "Tulis sesuatu..." opens "Buat postingan" with an editor
  "Buat postingan publik...". A member cannot schedule. Joined groups are
  the ones with "Lihat Grup" on `/groups/joins/`.
- **The pickers need a shown page,** like Threads' menus. While the user
  typed elsewhere, Windows kept Chrome from coming to the front, and a day
  click went nowhere. So the exploration and the live runs waited until the
  user had left the PC.

## X.

**The composer, not the API.** Posting through X's API needs a paid
developer plan. So `internal/x` goes through the composer, as Threads does.
The user asked for all of it (2026-10-24): post and schedule, list
scheduled, reschedule or delete scheduled, list or delete published. They
chose a schedule-only test, since nothing can be posted "only me" on X.

**English, and X's own test ids.** X's controls carry `data-testid`
attributes that do not change with the language: `SideNav_NewTweet_Button`,
`tweetTextarea_0`, `addButton`, `fileInput`, `scheduleOption`,
`scheduledConfirmationPrimaryAction`, `tweetButton`, `unsentButton`,
`confirmationSheetCancel`. Only the "Will send on" line and the button's
label ("Post", "Post all", "Schedule") are read as English text.

**Found on x.com, 2026-10-24** (the account Warga Sipil, @archetipe_, made
that month, no posts, no Premium):
- **The compose dialog** is `/compose/post`, opened by the side bar's Post
  button. The home page keeps an editor of its own behind it, so editors are
  looked up inside the dialog. A paste (a `ClipboardEvent` with a
  `DataTransfer`) works, and the counter ring and the Post button follow it.
- **Rendering needs a shown page.** In a background tab the schedule dialog
  drew only its title, and a screenshot failed ("image readback failed").
  `status` and `posts` only read, and work in the background.
- **A thread.** "Add post" (`addButton`) adds `tweetTextarea_1`, and so on.
  The button reads "Post all" for a thread.
  - A `.click()` on it while the editor still had the focus emptied the
    post instead of adding one. A mouse click moves the focus first, so
    every press in `x.js` blurs the focused element first.
- **Media.** The dialog has one file input (`fileInput`, jpg, png, webp,
  gif, mp4, mov). Files set on it go to the post that has the cursor. A
  post's attachments (`attachments`) follow its editor.
- **The schedule dialog** is `/compose/post/schedule`: six dropdowns (month,
  day, year, hour 1-12, minute, AM/PM) in the account's time zone (Western
  Indonesia Time), with years to 2028.
  - Their ids count up each time it opens (`SELECTOR_1`… then `SELECTOR_7`…),
    so each is known by its options. The day is the one with a 28th:
    November has no 31st.
  - React only takes a value set with the native setter, then a `change`
    event.
  - The dialog shows "Will send on Fri, Nov 20, 2026 at 9:00 AM". After
    Confirm the composer shows the same line on each post, and its button
    reads "Schedule". A thread can be scheduled.
- **Closing the composer** asks "Save post?" (Save / Discard). Drafts
  (`unsentButton`) has two tabs, Unsent posts and Scheduled
  (`/compose/post/unsent/scheduled`).
- **X taking a thread** is a toast, "Your post will be sent on Fri, Nov 20,
  2026 at 9:00 AM" with a View link, or "Your post was sent." The composer
  can still be open when it shows, so the toast counts as taken.
- **A refusal** keeps the composer open. "Your account is suspended and is
  not permitted to send posts." came as a toast. "Some of your media failed
  to upload." came as a red notice in the composer, a `role=status`
  element. The tool reports either one and discards the draft.
- **Drafts → Scheduled.** Each thread is an `unsentTweet`: "Will send on
  …", its first post (`tweetText`), and "1 more post" for a thread.
  - A click opens it in the composer with its posts, its media and its
    time (`scheduledTweetIndicator`), and the button reads Schedule. A new
    time and Schedule move it: X keeps one thread, at the new time.
  - "Edit" (no test id) puts the list in Edit mode with "Select all" and
    "Delete". Each thread sits in a `<label>` with a hidden checkbox, and a
    click ticks it. Delete asks "Discard unsent posts. This can't be undone
    and you'll lose your unsent posts." (`confirmationSheetConfirm` Delete),
    then "Your selected unsent posts were deleted."
- **A published post's "…" menu** (`caret`, "More") starts with Delete, then
  Pin to your profile and the rest. Its Delete was not pressed, so the
  question after it was not seen. The tool expects X's usual
  `confirmationSheetConfirm`.
- **A profile's posts** are `article[data-testid=tweet]`, with `time` (its
  `datetime`) inside the post's `/handle/status/id` link, and `tweetText`.
  A pinned post comes first whatever its age, so `posts` sorts by time.
- **Downloading a post** (the user, 2026-10-25: "x post downloader that
  can download media and caption and text and tag, and input is link").
  The page draws a video from a `blob:` URL (Media Source), and X clears
  its resource timings, so neither the DOM nor `performance` gives the
  file. X's own app does: the React props of the post's `article` (from its
  `__reactFiber$…` key, walking up to the props whose `tweet.id_str` is the
  post) hold the post as X's API sent it.
  - `extended_entities.media`: each video's `video_info.variants` are
    direct mp4 links on video.twimg.com, by bitrate (up to 1080×1920 at
    10 Mbit/s for the test post), and a photo's `media_url_https` takes
    `?format=jpg&name=orig`.
  - `note_tweet.text` is a long post's whole text; `full_text` stops at
    280 with a t.co link. `entities.urls` gives the t.co links' targets.
  - An X Article is drawn whole on its post's page: the props of
    `twitterArticleReadView` hold `twitterArticle`, with `title`,
    `cover_media`, `media_entities` and `content_state` (Draft.js blocks and
    an entity map: MARKDOWN for tables, MEDIA for images, LINK).
  - post.js reads them in the main world (`CallMain`) and returns plain
    data. Go saves the files from X's media servers with no cookies, which
    they do not need. Nothing on the page is clicked, and the tab is a
    background one.

## Synthetic input, site by site.

The chromedp tools typed with real key events (`Input.dispatchKeyEvent`),
clicked with real mouse events, and took file choosers with
`Page.setInterceptFileChooserDialog`. An extension has none of that without
`chrome.debugger` and its bar. What reached each page from the isolated
world instead, found live on 2026-10-25:

| | Medium | TikTok Studio | ChatGPT | LinkedIn |
|---|---|---|---|---|
| text box | one keydown + keypress, then `execCommand('insertText')` (title) | paste event (Draft.js caption) | `execCommand('insertText')` (ProseMirror) | `insertText` (title, alt, caption); per line with an Enter keydown (share box, Quill) |
| rich text | paste event with HTML | paste event, lines kept | — | paste event with HTML (ProseMirror) |
| Backspace | keydown, which Medium handles | keydown on a script-made selection, half a second later | — | keydown |
| a file | the input the button adds, given the file after `.click()` | the upload page's own input | — | the input the button clicks or adds, given the file |
| a list item | `.click()` on the topic suggestion | whole mouse press (mousedown, mouseup, click) on the hashtag and the "…" menu item | — | `.click()` on the row menu's items |
| a picker | `.click()` on its buttons; Escape now and then closes it | `.click()`; Escape closes the time list | — | whole mouse press on the month arrows, days and times |

What did not work:
- **Medium's title with insertText alone** left Medium's "Title" placeholder
  in place and made no story. A keydown first makes Medium drop it.
- **A keyCode for punctuation.** "(" sent keyCode 40, which is ArrowDown:
  the rest of the title landed in the body. Only letters, digits and space
  get one now.
- **TikTok's caption with insertText** doubled a line and then broke Studio
  ("Something went wrong"; Retry gave an empty upload page). The paste event
  splits lines into Draft.js blocks itself.
- **A Backspace right after the select-all** deleted one character: Draft.js
  reads the selection on a later event.
- **Arrow keys and Enter in Medium's topic box** did nothing; the suggestion's
  own click does what it did not with a real mouse (there, the box lost the
  focus first and closed, 2026-10-11).

## Medium.

**The editor, not an API.** Medium issues no API tokens. The service is
san_medium's flow and selectors (classic editor, `.postArticle-content`,
`graf--*`) with the input above.
- **Signed in** is the header's Write link (`/new-story`); the name is the
  "user options menu" avatar's alt, the handle the sidebar's Profile link.
- **A new story** is made once the title is in: the tab moves to
  `/p/<id>/edit`.
- **An image:** the placeholder's Backspace leaves an empty paragraph with
  the + button beside it; + opens the menu, and "Add an image" adds a file
  input to the page and clicks it. That click opens no dialog (no user
  gesture), and the input is given the file. The upload ends with
  `data-image-id` and an `https://cdn-images-1.medium.com` src.
- **Alt text** goes through the image toolbar's alt button, a paste into
  `.js-textAreaEditor`, and its Save; **the caption** is a paste into the
  focused figcaption. Medium keeps a no-break space before a caption's last
  word.
- **The publish page** comes from the editor's Publish (a new draft) or the
  Drafts tab's row menu, "Publish story". The date picker's buttons are the
  month arrows, the days, 24 hours and 60 minutes, as in san_medium. "Schedule
  to publish" leads to the story's page.
- **The Scheduled tab** opens as a calendar; its table needs
  `?tab=posts-scheduled&view=table`, the one link the service loads itself.
  A row says "Scheduled Nov 24, 9:00 AM", without the year. "Review scheduled
  story" in its menu opens the publish page with "Cancel scheduling".

## LinkedIn.

The service is san_linkedin's flow and selectors (Ember, ProseMirror, the
media dialog, the share box, the Articles dialog), with the input that
worked on Medium. It first met the live site on 2026-10-25, in a second
Chrome signed in to LinkedIn. Most of san_linkedin's steps held:
- **The editor in the feed's iframe.** Code runs only in the top frame; the
  script reaches the same-origin `/preload/` frame through `contentDocument`.
  That worked as built.
- **File inputs.** "Upload from computer" (the cover) and the toolbar's
  image button take the file the same way, through a capture listener and
  the inputs added after the press. Both worked as built.
- **The share box (Quill).** The post's text is `insertText` per line, with
  an Enter keydown between; Quill handles the Enter itself.

What changed on the live site:
- **The name.** The redesigned top bar (hashed class names, no
  `.global-nav__me`) has no photo with the name. The feed's profile card,
  the first `/in/` link with text, has it.
- **The share box shows before its text box.** `.ql-editor` comes a moment
  after the dialog; typing at once typed nowhere. The service now waits for
  it.
- **"Draft", not "Draft - saved".** A draft opened with no edits says only
  "Draft", which now counts as saved (the wait before Next timed out).
- **The schedule view's calendar and time list** ignore `.click()` and a
  lone mousedown on the month arrow, the days and the times. A whole mouse
  press (pointer and mouse events) works. The date and time boxes and the
  dialog's Next take `.click()`.
- **The Articles dialog's question** ("Delete draft?", "Move to drafts") is
  pressed only once it is shown, as other presses are, and again if it is
  still there after 5 s. A press made the instant it was drawn was lost
  once.

## TikTok.

The service is san_tiktok's flow with Studio as it is on 2026-10-25:
- **The upload page's file input** (`accept="video/*"`) is in the page from
  the start; Studio uploads at once and shows "Uploaded（62.88KB）".
- **The schedule** changed since san_tiktok: Picker comboboxes "Select date"
  and "Select time", a `DatePicker__panel` whose days are buttons named
  "Tuesday, November 3rd, 2026" (disabled outside the window), a
  `TimePicker__panel` with options "Hours 19" and "Minutes 00" (5-minute
  steps), and a time zone box "(GMT+7) Local Time". The calendar offers 30
  days ahead now, not 10.
- **Only you cannot be scheduled:** choosing it switches When to post back to
  Now, with "Private videos can't be scheduled." So a scheduled post is for
  Everyone, as in san_tiktok, and Who can see it is set before the schedule.
- **Posts** rows read "00:06 <caption> Nov 3, 7:00 PM Everyone 0 0 0". A
  row's "…" opens Download and Delete; "Delete post?" is a `role=dialog`.

## ChatGPT.

**The page sends, the tool steers.** san_gpt's rule holds: the page's own
code makes every request. Its fetch tee needed CDP; here the answer is read
from the page instead.
- **The composer** is a ProseMirror "Ask ChatGPT"; insertText makes each
  line a paragraph, and Send enables.
- **Send too soon** lost a prompt once: the composer emptied and no turn
  came. The service waits a second, presses Send, and writes the prompt once
  more if no turn and no Stop button come.
- **Turns** are `[data-turn-key]`, each the prompt and its answer, with
  `data-talvt-turn-state` in_progress, then complete. A new turn reads
  complete for a moment before the answer starts, so the service waits for
  answer text that holds still with no Stop button.
- **The answer's markdown** comes from its Copy button: in the page's own
  world, `navigator.clipboard.write` and `writeText` are caught for that one
  press (ChatGPT writes a ClipboardItem with text/plain and text/html) and put
  back at once, so nothing reaches the clipboard. The DOM's text is the
  fallback. `page.Session.CallMain` was added for this.
- **Temporary chat** is the button "Temporary chat"; the URL gains
  `?temporary-chat=true`. The user chose it for the tower's step (2026-10-25).
- **A background tab** is enough: the answer streams and renders in a
  hidden page; only the script's own timers slow down.

## Gemini.

**Like ChatGPT, but shown.** The same shape as `internal/chatgpt` (the page
sends, the tool steers; Copy button caught in the page's own world), found
live on 2026-10-25 with the interface in English (en-GB):
- **The prompt box** is a Quill editor, `rich-textarea .ql-editor` ("Enter a
  prompt for Gemini"); insertText reaches it, and "Send message" shows.
- **A shown page is needed to send.** A Send pressed in a background tab
  (`.click()`, a whole mouse press, an Enter keydown, or from the page's own
  world) did nothing until the tab came to the front; then the prompt went,
  once. So an ask opens its tab in front and keeps it there (`Front`), as
  the Threads steps do; status reads in a background tab.
- **Answers** are `model-response` elements, each with `message-content
  .markdown`, whose `aria-busy` turns false once written; meanwhile the
  button is "Stop response", and a written answer shows "Copy", "Redo" and
  "Show more options". The prompt's own button is "Copy prompt".
- **Copy** writes a ClipboardItem with text/html and text/plain; copy.js
  catches it for that one press, as ChatGPT's does.
- **Temporary chat** is the button "Temporary chat". The address stays
  `/app`; an empty Temporary chat greets with "Just stopping by? Temporary
  chats ... don't appear in recent chats", and once a prompt is sent a
  "Temporary chat" label stands in the header. A kept chat moves to
  `/app/<id>`.
- **Signed in** is the avatar's label, "Google Account: Name (email)"; the
  tool keeps the name only.

## Claude.

**Like ChatGPT.** claude.ai, found live on 2026-10-25 (lang en-US, the
account sease, Free):
- **The prompt box** is a ProseMirror (tiptap) editor, data-testid
  `chat-input` ("Write your prompt to Claude"); insertText reaches it, and
  "Send message" (`chat-input-send`) sends.
- **A background tab** is enough: the prompt went and the answer was written
  in a hidden page.
- **Answers** are `[data-testid=assistant-message]` with `data-is-streaming`
  true while Claude writes, then false; the text is in `.standard-markdown`.
  A written answer has "Copy", "Retry" and "Edit"; the prompt's copy button
  is `user-message-copy`, outside the answer.
- **Copy** writes a ClipboardItem with text/plain (markdown) and text/html;
  copy.js catches it for that one press.
- **Incognito** is the button "Use incognito": the address gains
  `?incognito`, which a sent chat keeps (`/chat/<id>?incognito`), and the
  button becomes "Exit incognito".
- **Signed out:** a lapsed session goes to `/logout?involuntary=1` and on to
  `/login` ("Continue with Google"); the tool says not signed in.
- **The account button** (`user-menu-button`) reads the avatar's initial,
  the name, "·" and the plan.

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
- **2026-10-24, Vidara, on getresolved, in the user's Chrome.**
  - **A first upload, by hand,** with `san_remote exec`. It found what the
    page does after an upload: the Export page, the title from the file
    name, and the row in My videos.
  - **`vidara status`:** "signed in as getresolved, 0.01 GB used" (4 s).
  - **`vidara videos`**, also with `--search`, listed that video.
  - **`vidara download`** saved it as an .mp4 of 16.02 s. The first run
    took 57 s, the second 10 s.
  - **`vidara delete`**, given its link, deleted it (18 s). `videos` then
    listed none.
  - **`e2e.TestVidaraUpload` passed (94 s):**
    1. It made a 12 MB clip and uploaded it as "Uji vidara san_remote
       023019 (hapus saja)" at about 1 MB/s.
    2. It found the clip by its code, deleted it, and checked that it was
       gone.
    3. The cleanup found nothing left.
  - **Five runs in all, four passed.** The second failed after the upload,
    with the error line filtered out of the log. Its cleanup deleted the
    video, and no screenshot was saved. The two runs after the change to the
    waits passed (93 s, 97 s), and My videos was empty at the end.
  - **Threads on `internal/page`:** `threads status` read "signed in as
    @getresolvedid". The browser's Threads session is on another account
    than the day before.

- **2026-10-24, Facebook, on Santoso Heri, in the user's Chrome.** Only
  Only-me posts on the profile, each deleted again (the user's choice:
  "Explore + Only-me test").
  - **`facebook status`** (20 s) listed the profile, two Pages and 26 groups.
  - **A scheduled post,** for 20 Nov 09:00 (45 s): composer, "Hanya saya",
    paste, picker, time list, "Jadwalkan Postingan", then found in
    Terjadwal at that time. `posts` listed it.
  - **Its delete** worked, but the tool said it had failed: the reloaded
    Terjadwal still listed it. `posts` a minute later showed it gone. That
    led to the reloads in `delete`.
  - **A post now** (28 s), found in Diterbitkan as "Hari ini pukul 12.40"
    and deleted from there (28 s).
  - **A scheduled post with an image,** for 21 Nov 10:30 (45 s). Its row in
    Terjadwal had the 128x128 image, and it was deleted with the new reloads.
  - **`e2e.TestFacebookSchedule`.** The first run was cut off when the
    Claude Code session ended. It left its post scheduled for 18 Nov, and
    `posts` and `delete` found and removed it. The second run passed (175 s)
    and left nothing scheduled.
  - **Not run:** a Page post and a group post. The Page test was refused
    twice by Claude Code's permission check (a write to an outside system),
    even after the user said to do it. The user skipped the group test,
    since its members would see the post. Business Suite's media upload is
    not tried yet either.
  - **The Page, later that day.** The user ran the post in their own
    terminal. It stopped after a minute at "waited 1m0s for Meta Business
    Suite": the sign-in check, fixed above. Then every step up to the last
    button was run by hand with the tool's own helpers: the composer for
    Uni_Q, the paste, the schedule switch, 15 November 2026 09:00, and the
    Jadwalkan button. Jadwalkan was not pressed, and the tab was closed.
- **2026-10-24, X, on @archetipe_, in the user's Chrome.**
  - **`x status`** read "signed in as Warga Sipil (@archetipe_)".
  - **`x posts`** found the profile and no posts, which was right. Its reader
    was also run on a public profile (@FiersaBesari) and read text, time
    and link.
  - **The account is suspended.** The first scheduled test post was refused
    with "Your account is suspended and is not permitted to send posts."
    Nothing has been posted or scheduled on X.
  - **`x post`** of a two-post thread with an image on post 2, for 20 Nov
    09:00, went through every step: paste, Add post, paste, attach, the
    schedule dialog, Confirm, and Schedule. X refused it with "Some of your
    media failed to upload." The tool reported that in 27 s and discarded the
    draft. Drafts and Scheduled were empty afterwards.
- **2026-10-24, X, on @getresolvedid (the user switched accounts).**
  - **`x post`** of the same thread was taken: "Your post will be sent on
    Fri, Nov 20, 2026 at 9:00 AM". The tool still called that a refusal,
    which led to reading X's success toast.
  - **By hand:** the Scheduled tab, opening the thread, a move to 21 Nov,
    Edit mode, and its Delete, which removed the thread.
  - **With the tool:** a new thread was scheduled (26 s), listed by `x
    scheduled` (13 s), moved by `x reschedule` to 22 Nov 10:30 (32 s), and
    deleted by `x unschedule` (17 s). The tab was empty afterwards.
  - **`e2e.TestXSchedule` passed (113 s):** schedule for 23 Nov, find,
    move to 24 Nov, delete, and the cleanup found nothing.
  - **`x posts`** read the account's post. **`x delete`** was not run: it
    would delete a real post. Its menu was opened on a real post and closed
    again.

- **2026-10-25, ChatGPT, Medium and TikTok, in the user's Chrome** (the
  user approved test drafts, a test TikTok post, and test prompts in a
  Temporary chat, each deleted again, and stayed off Chrome meanwhile).
  - **ChatGPT** (Get Resolved, Free): `chatgpt status`, and `chatgpt ask` in
    a Temporary chat answered a list and a code block as markdown (19 s). The
    first try returned before the answer began, which led to waiting for the
    answer's text to hold still. `e2e.TestChatGPTAsk` passed (19 s). Through
    a scratch `san_remote api`, ChatGPTService.Ask answered "Merah".
- **2026-10-25, Gemini, in the user's Chrome** (the user approved one test
  prompt in a Temporary chat): the page read in a background tab; "Reply
  with only the word: ok" sent only once the tab came to the front, and
  answered "ok"; the Copy button's text/plain was "ok". The tab was closed.
  Then with the tool (a scratch build): `gemini ask` in a Temporary chat
  answered "Biru", and `gemini status` read the account.
- **2026-10-25, Claude on claude.ai, in the user's Chrome** (the user signed
  in for it and approved short test prompts): one prompt by hand in an
  incognito chat, sent from a background tab, answered "ok"; then with the
  tool, `claude ask` answered a markdown list ("- Jakarta" / "- Surabaya"),
  and `claude status` read "sease (Free)".
  - **Medium** (Getresolvedid): explored by hand first (a draft with an
    image, alt text and caption, checked after a reload; a topic; a schedule
    for 24 Nov; Cancel scheduling; delete). Then with the tool: `draft --at`
    stopped at the date picker, which did not close on Escape that time;
    `schedule` of the draft it left (43 s), `scheduled`, `get`,
    `unschedule` (46 s), `drafts`, `delete` (25 s). After the fix, `draft
    --at` ran whole (90 s), and `e2e.TestMediumDraft` passed (166 s).
    Drafts and Scheduled were empty at the end.
  - **TikTok** (@getresolved.id): explored by hand (upload, caption,
    hashtag, the new pickers, schedule for 3 Nov 19:00 Everyone, delete).
    The first `tiktok post` stopped at the caption, where a Backspace came
    before Draft.js took the selection; the next scheduled it (44 s) and
    `delete` removed it (21 s). `e2e.TestTikTokSchedule` passed (65 s). Posts
    held only the account's own videos at the end.
  - **LinkedIn** (Heri Santoso, in a second Chrome; the first was signed
    out): the first `e2e.TestLinkedInDraft` wrote the draft with a cover and
    two images and checked it after a reload, then stopped at the post's
    text (no text box yet). By hand: the share box, the calendar and the
    time list (whole mouse press). With the tool on a small test draft:
    `schedule` for 24 Nov 09:00 with two lines, `unschedule`, `delete`.
    The next e2e run stopped when Chrome could not be brought to the front,
    and its cleanup's "Delete" press was lost; a third lost 7476 when the
    tower went down. Then `e2e.TestLinkedInDraft` passed (194 s). Drafts
    were empty and the 5 real scheduled articles untouched at the end.
  - **The tower:** sites.go's steps against fake services (sites_test.go);
    not yet run from a workflow.

## Open questions.

- **From another machine** (the Pi, heri-office-2 over VPN): this needs a
  token and TLS. The Origin check only protects against browsers.
- **Trusted input.** Sites that check `isTrusted` would need
  `chrome.debugger` with `Input.dispatchMouseEvent` / `dispatchKeyEvent`,
  per site, with Chrome's debugging bar showing meanwhile.
- **The rest of the chromedp tools.** Medium, LinkedIn, TikTok and ChatGPT
  have services here now (2026-10-25); `san_youtube` has not. Whether the
  chromedp packages go, as `san_threads` did, is the user's call.
- **Frames.** Only a tab's top frame runs code.
- **More of Threads.** The composer's GIF, sticker, poll, quote, location,
  music and reply settings. "Add AI label" stays off for the user's own
  articles (the user, 2026-10-24).
- **One Threads command at a time across processes.** It could become a
  lock file, if the CLI and Claude ever post at the same time. The same
  holds for Vidara.
- **More of Vidara.** Folders, URL upload, clone, thumbnails, subtitles and
  renaming. A download of the uploaded file itself, if Vidara ever offers
  one; the player only streams its own encoding.
- **More of Facebook.** A Page's own posts (Business Suite's Diterbitkan,
  Dijadwalkan and Draf), rescheduling and editing, Reels and Stories.
- **More of X.** A first real run of `delete` on a published post (X's
  question after it is not seen yet), Drafts, polls, and Premium's longer
  posts.
