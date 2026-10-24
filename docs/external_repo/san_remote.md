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
   - All three work in a tab through `internal/page`.

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
  internal/page/   page.go            a site service's tab: call into the site's script, waits,
                                      pace, front, screenshot; the browser through RemoteService
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
  internal/mcp/                       MCP server: remote_*, threads_*, vidara_* and facebook_* tools
  cmd/san_remote/                     urfave/cli v3 tree: serve, browsers, tabs, exec, open,
                                      navigate, close, focus, screenshot, threads, vidara, facebook, mcp
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
- **Business Suite's composer.** The editor is named "Tulis ke dalam kotak
  dialog…" and takes a paste. The switch "Atur tanggal dan waktu" shows a
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
  lock file, if the CLI and Claude ever post at the same time. The same
  holds for Vidara.
- **More of Vidara.** Folders, URL upload, clone, thumbnails, subtitles and
  renaming. A download of the uploaded file itself, if Vidara ever offers
  one; the player only streams its own encoding.
- **More of Facebook.** A Page's own posts (Business Suite's Diterbitkan,
  Dijadwalkan and Draf), rescheduling and editing, Reels and Stories.
