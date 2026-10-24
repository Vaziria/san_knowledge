# san_remote

Runs JavaScript in your own browser, the one you use every day and are
signed in to. The San Remote extension
([typescripts/san_remote_extension](../../../../typescripts/san_remote_extension))
sits in Chrome or Edge and attaches to `san_remote serve` on this computer.
The CLI and the MCP server send the server JavaScript to run in a tab, plus
tab commands, and the extension runs them and sends back the result.

```
san_remote exec / MCP ──RemoteService──▶ san_remote serve ◀──BrowserService── extension ──▶ tab
                                          127.0.0.1:7476      (Connect RPC)      userScripts.execute
```

Unlike the chromedp tools (`san_tiktok`, `san_medium`, …), there is no
separate Chrome, no debugging port and no `browser_profiles/` profile: it is
your real browser with your real sign-ins.

Why it is built this way is in the design doc,
[docs/external_repo/san_remote.md](../../../../docs/external_repo/san_remote.md).

Module `github.com/wargasipil/san_remote`, Go 1.26.

## Install

You need:
- Go 1.26
- Node 24 with npm
- Chrome or Edge 135 or newer
- buf, `protoc-gen-go`, `protoc-gen-connect-go` and `protoc-gen-es`, only to
  regenerate code from the proto

1. Build the server: `pwsh build.ps1` (or `bash build.sh`) in
   `golang/apps/san_remote`. It vets, tests, and writes `bin/san_remote.exe`
   and `bin/san_remote`.
2. Build the extension: in `typescripts/san_remote_extension`, run
   `npm install`, then `npm run build`. It writes `dist/`.
3. Load it:
   1. Open `chrome://extensions` (Edge: `edge://extensions`).
   2. Turn on **Developer mode**.
   3. Choose **Load unpacked** and pick `typescripts/san_remote_extension/dist`.
4. On the San Remote card, open **Details** and turn on **Allow User
   Scripts**. Without it the extension attaches but cannot run JavaScript,
   and its popup says so.

## Quickstart

```powershell
cd golang/apps/san_remote
bin\san_remote.exe serve            # leave it running in its own terminal
```

The extension attaches within 30s, or at once when you open its popup. The
popup shows **Connected**. Then:

```powershell
bin\san_remote.exe browsers                       # Chrome, attached
bin\san_remote.exe tabs                           # its tabs
bin\san_remote.exe exec 'return document.title'   # the active tab's title
```

To see it end to end, `$env:SAN_REMOTE_E2E=1; go test ./e2e -v` searches
Google for "cute cat" in your browser (see Development).

## CLI

```powershell
bin\san_remote.exe serve                          # the server; 127.0.0.1:7476
bin\san_remote.exe browsers                       # attached browsers
bin\san_remote.exe tabs                           # id, window, active, title, url
bin\san_remote.exe exec 'return document.title'   # in the active tab
bin\san_remote.exe exec --tab 123 --world main --timeout 10s 'return window.ytInitialData?.responseContext'
Get-Content script.js | bin\san_remote.exe exec -
bin\san_remote.exe exec --file script.js
bin\san_remote.exe open https://example.com       # new tab, waits for the load
bin\san_remote.exe open --focus https://example.com   # and brings the window to the front
bin\san_remote.exe open --background https://example.com
bin\san_remote.exe open --new-window https://example.com   # a window of its own, the size of the current one
bin\san_remote.exe navigate --tab 123 https://example.com
bin\san_remote.exe close --tab 123
bin\san_remote.exe focus --tab 123                # make it the active tab, window to the front
bin\san_remote.exe screenshot --tab 123 -o shot.png
bin\san_remote.exe threads ...                    # Threads: see "Threads" below
bin\san_remote.exe vidara ...                     # Vidara: see "Vidara" below
bin\san_remote.exe facebook ...                   # Facebook: see "Facebook" below
bin\san_remote.exe mcp                            # MCP on stdio, for Claude
```

**exec.** The code is the body of an async function, so `return` gives the
result and `await` works. The result is printed as JSON. DOM nodes don't
survive JSON, so return their text or attributes.
- An exception comes back as an error with its stack.
- A syntax error comes back as the `SyntaxError`.
- Without `--tab` the code runs in the active tab of the window you used
  last.

`--world`:
- `isolated` (default): the extension's own world. It sees the page's DOM
  but not its JavaScript globals, and the page cannot see it.
- `main`: the page's own world, with its globals (`window.ytInitialData`,
  the app's state). The page can see what you do there.

Both work on pages with a strict CSP or Trusted Types: the browser injects
the code, not `eval`.

**open, navigate.** These wait until the tab has finished loading.
JavaScript that navigates away loses its result, so navigate with
`navigate`, then `exec`.

A new tab becomes the active tab of its window, but the window stays behind
whatever you are working in. `--focus` brings it to the front, for when you
want to watch. Windows can refuse and only flash its taskbar button.

**A page that is not shown stalls.** Chrome throttles a hidden page hard:
- Animation frames stop, so menus that wait for one never open.
- After about 5 minutes hidden, timers fire at most once a minute.

Reading a page works in a background tab. Anything that clicks through a
site's interface needs its tab in front, which `focus` restores.

**close** always needs `--tab`: it never defaults to the active tab.

## MCP

`.mcp.json` registers `san_remote mcp` as the `san_remote` server. It is a
client of `san_remote serve` like the CLI, so start `serve` first. After
rebuilding, reconnect with `/mcp`.

| Tool | Arguments |
|---|---|
| `remote_browsers` | |
| `remote_tabs` | browser |
| `remote_open` | url, background, focus, browser, timeout_ms |
| `remote_navigate` | url, tab_id, browser, timeout_ms |
| `remote_close` | tab_id, browser |
| `remote_focus` | tab_id, browser |
| `remote_screenshot` | tab_id, browser; returns the image |
| `remote_exec` | code, tab_id, world (`isolated`/`main`), browser, timeout_ms |
| `threads_status`, `threads_scheduled` | browser |
| `threads_post` | file, or text with media; topic, at, now, browser |
| `threads_reschedule` | match, at, to, browser |
| `threads_unschedule` | match, at, browser |
| `threads_posts` | limit, browser |
| `vidara_status` | browser |
| `vidara_upload` | file, title, browser |
| `vidara_videos` | search, limit, browser |
| `vidara_delete` | match, browser |
| `vidara_download` | link, out, browser |
| `facebook_status` | browser |
| `facebook_post` | file, or text with media; to, audience, at, now, browser |
| `facebook_posts` | tab, browser |
| `facebook_delete` | match, tab, browser |

The server's instructions tell Claude it is your real browser, and to ask
you before it submits forms, posts, buys or deletes anything. They also say
to schedule only what you asked for, and never to post now unless you asked.
For Vidara they say to upload only what you asked for, to ask before a
delete, and never to press a video page's "Download file" button, which is
an ad. For Facebook they say to post only what you asked for, where you
asked, and to ask before a delete.

## Threads

`san_remote threads` posts to Threads through your browser's Threads
session, the way you would in the composer:
- It pastes each post, picks the topic from Threads' suggestions, attaches
  the images or videos, and schedules the thread.
- Threads then publishes it by itself, even when this computer is off.
- Scheduled threads live in the composer's Drafts, where they are listed,
  moved and deleted.

Threads' API is not used: it needs a registered Meta app, takes media only
from a public URL, and cannot schedule.

```powershell
bin\san_remote.exe threads status                               # signed in as @archetipe__
bin\san_remote.exe threads post --dry-run thread.md             # the posts and their lengths; opens nothing
bin\san_remote.exe threads post thread.md                       # Posting on Sun 25 Oct at 19:00 GMT+7
bin\san_remote.exe threads post --at "2026-11-22 09:00" --text "Satu post" --media pic.png
bin\san_remote.exe threads scheduled                            # AT, TEXT: soonest first
bin\san_remote.exe threads reschedule --match "Uji jadwal" "2026-11-23 10:00"
bin\san_remote.exe threads unschedule --match "Uji jadwal" --at "2026-11-23 10:00"
bin\san_remote.exe threads posts --limit 10                     # your published posts: time, link, text
```

**The thread file:**

```markdown
---
at: 2026-10-25 19:00
topic: VPN
---

The first post. Line breaks are kept.

![](../assets/diagram.png)

---

The second post, under the first one.
```

- **`at`** is local time, `"2006-01-02 15:04"`, or RFC 3339. It must be a
  whole minute, 15 minutes to 90 days ahead. `--at` overrides it.
- **`topic`** is the first post's topic. It must be one of Threads'
  suggestions for it; `--topic` overrides it.
- **`---`** on a line of its own starts the next post.
- **`![](file)`** puts an image or video in the post it is in, relative to
  the file. Threads takes jpg, png, webp, avif, mp4, mov and webm, but not
  gif.
- **Limits:** 500 characters and 20 media per post.
- **Write it in Indonesian,** like the articles. Thread files sit next to
  their article, in `docs/articles/<topic>/threads/`.

`--now` publishes at once, publicly, instead of scheduling. No AI label is
ever added: the articles are your own ideas.

**Naming a scheduled thread.** `reschedule` and `unschedule` name one by
`--match` (part of its first post's text), `--at` (its time), or both. Those
must name exactly one, or nothing is done. Threads deletes without asking,
so check the name with `scheduled` first.

**While it runs:**
- `post`, `scheduled`, `reschedule` and `unschedule` open a tab of their own
  in your current Chrome window and bring Chrome to the front. Leave it alone
  until the command is done: a click or a key there lands in Threads.
- If you switch away anyway, the tool brings the window back.
- `status` and `posts` only read, in a background tab.
- Each step is printed to stderr as `== …`.
- A failed step stops with a screenshot of the tab in the temp folder: `,
  screenshot C:\…\san_remote-threads-150405.png`.

**How it runs.** The Threads steps run in the process of the CLI or the MCP
server, and drive the browser through `serve`'s ordinary tab and exec
commands. A change to them needs only a rebuild (and `/mcp` for the MCP
server), never a `serve` restart or an extension reload. Two processes do not
wait for each other, so run one Threads command at a time.

## Vidara

`san_remote vidara` works on your Vidara account (vidara.to, a video host
that pays for views) through your browser's Vidara session, the way you
would on its pages:
- **upload** gives the Upload page's uploader the video, as if you chose it
  in **Browse files**. The page sends it in chunks, the tool waits, then
  finds the video in My videos.
- **videos** reads My videos, and **delete** deletes a video there.
- **download** saves a video from its link through Vidara's own player.
- **status** says who is signed in.

Vidara's API (Settings → API) is not used: you chose the web uploader.

```powershell
bin\san_remote.exe vidara status                                # signed in as getresolved, 0.01 GB used
bin\san_remote.exe vidara upload "VPN Sendiri - Bagian 1.mp4"   # https://vidara.to/v/qrQHbMCtwQw5r  VPN Sendiri - Bagian 1  Active
bin\san_remote.exe vidara upload --title "VPN Sendiri, Bagian 1" video.mp4
bin\san_remote.exe vidara videos                                # CODE, TITLE, SIZE, VIEWS, UPLOADED, STATUS
bin\san_remote.exe vidara videos --search VPN --limit 10
bin\san_remote.exe vidara delete qrQHbMCtwQw5r                  # or its link, or its whole title
bin\san_remote.exe vidara download https://vidara.to/v/qrQHbMCtwQw5r
bin\san_remote.exe vidara download --out D:\videos qrQHbMCtwQw5r
```

**upload.**
- **The file:** mp4, mkv, avi, mov, webm, ts, mpg, mpeg, vob, flv, wmv or
  3gp, from 10 MB to 10 GB. These are the Upload page's own limits, so a
  short clip under 10 MB is refused before anything is sent.
- **The title:** Vidara names a video after its file, without the
  extension, and its Upload page has no title box. `--title` hands the page
  the file under another name, as if you had renamed it first. A title
  cannot hold `/`, `\` or line breaks.
- **What you get:** the video's link (`https://vidara.to/v/<code>`), once
  My videos lists it. Its embed link is `/e/<code>`.
- **How it goes:**
  1. The file goes into the tab in 4 MB pieces, about 20 MB/s.
  2. Vidara's own uploader sends it at your upload speed (about 1 MB/s on
     2026-10-24).
  3. Progress is printed every 10%.
- **When it stops:** if Vidara refuses the file, the tool stops with
  Vidara's message. It also stops if the bar does not move for 3 minutes.

**videos** lists what My videos' first page shows (100 rows, by its
"entries" setting), with how many there are in all. `--search` goes
through My videos' search box, which finds titles and codes.

**delete** names the video by its code, its link, or its whole title (case
ignored). That must name exactly one video. Several videos with the same
title fail with a list of their codes. Vidara asks first, and the tool
answers only once the question names that video. Vidara says the delete
cannot be undone.

**download.**
- **The link** can be `https://vidara.to/v/<code>`, `/e/<code>`, on
  vidara.so or a custom domain, or just the code. Any public video works,
  not only yours.
- **How it gets the video:** the video's page embeds Vidara's player from
  another host, which streams HLS. The tool opens that player in the tab and
  fetches the stream's segments there, as the player does. The segments only
  load from the player's host.
- **What you get:** Vidara's own encoding of the video (720p at 3 Mbps for
  a 720p upload), not the file that was uploaded.
- **The file:** ffmpeg puts the segments in an `.mp4` without re-encoding.
  Without ffmpeg on PATH they are kept as `.ts`.
- **Where it goes:**
  - `--out` is a file or a folder.
  - By default the file is named after the title, in the current folder.
  - An existing file is never overwritten.
- **Never press "Download file".** The video page's "Download file" button
  is an ad (it links to another site), and the tool never presses it.
- **A video that is still being encoded** has no stream yet. Download it
  later.

**While it runs:**
- `upload` and `delete` open a tab of their own in your current Chrome
  window and bring Chrome to the front. Leave it alone while they click.
- During the upload itself the tab may go behind other windows, but it must
  stay open.
- `status`, `videos` and `download` read in a background tab.
- Each step is printed to stderr as `== …`.
- A failed step stops with a screenshot in the temp folder:
  `san_remote-vidara-150405.png`.

## Facebook

`san_remote facebook` posts to Facebook through your browser's Facebook
session, the way you would in its composers:
- **post** writes to your profile (with its audience), a Page you manage or a
  group you are in, now or scheduled. Facebook then publishes a scheduled
  post by itself, even when this computer is off.
- **posts** lists your profile's posts in its Content Library, and
  **delete** deletes one there.
- **status** says who is signed in, and lists your Pages and groups.

Facebook's Graph API is not used: it needs a registered Meta app, and it
cannot post to a personal profile at all.

```powershell
bin\san_remote.exe facebook status                              # signed in as Santoso Heri, then Pages and groups
bin\san_remote.exe facebook post --dry-run post.md              # where, when, the audience and the text; opens nothing
bin\san_remote.exe facebook post post.md                        # scheduled: 20 Nov pada 09.00
bin\san_remote.exe facebook post --audience only_me --at "2026-11-20 09:00" --text "Satu post" --media pic.png
bin\san_remote.exe facebook post --to "page: Uni_Q" --at "2026-11-20 09:00" post.md
bin\san_remote.exe facebook post --to "group: FREELANCER INDONESIA" --now post.md
bin\san_remote.exe facebook posts                               # STATUS, WHEN, TEXT: the Terjadwal tab
bin\san_remote.exe facebook posts --tab published               # or draft
bin\san_remote.exe facebook delete --tab scheduled --match "Satu post"
```

**The post file:**

```markdown
---
to: profile
audience: friends
at: 2026-11-20 09:00
---

The post. Line breaks are kept.

![](../assets/diagram.png)
```

- **`to`** is `profile` (the default), `"page: <name>"` or
  `"group: <name or link>"`. A Page or group is named by its name (case
  ignored), its link or its id, as `status` lists them. `--to` overrides it.
- **`audience`** is `public`, `friends` or `only_me`, for the profile only.
  Without it the post gets the audience Facebook picks (your last one).
  `--audience` overrides it.
- **`at`** is local time, `"2006-01-02 15:04"`, or RFC 3339. It must be on a
  5-minute mark, at least 20 minutes ahead, and no later than 29 days ahead,
  because Facebook's pickers offer nothing else. `--at` overrides it.
- **`![](file)`** attaches an image or video, relative to the file: jpg,
  png, gif, webp, heic, mp4, m4v, mov, mkv or webm.
- A post needs text, media or both. **Write it in Indonesian,** like the
  articles.

`--now` posts at once instead of scheduling. A group post can only go now:
Facebook lets only a group's admins schedule.

**Where each kind goes:**
- **Profile:** the home page's composer. The audience is chosen in "Pilih
  audiens". A time is set in "Opsi penjadwalan" with its date picker and
  time list, and the post is sent with "Jadwalkan Postingan" or "Posting". A
  scheduled post is then looked up in the Content Library, to check its
  time.
- **Page:** Meta Business Suite's composer for that Page. Posting from the
  Page itself would switch your whole Facebook session to the Page ("Beralih
  profil"). The composer is checked to name that Page before anything is
  written, and the post is sent with "Jadwalkan" or "Terbitkan".
- **Group:** the group's composer ("Tulis sesuatu..."), sent with "Posting".
  A group that holds posts for approval shows yours only once an admin
  accepts it.

**posts and delete** work on your profile's Content Library (your profile,
then "Kelola postingan"), on its Diterbitkan, Terjadwal or Draf tab
(`--tab published`, `scheduled` or `draft`). `delete` names the post by part
of its text (`--match`), which must name exactly one post in that tab.
Facebook asks "Hapus postingan secara permanen?" and the tool answers Hapus,
then reloads the tab until the post is gone. It cannot be undone. A Page's
posts are not listed or deleted here; use Business Suite.

**While it runs:**
- `post` and `delete` open a tab of their own in your current Chrome window
  and bring Chrome to the front. Leave it alone until the command is done:
  Facebook's date picker and time list only work in a shown page, and a
  click or a key there lands in Facebook.
- `status` and `posts` read in a background tab.
- Each step is printed to stderr as `== …`.
- A failed step stops with a screenshot in the temp folder:
  `san_remote-facebook-150405.png`.

## Debug panel

The extension keeps a log of every command the server sends and the reply
it gives. Open it with **Open debug panel** in the popup, or from Chrome's
side panel menu (San Remote). It stays to the right of the page while you
browse.

Each entry shows:
- the kind (`exec`, `openTab`, `navigate`, `closeTab`, `listTabs`)
- running, ok or error, and how long it took
- the time and a one-line summary: tab, world, url
- the code, or the command as JSON
- the result as JSON, or the error with its stack

New entries arrive live at the top. The header shows the server and the
time of its last ping, which arrives every 20s while connected. **Clear**
empties the log.

The log keeps the last 200 commands, with each code or result cut at 20,000
characters, in `chrome.storage.session`. That storage survives the service
worker stopping and is emptied when the browser closes. Pings are not
logged.

## Configuration

| Where | Setting | Default |
|---|---|---|
| `serve --listen` | address to listen on, loopback only | `127.0.0.1:7476` |
| `serve --extension-id` | extension ids allowed to attach (repeatable) | `kmlmiiljeiakjbilabmpdiegmoagkmkg` |
| `serve --log-level` | debug, info, warn, error | `info` |
| `--server`, `$SAN_REMOTE_SERVER` | where the CLI and MCP find `serve` | `http://127.0.0.1:7476` |
| `--browser`, `$SAN_REMOTE_BROWSER` | browser id or name, when several are attached | |
| `--timeout` (exec, open, navigate) | how long to wait for the browser | `30s` |
| extension popup: Server | where the extension finds `serve` | `http://127.0.0.1:7476` |
| extension popup: Browser name | name shown in `browsers`, used by `--browser` | `Chrome` or `Edge` |

Changing the server or the name in the popup reconnects at once.

## Security

Anything that can send commands can act as you on every site you are signed
in to, so the server is kept to this computer:

- `serve` refuses a non-loopback `--listen` address.
- Requests must have a loopback `Host`, against DNS rebinding.
- Only the San Remote extension may attach. Its id is fixed by the `key` in
  its `manifest.json` and checked against the `Origin` header. Other
  extensions and web pages can neither attach nor read the commands.
- The CLI and MCP side refuses any request with an `Origin`, so web pages
  cannot send commands.

There is no token, because a local process running as you could read the
browser's cookies anyway. Reaching the server from another machine would
need a token and TLS first.

## Troubleshooting

| What you see | What to do |
|---|---|
| `san_remote is not running at http://127.0.0.1:7476` | Start `san_remote serve`. |
| `no browser is attached` | Open the extension's popup. It retries at once and shows why it can't connect: a wrong server URL, or `serve` not running. |
| popup: **Allow User Scripts is off** | Use its button to open the details page and turn the toggle on. |
| `several browsers are attached, pick one` | Add `--browser <name or id>`; `san_remote browsers` lists them. |
| `Frame with ID 0 is showing error page`, or cannot access a page | Chrome won't let extensions run code on its error pages, `chrome://` pages or the Web Store. Load a normal page first. |
| `the code did not finish in 30000ms` | Raise `--timeout`. The code may still be running in the page. |
| `the page did not finish loading` | The page kept loading past the timeout. Raise `--timeout`, or `exec` anyway: the tab is there. |
| it ran, but nothing showed up | The tab opened in a window behind your other apps. Use `open --focus`, or look at the debug panel. |
| the extension still acts the old way after `npm run build` | Press reload on its card in `chrome://extensions`. |
| the CLI or MCP still acts the old way after `build.ps1` | A running `serve` or `mcp` keeps the old binary; restart it (`/mcp` for MCP). Windows won't overwrite a running exe, so move it to `bin/san_remote.exe~` first. |
| threads: `waited … for …`, with a screenshot | A Threads step did not see what it expected. Look at the screenshot: a dialog in the way, a change in Threads' page, or the tab hidden. |
| threads: `the arrow keys never reached the suggestion` | Threads does not suggest that topic. The error lists what it does suggest; pick one of those. |
| threads: `the browser is not signed in to Threads` | Sign in to Threads (with Instagram) in that browser. |
| threads: `several scheduled threads match` | Make `--match` longer, or add `--at`. |
| vidara: `the browser is not signed in to Vidara` | Sign in to vidara.to in that browser. |
| vidara: `Vidara takes 10 MB to 10 GB` | The Upload page's own limit. A short clip has to be longer, or encoded at a higher bitrate. |
| vidara: `Vidara refused the upload: …` | Vidara's own message, with its reference code. Usually storage, or a file it cannot read. |
| vidara: `the upload stalled at …%` | The connection stopped. Run the same upload again: Vidara's uploader remembers the file by its name, size and time, and continues from where it stopped. |
| vidara: `several videos match the title` | Name the video by one of the codes listed. |
| vidara download: `waited … for the player's stream` | The video may still be encoding, or the player changed. Try later, and look at the screenshot. |
| facebook: `the browser is not signed in to Facebook` | Sign in to facebook.com in that browser. |
| facebook: `Facebook's interface is not in Indonesian` | The tool finds Facebook's controls by their Indonesian names. Set Facebook's language to Bahasa Indonesia. |
| facebook: `nothing matches: no Page "…" (yours: …)` | The error lists your Pages or groups, as `status` finds them; name one of those. |
| facebook: `the post is still in … after Hapus` | Facebook was slow to drop it. Run `posts` again: it is usually gone a minute later. |
| `aborted: No current window` | Chrome is running with no window open. Open a Chrome window. |
| vidara delete waits, then `the code did not finish` | With two-factor login on, Vidara asks for a code before a delete, in a prompt the tool cannot answer. Delete it on the page. |

## Development

```powershell
pwsh build.ps1                                  # vet, test, build both binaries
cd ../../../typescripts/san_remote_extension
npm run build                                   # typecheck + build dist/
npm run dev                                     # rebuild on change; reload the extension
npm run gen                                     # buf generate: Go in gen/, TS in src/gen/
```

After a proto change, run `npm run gen`, rebuild both sides, and reload the
extension. The extension talks binary protobuf, which skips fields it
doesn't know, so a newer server keeps working with an extension that has
not been reloaded yet.

**Tests.**
- `go test ./...` runs the hub, the CLI and the MCP server against fake
  browsers (`internal/hubtest`) that attach over Connect as the extension
  does.
- `e2e/` drives your real browser through a running `serve`. It is skipped
  unless `SAN_REMOTE_E2E=1`:
  ```powershell
  $env:SAN_REMOTE_E2E=1; go test ./e2e -v
  ```
  `TestGoogleSearch` opens google.com with the window brought to the front.
  It types "cute cat" into the search box key by key, submits it, checks the
  results page's query and titles, and leaves the tab open.
- `TestThreadsSchedule` needs `SAN_REMOTE_E2E_THREADS=1` too, since it
  touches your Threads account:
  1. It schedules a two-post test thread 30 days ahead, with a topic and an
     image.
  2. It finds the thread among the scheduled ones, moves it a day later, and
     deletes it.

  Its cleanup deletes the test thread even when a step fails. Nothing is ever
  posted now.
- `TestVidaraUpload` needs `SAN_REMOTE_E2E_VIDARA=1` too, and ffmpeg. It
  makes a 16 s test clip of about 12 MB, uploads it under a test title,
  finds it in My videos, deletes it, and checks that it is gone. Its cleanup
  deletes the test video even when a step fails.
- `TestVidaraDownload` downloads the video `SAN_REMOTE_E2E_VIDARA_LINK`
  names into a temp folder.
- `TestFacebookSchedule` needs `SAN_REMOTE_E2E_FACEBOOK=1` too. It schedules
  an Only-me test post with an image 25 days ahead on your profile, finds it
  in the Content Library's Terjadwal, deletes it, and checks that it is
  gone. Its cleanup deletes the test post even when a step fails. Nothing is
  posted now.

**A throwaway browser.** To test the extension without your own browser,
start a headless Chrome on a temp profile with `--remote-debugging-pipe
--enable-unsafe-extension-debugging`. Then:
1. Load `dist/` with the CDP command `Extensions.loadUnpacked`.
2. On `chrome://extensions`, turn on user scripts with
   `chrome.developerPrivate.updateExtensionConfiguration({extensionId,
   userScriptsAccess: true})`.

Give it its own server (`serve --listen 127.0.0.1:7499`, with the
extension's default server changed to match in a copy of `dist/`).
Otherwise it attaches to your `serve` as a second browser.

The layout of the code is in the design doc.

## Limitations

- Clicks and key events made from JavaScript (`el.click()`,
  `dispatchEvent`) have `isTrusted: false`. Most sites accept them; some
  upload or anti-bot flows don't.
- Chrome's error pages, `chrome://` pages and the Web Store can't run code.
- Only the top frame of a tab runs code.
- The extension is loaded unpacked. The Web Store does not allow extensions
  that run remote code.
- It is local only.
- **Threads:**
  - Threads' interface must be in English: the tool finds its controls by
    their English names ("New thread", "Schedule...", "Done").
  - A redesign of the composer breaks a step, and the tool stops there with
    a screenshot.
  - Not done: GIFs, stickers, polls, quote posts, attached text, location,
    music, the AI label, reply settings, and editing a scheduled thread's
    text.
- **Vidara:**
  - Its interface must be in English, as for Threads.
  - One video per upload. Not done: folders, URL upload, clone, thumbnails,
    subtitles, intros, and renaming.
  - `videos` reads only My videos' first page.
  - A download is Vidara's stream of the video, not the uploaded file, and
    an encrypted stream is refused.
- **Facebook:**
  - Its interface must be in Indonesian: the tool finds its controls by
    their Indonesian names ("Buat postingan", "Jadwalkan Postingan",
    "Hapus").
  - Scheduling reaches only 29 days ahead, the limit of the profile's date
    picker.
  - Not done: Reels, Stories, feelings, tagging people, location, editing or
    rescheduling a post, and listing or deleting a Page's posts.

## See also

- [docs/external_repo/san_remote.md](../../../../docs/external_repo/san_remote.md):
  the design and the reasons for it
- [typescripts/san_remote_extension/README.md](../../../../typescripts/san_remote_extension/README.md):
  the extension's files
- [docs/external_repo/san_gpt.md](../../../../docs/external_repo/san_gpt.md):
  the CDP approach this is the alternative to
