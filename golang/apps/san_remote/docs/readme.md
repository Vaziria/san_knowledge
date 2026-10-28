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
your real browser with your real sign-ins. Medium, LinkedIn, TikTok and
ChatGPT have services of their own here too (2026-10-25), the chromedp
tools' steps on your own browser, and so do Gemini and Claude (claude.ai,
the same day).

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
bin\san_remote.exe x ...                          # X: see "X" below
bin\san_remote.exe medium ...                     # Medium: see "Medium" below
bin\san_remote.exe linkedin ...                   # LinkedIn: see "LinkedIn" below
bin\san_remote.exe tiktok ...                     # TikTok: see "TikTok" below
bin\san_remote.exe chatgpt ...                    # ChatGPT: see "ChatGPT" below
bin\san_remote.exe gemini ...                     # Gemini: see "Gemini" below
bin\san_remote.exe claude ...                     # claude.ai: see "Claude" below
bin\san_remote.exe mcp                            # MCP on stdio, for Claude
bin\san_remote.exe api                            # the site services over Connect (gRPC), on 127.0.0.1:7478
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
| `vidara_upload` | file, title, folder, browser |
| `vidara_videos` | search, limit, browser |
| `vidara_delete` | match, browser |
| `vidara_download` | link, out, browser |
| `facebook_status` | browser |
| `facebook_post` | file, or text with media; to, audience, at, now, browser |
| `facebook_posts` | tab, browser |
| `facebook_delete` | match, tab, browser |
| `x_status` | browser |
| `x_post` | file, or text with media; at, now, browser |
| `x_posts` | limit, browser |
| `x_scheduled` | browser |
| `x_reschedule` | match, at, to, browser |
| `x_unschedule` | match, at, browser |
| `x_delete` | match (link, id or text), browser |
| `x_download` | link (any post's, or its id); out, browser |
| `medium_status`, `medium_drafts`, `medium_scheduled` | browser |
| `medium_draft` | file; at, topics, browser |
| `medium_get`, `medium_unschedule`, `medium_delete` | story (id or link), browser |
| `medium_schedule` | story, at; topics, remove_topics, browser |
| `linkedin_status`, `linkedin_drafts`, `linkedin_scheduled` | browser |
| `linkedin_draft` | file; cover, at, text, browser |
| `linkedin_get`, `linkedin_delete` | story (id or edit link), browser |
| `linkedin_schedule` | story, at; text, browser |
| `linkedin_unschedule` | match (the title); when, browser |
| `tiktok_status`, `tiktok_posts` | browser |
| `tiktok_post` | file, or video with caption; at or now, browser |
| `tiktok_delete` | caption, when, browser |
| `youtube_status`, `youtube_videos` | browser |
| `youtube_upload` | file, or video with title; description, tags, at, short, browser |
| `youtube_private` | video (id or link), browser |
| `youtube_delete` | video (id or link); allow_public, browser |
| `telegram_status` | browser |
| `telegram_create_bot` | name, username; browser |
| `telegram_delete_bot` | username, browser |
| `telegram_create_group` | title, browser |
| `telegram_delete_group` | group (id or title), browser |
| `telegram_add_member`, `telegram_remove_member` | group, user (username), browser |
| `chatgpt_status` | browser |
| `chatgpt_ask` | prompt; keep, chat, wait_seconds, browser |
| `gemini_status` | browser |
| `gemini_ask` | prompt; keep, wait_seconds, browser |
| `claude_status` | browser |
| `claude_ask` | prompt; keep, wait_seconds, browser |

The server's instructions tell Claude it is your real browser, and to ask
you before it submits forms, posts, buys or deletes anything. They also say
to schedule only what you asked for, and never to post now unless you asked.
For Vidara they say to upload only what you asked for, to ask before a
delete, and never to press a video page's "Download file" button, which is
an ad. For Facebook they say to post only what you asked for, where you
asked, and to ask before a delete. For X, as for Threads, they say to
schedule or post only what you asked for, and never now unless you asked.
For Medium, LinkedIn and TikTok they say that nothing is published at once,
to post only what you asked for, and to ask before a delete. For Telegram
they say the bot token is a secret not to repeat in chat, and to ask before
a delete, a removal or adding someone to a group. For ChatGPT
and Gemini they say a new chat is a Temporary chat unless `keep` is set, and
for Claude an incognito chat.

## API

`san_remote api` serves the Threads, X, Facebook and Vidara services over
Connect, which also speaks gRPC, for other programs. The first one is the
tower: its San Remote connection points at it, and its workflow steps post
and read through it. Like the MCP server, it is a client of `serve`, so
start `serve` first.

```powershell
bin\san_remote.exe api                            # 127.0.0.1:7478, serve at --server
bin\san_remote.exe api --listen 127.0.0.1:7479 --log-level debug   # each site step in the log
```

| Service | RPCs |
|---|---|
| `san.remote.v1.RemoteService` | everything `serve` has, passed through to it |
| `san.remote.threads.v1.ThreadsService` | GetStatus, SchedulePost (posts with a topic on the first, or a markdown file or text), ListScheduled, ListPosts |
| `san.remote.x.v1.XService` | GetStatus, SchedulePost (a thread, each post with its images or video by absolute path, or a markdown file or text), ListScheduled, ListPosts, DownloadPost (any post's media and text, into an absolute folder) |
| `san.remote.facebook.v1.FacebookService` | GetStatus, SchedulePost (to the profile or a Page, with an audience; text, or a markdown file or text), ListPosts (a Content Library tab) |
| `san.remote.vidara.v1.VidaraService` | GetStatus, UploadVideo, ListVideos, DownloadVideo (by link or code, to an absolute folder or file) |
| `san.remote.medium.v1.MediumService` | GetStatus, CreateDraft (an absolute article file, or its markdown; with `at`, scheduled too, with topics), ScheduleDraft, ListDrafts, ListScheduled |
| `san.remote.linkedin.v1.LinkedInService` | GetStatus, CreateDraft (an absolute article file, or its markdown, and cover; with `at`, scheduled too, shared with `text`), ScheduleDraft, ListDrafts, ListScheduled |
| `san.remote.tiktok.v1.TikTokService` | GetStatus, SchedulePost (an absolute video file, for Everyone; the caption, or a markdown file or text), ListPosts |
| `san.remote.youtube.v1.YouTubeService` | GetStatus, UploadVideo (an absolute video file; scheduled, or saved as Private; `short`), ListVideos, DeleteVideo (never a Public one unless `allow_public`), MakePrivate |
| `san.remote.telegram.v1.TelegramService` | GetStatus, CreateBot (through @BotFather; the bot with its token), DeleteBot, CreateGroup (the group's id), DeleteGroup, AddMember, RemoveMember (a group by id or title, a user by username) |
| `san.remote.chatgpt.v1.ChatGPTService` | GetStatus, Ask (in a Temporary chat unless `keep`; the answer as markdown; `stay` leaves the tab open on the chat; `idle_seconds` keeps it open for the chat's next ask, which goes on in it, and closes it after that long without one) |
| `san.remote.gemini.v1.GeminiService` | GetStatus, Ask (in a Temporary chat unless `keep`; brings Chrome to the front; `stay` leaves the tab open on the chat; `idle_seconds` keeps it open for the chat's next ask, which goes on in it, and closes it after that long without one) |
| `san.remote.claude.v1.ClaudeService` | GetStatus, Ask (in an incognito chat unless `keep`; the answer as markdown; `stay` leaves the tab open on the chat; `idle_seconds` keeps it open for the chat's next ask, which goes on in it, and closes it after that long without one) |

- **Only scheduling.** A post's `at` is required, and there is no "now"
  (the user's choice, 2026-10-24): the site publishes it at its time, and
  until then it can be deleted from the site's scheduled list. So a group
  post on Facebook, which cannot be scheduled, is refused. A Medium or
  LinkedIn article without `at` stays a draft, and a YouTube video without
  it is saved as Private (2026-10-27: the tower's YouTube Upload sends none
  when At is empty); nobody else sees either.
- **Files are absolute paths** (articles, covers, videos): a relative one
  would be inside wherever the api was started.
- **Markdown posts** (the user, 2026-10-25, for the tower's post steps'
  Format Markdown). SchedulePost of Threads, X, Facebook and TikTok takes,
  instead of plain text (`posts`, `text`, `caption`), `file`, a post file as
  the CLI reads it (images relative to it), or `markdown`, the same as text
  (images and a video by absolute paths). What the request gives wins over
  the frontmatter: `at`, Threads' `topic`, Facebook's `to` and `audience`,
  TikTok's `video`; `at` may be left out for the file's. CreateDraft of
  Medium and LinkedIn takes `markdown` instead of `file` the same way.
- **Checked first.** What the site would refuse (no post, a time too soon or
  too far, a missing video file, a download's bad link or relative path) is
  refused before a tab opens, as InvalidArgument. A browser that is not signed in, or not in the language
  the steps expect, is FailedPrecondition. `serve` not running is
  Unavailable: "san_remote is not running at …".
- **One address.** RemoteService goes through to `serve`, so a program needs
  only the api's address; the tower's connection Test lists the attached
  browsers through it.
- **How it runs.** Like the CLI and the MCP server, the site steps run in
  the api's own process and reach the browser through `serve`. A fix to a
  site needs only an api restart. A post works in a tab brought to the
  front for a minute or two, and only one runs at a time per process, as
  with the CLI.
- **Security.** It only listens on loopback and takes no calls from web
  pages (`hub.Guard`), like `serve`: whatever reaches it can post as you.

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
- **Markdown** in a post is written as plain text, since a post has none:
  `**bold**` is bold, `# Title` is Title, `[text](url)` is "text (url)",
  `` `code` `` is code; lists keep their markers, line breaks stay, and
  `#hashtags` are not headings. Text without markup is left as it is
  (2026-10-25), so an AI's markdown answer can be posted as it is.
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
bin\san_remote.exe vidara upload --folder Promo video.mp4          # into the folder Promo (made if it is not there)
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
- **The folder:** `--folder` puts the video in one of your folders, picked
  in the Upload page's **Folder** list before the file is chosen, as you
  would. Give its name, or its path for a folder in a folder
  (`Promo/2026`); case does not matter when nothing matches exactly.
  - A top-level folder that is not there is made, as **-Create new-**
    makes one: the tool answers its "Enter folder name" prompt.
  - A folder in a folder must be there already (make it in My videos with
    **New folder**), since -Create new- makes only top-level folders.
  - Without `--folder` the video goes in the root (/), as before.
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
  - `--out` is a file or a folder. A folder that is not there yet is one
    when the path ends in `\` or `/`.
  - By default the file is named after the title, in the current folder.
  - Missing folders are made.
  - An existing file is never overwritten.
- **Never press "Download file".** The video page's "Download file" button
  is an ad (it links to another site), and the tool never presses it.
- **A video that is still being encoded** has no stream yet. Download it
  later.

**While it runs:**
- `upload` and `delete` open a tab of their own in your current Chrome
  window and ask for Chrome's window. Leave the tab alone while they click.
- `upload` never waits for the window (2026-10-25): putting a big file in
  the page takes a while, and by then you are in another window, which
  Windows keeps in front. Vidara's uploader sends the file the same way
  behind other windows, so you can keep working; the tab must stay open.
  The check in My videos afterwards also runs behind other windows.
- `delete` waits for the window before it searches and presses Delete, as
  the other sites' posting steps do (see "Chrome's taskbar button flashes"
  under Troubleshooting).
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
- **Markdown** is written as plain text, as in a thread file.
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

## X

`san_remote x` posts to X through your browser's X session, the way you
would in its composer:
- **post** pastes each post of a thread, attaches its images or videos,
  and schedules the thread, or posts it now. X then publishes a scheduled
  thread by itself.
- **scheduled**, **reschedule** and **unschedule** list, move and delete
  scheduled threads, in the composer's Drafts → Scheduled.
- **posts** lists your latest published posts from your profile, and
  **delete** deletes one.
- **download** reads anyone's post from its link and saves its images and
  videos, with its caption, hashtags and, for an X Article, the article
  as markdown (below).
- **status** says who is signed in.

X's API is not used: posting through it needs a paid developer plan.

```powershell
bin\san_remote.exe x status                                # signed in as Warga Sipil (@archetipe_)
bin\san_remote.exe x post --dry-run thread.md              # each post with its length in X's count; opens nothing
bin\san_remote.exe x post thread.md                        # scheduled: Will send on Fri, Nov 20, 2026 at 9:00 AM
bin\san_remote.exe x post --at "2026-11-20 09:00" --text "Satu post" --media pic.png
bin\san_remote.exe x scheduled                             # AT, TEXT, MORE ("1 more post")
bin\san_remote.exe x reschedule --match "Uji jadwal" "2026-11-22 10:30"
bin\san_remote.exe x unschedule --match "Uji jadwal" --at "2026-11-22 10:30"
bin\san_remote.exe x posts --limit 10                      # AT, LINK, TEXT: newest first
bin\san_remote.exe x delete https://x.com/getresolvedid/status/2096950313257484398
bin\san_remote.exe x download --out D:\x --md https://x.com/IntEngineering/status/2113210739255070922
```

**The thread file** is the Threads format without the topic:

```markdown
---
at: 2026-11-20 09:00
---

The first post. Line breaks are kept.

![](../assets/diagram.png)

---

The second post, under the first one.
```

- **`at`** is local time, `"2006-01-02 15:04"`, or RFC 3339, on a whole
  minute, 10 minutes to 18 months ahead. `--at` overrides it.
- **`---`** on a line of its own starts the next post.
- **`![](file)`** puts an image or video in the post it is in: jpg, png,
  webp, gif, mp4 or mov. A post holds up to 4 images, or one video or GIF.
- **Markdown** is written as plain text, as in a Threads file; a link's
  address stays in the post, in brackets after its text.
- **Length:** 280 per post in X's count, for an account without Premium. A
  link counts 23 whatever its length; CJK characters and emoji count 2.
  `--dry-run` shows each post's count.

`--now` posts at once, publicly, instead of scheduling.

**When X says no**, the tool stops with X's own words and discards the
draft, so nothing is left in Drafts. For example `x: X refused the post:
Your account is suspended and is not permitted to send posts.`, or `Some
of your media failed to upload.`

**Scheduled threads** are named by `--match` (part of the first post's
text), `--at` (their time), or both, which must name exactly one.
- `reschedule` opens the thread from the Scheduled tab, sets the new time,
  and presses Schedule. X keeps one thread at the new time, and the tool
  checks the tab lists it there and not at the old time.
- `unschedule` ticks that one thread in the tab's Edit mode and presses
  Delete. X asks "Discard unsent posts?" and the tool answers Delete. It
  cannot be undone.

**Downloading a post** (`x download LINK`, 2026-10-25) opens the link in a
background tab and reads the post from X's own app, the data its page is
drawn from, so nothing is clicked and nothing changes on the account.
- **Files** go to `--out` (default: the current folder), made when
  missing: `<id>-1.mp4`, `<id>-2.jpg`, in the post's order. A photo is its
  original size, a video or GIF its best mp4 (the 1080p one where there is
  one). They come from X's media servers, which need no sign-in. A file of
  the same name is replaced. A connection that breaks while a file comes
  (2026-10-27: several downloads at once share one connection to
  video.twimg.com, and a reset killed them all) is opened again, up to 4
  tries 2, 4 and 8 s apart, going on from the byte it broke at; a file X
  will not give (404) is not asked again.
- **Caption** is the post's text: a long post's whole text, links written
  out, the link to its own media dropped. **Tags** are its hashtags (X's
  own, else the `#words` in the caption).
- **An X Article** (a post that is only a link to x.com/i/article/…) is
  drawn whole on its post's page. Its cover is saved as `<id>-cover.jpg`
  and its images as `<id>-article-1.jpg`; its text becomes markdown: `#`
  the title, the cover, then the body (headings one level down, bold,
  italic, links, lists, quotes, code, its tables as markdown tables, its
  images pointing at the saved files by absolute path).
- `--md` also writes that text (the article, or the caption) to `<id>.md`.

Checked on 2026-10-25 with a video post (one 1080×1920 mp4, 113 s, and its
long caption) and an X Article post (its cover, 1983×793, and its markdown
with two tables).

**delete** names a published post by its link, its id, or part of its
text, which must name exactly one of your latest 50 posts. From your
profile it opens the post's "…" menu and presses Delete, and answers Delete
when X asks. It cannot be undone.

**While it runs:**
- `post`, `scheduled`, `reschedule`, `unschedule` and `delete` open a tab
  of their own in your current Chrome window and bring Chrome to the front:
  X draws its composer, its dialogs and its menus only in a shown page.
  Leave Chrome alone until the command is done. If you work in another
  window, Windows may keep Chrome behind it, and the step waits.
- `status` and `posts` read in a background tab.
- Each step is printed to stderr as `== …`.
- A failed step stops with a screenshot in the temp folder:
  `san_remote-x-150405.png`.

## Medium

`san_remote medium` writes your markdown articles into Medium through your
browser's Medium session, the way you would in Medium's editor. It is
[san_medium](../../../packages/san_medium/README.md) on your own browser
instead of a Chrome of its own:
- **draft** opens a new story with Write, types the title, pastes the text,
  and puts each image in its place with the editor's + menu, with its alt
  text and caption. Medium saves the draft itself; the tool then checks it
  against the article. With `--at` it then schedules it, with `--topic`s.
- **schedule** sets a draft's time on Medium's publish page (topics, "Schedule
  for later", the date picker). Medium publishes it by itself, within 5
  minutes of that time. **unschedule** presses "Cancel scheduling"; the story
  stays a draft.
- **drafts**, **scheduled** and **get** read, **delete** deletes a draft.
- **status** says who is signed in.

Nothing is ever published at once. Medium issues no API tokens any more.

```powershell
bin\san_remote.exe medium status                       # signed in as Getresolvedid (@getresolvedid)
bin\san_remote.exe medium draft --dry-run article.md   # how the article will be cut; opens nothing
bin\san_remote.exe medium draft article.md             # https://medium.com/p/f08623feda65/edit
bin\san_remote.exe medium draft --at "2026-11-24 09:00" --topic Programming article.md
bin\san_remote.exe medium schedule --at "2026-11-24 09:00" --topic Golang f08623feda65
bin\san_remote.exe medium scheduled                    # ID, AT, TITLE
bin\san_remote.exe medium unschedule f08623feda65
bin\san_remote.exe medium drafts                       # ID, UPDATED, WORDS, TITLE
bin\san_remote.exe medium get f08623feda65             # status, title, subtitle, images, then the text
bin\san_remote.exe medium delete f08623feda65
```

**The article** is read as the
[article guideline](../../../../docs/articles/guideline.md) writes it:
- the first `# ` heading is the title, and an italic paragraph right under it
  the subtitle
- `##` becomes Medium's large heading, `###` and deeper the small one
- text, lists, quotes, links, inline code and code blocks are pasted
- an image on a paragraph of its own (`![alt](assets/x.png)`) is uploaded,
  with its alt text; an italic paragraph right under it is its caption
- frontmatter is skipped

A missing image file or a remote image (`https://…`) is refused before a tab
opens; warnings name links to local files and images inside a line of text.

A story is named by its id or any link to it (`/p/<id>/edit`, or a story
link ending in `-<id>`). **`--at`** is local time, on a whole minute, at least
10 minutes ahead; up to 5 topics, each one Medium suggests. `schedule
--remove-topic` takes a topic off first.

**While it runs:**
- `draft`, `schedule`, `unschedule` and `delete` open a tab of their own in
  your current Chrome window and bring Chrome to the front: the editor's +
  button glides into place only in a shown page. Leave it alone until the
  command is done.
- `status`, `drafts`, `scheduled` and `get` read in a background tab.
- Every command starts at Medium's home page and clicks through (Write,
  Stories, a row's menu), as a person does; only the Scheduled tab's table
  and `get` open a link of their own.
- A failed step stops with a screenshot: `san_remote-medium-150405.png`.

## LinkedIn

`san_remote linkedin` writes your markdown articles into LinkedIn articles
through your browser's LinkedIn session, the way you would in the article
editor. It is [san_linkedin](../../../packages/san_linkedin/README.md) on your
own browser:
- **draft** opens "Write article" from the feed, types the title, uploads the
  cover (`--cover`), pastes the text, and puts each image in its place with
  the editor's image button, with its alt text and caption. With `--at` it
  then schedules it, shared with `--text`.
- **schedule** goes through the editor's Next: the post's text, the clock,
  the date and time. LinkedIn publishes it by itself. **unschedule** moves a
  scheduled article back to the drafts, named by its title (the Scheduled tab
  shows no ids).
- **drafts**, **scheduled** and **get** read the editor's Articles dialog,
  **delete** deletes a draft.
- **status** says who is signed in.

```powershell
bin\san_remote.exe linkedin status
bin\san_remote.exe linkedin draft --cover assets\cover.png article.md
bin\san_remote.exe linkedin draft --at "2026-11-20 09:00" --text "Bagian 1 dari seri VPN." article.md
bin\san_remote.exe linkedin schedule --at "2026-11-20 09:00" 7515804761766535168
bin\san_remote.exe linkedin scheduled                  # WHEN, TITLE
bin\san_remote.exe linkedin unschedule "Judul artikel" # --when "Fri, Nov 20 at 9:00 AM" picks one of two
bin\san_remote.exe linkedin drafts                     # ID, MODIFIED, TITLE
bin\san_remote.exe linkedin get 7515804761766535168
bin\san_remote.exe linkedin delete 7515804761766535168
```

The article is read as for Medium; LinkedIn has no subtitle field, so the
subtitle opens the body as an italic paragraph. LinkedIn keeps 150 characters
of a title, 1,000 of alt text and 250 of a caption. **`--at`** is on a quarter
hour, at least 15 minutes ahead. The time LinkedIn states, and its zone
("based on your location"), are read back before Schedule is pressed.

Every command but `status` brings Chrome to the front: the Articles dialog
is reached through the editor's Manage menu.

Its interface must be in English. Every command but `get` was run live on
2026-10-25 (status, draft with a cover and images, schedule, scheduled,
unschedule, drafts, delete), and `TestLinkedInDraft` passes. Keep Chrome on screen while it
works (see "Chrome to the front" under Troubleshooting).

## TikTok

`san_remote tiktok` posts your videos through TikTok Studio, in your
browser's TikTok session. It is
[san_tiktok](../../../packages/san_tiktok/README.md) on your own browser:
- **post** gives Studio's upload page the video, writes the caption, picking
  each `#hashtag` from TikTok's suggestions, and schedules it for Everyone,
  or with `--now` posts it at once for Only you. A video is never public at
  once.
- **posts** lists Studio's Posts, and **delete** deletes one there.
- **status** says who is signed in.

```powershell
bin\san_remote.exe tiktok status                       # signed in as @getresolved.id
bin\san_remote.exe tiktok post --dry-run post.md       # the video, when, and the caption; opens nothing
bin\san_remote.exe tiktok post post.md                 # 00:06 Uji … Nov 3, 7:00 PM Everyone 0 0 0
bin\san_remote.exe tiktok post --video clip.mp4 --caption "Uji #gamedev" --at "2026-11-03 19:00"
bin\san_remote.exe tiktok posts                        # WHEN, LENGTH, PRIVACY, CAPTION
bin\san_remote.exe tiktok delete --caption "Uji" --when "Nov 3, 7:00 PM"
```

**The post file** is san_tiktok's: frontmatter `video:` (relative to the
file) and `at:`, then the caption.

```markdown
---
video: renders/clip.mp4
at: 2026-11-03 19:00
---

Baris pertama.
Baris kedua #gamedev
```

- **`at`** is local time on a 5-minute step, 15 minutes to 30 days ahead:
  Studio's calendar offers the next 30 days (10 when san_tiktok was built).
- **Who can see it:** Everyone for a scheduled post; Studio refuses to
  schedule a private one ("Private videos can't be scheduled"). `--now` is
  Only you.
- **Hashtags** TikTok does not suggest stay text.
- **Markdown** in the caption is written as plain text, as in a Threads
  file; an image is dropped.
- The time zone box must show this computer's offset ("(GMT+7) Local Time").
- Studio's checks (content check lite, about 10 minutes) are waited for
  before Schedule, since pressing it earlier makes Studio ask "Continue to
  post?".

`post` and `delete` bring Chrome to the front; `status` and `posts` read in
a background tab. A failed step stops with a screenshot:
`san_remote-tiktok-150405.png`.

## YouTube

`san_remote youtube` uploads your videos, and Shorts, through YouTube
Studio, in your browser's YouTube session (2026-10-26). It is the Studio
uploader of [san_youtube](../../../packages/san_youtube/README.md) on your
own browser: its flow and selectors (found 2026-10-13 with chromedp), with
the clicks and typing done from the page (`youtube.js`):
- **upload** goes to youtube.com, presses Create, then Upload video, gives
  the dialog the video, types the title and description, answers "not made
  for kids", adds the tags, presses Next to Visibility, and schedules it
  (`--at`) or saves it as Private. A video is never public at once. It
  waits for the file to go up, then finds the video in Studio's content
  list.
- **videos** lists Studio's content list: the Videos tab, then Shorts.
- **private** makes a video Private from the content list (its visibility,
  Private, Save); a scheduled one loses its schedule.
- **delete** deletes a video for good from the content list (the row's
  Options, Delete forever, "I understand…", Delete forever). A Public video
  is refused unless `--allow-public`. It cannot be undone.
- **status** says who is signed in (the account menu's name and @handle).

```powershell
bin\san_remote.exe youtube status                         # signed in as Heri Santoso (@herisantoso)
bin\san_remote.exe youtube upload --dry-run video.md      # the video, kind, when, title, tags; opens nothing
bin\san_remote.exe youtube upload video.md                # https://youtu.be/<id>  Scheduled  Nov 13, 2026 …
bin\san_remote.exe youtube upload --video clip.mp4 --title "Uji" --tags "a, b" --short --at "2026-11-03 19:00"
bin\san_remote.exe youtube videos                         # KIND, VISIBILITY, DATE, URL, TITLE
bin\san_remote.exe youtube private https://youtu.be/<id>   # makes it Private
bin\san_remote.exe youtube delete <id>                    # deletes it for good (not a Public one)
```

**The post file** is san_youtube's: frontmatter `video:` (relative to the
file), `tags:` (a list, or one line with commas), `at:` and `short:`, then a
`# ` title and the description.

```markdown
---
video: renders/clip.mp4
tags: [raspberry pi, radio 24 jam]
at: 2026-11-03 19:00
short: true
---

# Judul video

Deskripsinya, sebagai teks biasa.
```

- **`at`** is local time on a quarter hour (:00, :15, :30, :45), at least
  15 minutes ahead; without it the video is saved as Private. Studio's
  "Local Time" is the browser's: its offset at that time must be this
  computer's. The day is picked in Studio's calendar (a scrolling list of
  months, `div.calendar-month[aria-label="Nov 2026"]`, days
  `span.calendar-day`) and the time in its list of 96 quarter hours
  (`tp-yt-paper-item` "7:00 PM"): a date or time typed in the box with an
  Enter from the page did not take (2026-10-27). Studio shows the time with
  a narrow no-break space, U+202F ("7:00 PM"), which is read as a space.
- **A Short** is not a setting: YouTube makes a vertical or square video of
  at most 3 minutes a Short by itself. `--short` (or `short: true`) makes
  the upload fail when Studio lists the video under Videos instead; it is
  saved all the same, private or scheduled.
- **Limits** are checked before any browser opens: a title of up to 100
  characters, a description of up to 5000 bytes, tags of up to 500
  characters as Studio counts them, no `<` or `>`.
- **The file** goes to the dialog's file input through a DataTransfer, in 4
  MB pieces, as for TikTok: the dialog's Select files opens a file chooser
  a page cannot answer.
- Studio must be in English: the schedule's date and time are typed as it
  shows them ("Nov 13, 2026", "7:05 PM") and read back.
- Once Studio has made the video, an error names it (its Studio link): it
  is kept as Private, to finish or delete there.

- **private** and **delete** take the video's id or a link (youtu.be,
  youtube.com/shorts, watch?v=, its Studio link). They read the newest page
  of the Videos and the Shorts tabs. A row renders its actions (Details,
  Analytics, Comments, View on YouTube, Options) only once a mouseover
  reaches its inner elements, so the tool sends one to each; Options is the
  `ytcp-icon-button` named Options, and its menu's Delete forever is
  `tp-yt-paper-item[test-id=delete]`. Live-tested on 2026-10-27: a 5-second
  vertical clip uploaded with `--short` was listed under Shorts as Private
  (youtube.com/shorts/…), then deleted (the box "I understand…" ticked,
  Delete forever pressed, the row gone). A draft (an upload never saved)
  asks with "Delete draft video" instead. The visibility choice is
  `ytcp-video-visibility-edit-popup`: a scheduled video's opens on
  Schedule, with "Save or publish" folded (`#first-container-expand-button`)
  and its radio buttons drawn only once it is open; Private, then
  `#save-button`. Live-tested the same night: a 4-minute 1280x720 video
  scheduled for a week ahead (listed under Videos as Scheduled), the
  tower's Last Scheduled Post finding it, then made Private and deleted.

`upload`, `videos`, `private` and `delete` bring Chrome to the front
(each opens the upload dialog and closes it, to reach the content list as a
person does);
`status` reads in a background tab. A failed step stops with a screenshot:
`san_remote-youtube-150405.png`.

## Telegram

`san_remote telegram` works in Telegram Web (web.telegram.org/k) in your
browser, as your own account, the way you would there (2026-10-27: the
tower's Telegram nodes for bots and groups go through San Remote, not a
Telegram API). The steps are in Go (`internal/telegram/service.go`) and the
page knowledge in `telegram.js`, found in your signed-in Telegram Web and in
the client's source ([tweb](https://github.com/morethanwords/tweb)):
- **create-bot** opens the chat with @BotFather (from the chat list, or the
  search), sends `/newbot`, the name and the username, and reads the token
  out of @BotFather's "Done!". A refusal (a username taken or invalid, too
  many bots) fails with what @BotFather said, and `/cancel` ends its
  conversation.
- **delete-bot** sends `/deletebot`, the bot's @username, and the phrase
  @BotFather asks for ("Yes, I am totally sure."). A bot that is not yours
  is refused. It cannot be undone.
- **create-group** presses New Chats (the pencil), New Group, Next past Add
  Members (no members yet), types the name and presses Next; it prints the
  new group's id.
- **delete-group** opens the group, its More menu, Delete Group, ticks
  "Delete for all members" and presses Delete group. When the menu only
  offers Leave Group, or the popup has no "Delete for all members", the
  group is not yours and nothing is done. It cannot be undone.
- **add-member** opens the group's profile (its title bar), Members, the
  Add Members button, searches the username, ticks the user, Next, Add. A
  user already in the group is left as they are. A user whose privacy
  settings keep them out of groups fails with Telegram's message (a toast,
  or its "Invite via Link" offer, which is closed).
- **remove-member** opens the group's profile, finds the member (scrolling
  for a big group's), and right-clicks them: Remove from group.
- **status** says who is signed in.

```powershell
bin\san_remote.exe telegram status                                  # signed in as Heri Santoso (@…)
bin\san_remote.exe telegram create-bot "San Test" san_test_bot      # @san_test_bot  https://t.me/san_test_bot  San Test, then the token
bin\san_remote.exe telegram delete-bot @san_test_bot
bin\san_remote.exe telegram create-group "San Tower Test"           # -5012345678  San Tower Test
bin\san_remote.exe telegram add-member -5012345678 @someone
bin\san_remote.exe telegram remove-member "San Tower Test" @someone
bin\san_remote.exe telegram delete-group -5012345678
```

- **A group** is its id (what create-group prints, `-5012345678`) or its
  title, which must be one of your groups' only: the search's results are
  checked against what the page knows (a group you are in, not a channel),
  so a public group of the same title does not count, and two of yours are
  an error that lists their ids. A basic group's id (what create-group
  makes) is also its chat id for the Bot API, and so for the tower's
  Telegram Send.
- **Users and bots** are usernames, with or without the @, or a t.me link.
  A row shows a name, not a username: the username of each row is read in
  the page's own world (`rootScope.managers`, `getPeerUsername`), which is
  also how the tool checks it talks to the real @BotFather (user 93372553)
  and not a look-alike the search also finds.
- **The token** is printed last, on a line of its own; the MCP tool returns
  it too. It is a secret.
- Telegram Web does not start in a tab that is not shown (its chat list
  stayed empty, its account id 0, and reading the account never returned),
  so every command, status too, brings Chrome to the front with a tab of
  its own. A Telegram Web tab of yours goes on working beside it.
- A chat row opens on a mouse button going down, so rows and menus get a
  whole press (mousedown, mouseup, click); buttons a click. The message box
  is a ProseMirror editor that takes `execCommand('insertText')`; the send
  button sends once it shows its send state (it records voice messages while
  the box is empty).
- The interface must be in English: the steps press menu items and buttons
  by their names (New Group, Delete Group, Remove from group, Add), in any
  case, since a popup draws its buttons in capitals ("ADD") and the page
  reports them so.
- Add Members clears its search once a user is ticked, so the tick is read
  from the chip above the search (`.selector-user[data-key]`). The delete
  popup's "Delete for all members" is a row (`label.popup-peer-checkbox-row`,
  its name in `.row-title`). A delete is confirmed by what the page knows of
  the group (Telegram marks it deactivated), not by the screen.

Live-tested on 2026-10-27, as the account the browser is signed in to: a
bot made (@BotFather's token read, 46 characters), a group made, the bot
added to it by the group's id and removed by its title (the bot had the
same name: only the group counted), the group deleted for all members, a
second group made and deleted by title, the bot deleted, and deleting it
again refused as not one of the account's bots.

A failed step stops with a screenshot: `san_remote-telegram-150405.png`.

## ChatGPT

`san_remote chatgpt` asks ChatGPT through your browser's ChatGPT session, the
way you would in the composer. It is the extension way
[san_gpt](../../../../docs/external_repo/san_gpt.md#alternative-a-chrome-extension)
weighed:
- **ask** starts a **Temporary chat** (not kept in your history), writes the
  prompt, presses Send, waits for the answer, and prints it as **markdown**:
  the answer's own Copy button gives it, caught in the page before it reaches
  your clipboard.
- `--keep` asks in a normal chat instead, and prints its link to stderr;
  `--chat <link>` continues a kept chat.
- **status** says who is signed in, and on which plan.

```powershell
bin\san_remote.exe chatgpt status                      # signed in as Get Resolved (Free)
bin\san_remote.exe chatgpt ask "Dua kota besar di Indonesia, sebagai daftar"
Get-Content prompt.md | bin\san_remote.exe chatgpt ask -
bin\san_remote.exe chatgpt ask --file prompt.md --wait 20m
bin\san_remote.exe chatgpt ask --keep "Mulai topik baru"            # == chat https://chatgpt.com/c/...
bin\san_remote.exe chatgpt ask --chat https://chatgpt.com/c/... "Lanjutkan"
```

- The page's own code sends every request; the tool only writes in the
  composer and reads what the page shows.
- Both commands work in a background tab, which is closed afterwards.
- `--wait` (default 10 minutes) bounds a long answer, such as a reasoning
  model's.
- ChatGPT's interface must be in English.

## Gemini

`san_remote gemini` asks Gemini (gemini.google.com) through your browser's
Google session, as `chatgpt` does ChatGPT:
- **ask** starts a **Temporary chat** (not kept in your history), writes the
  prompt, presses Send message, waits for the answer, and prints it: the
  answer's own Copy button gives it, caught in the page before it reaches
  your clipboard.
- `--keep` asks in a normal chat instead, and prints its link to stderr.
- `--chat LINK` goes on in a kept chat (2026-10-27): it opens the chat by its
  link, as a person opens one they kept, and asks there.
- **status** says which Google account is signed in.

```powershell
bin\san_remote.exe gemini status                       # signed in as <your name>
bin\san_remote.exe gemini ask "Dua kota besar di Indonesia, sebagai daftar"
Get-Content prompt.md | bin\san_remote.exe gemini ask -
bin\san_remote.exe gemini ask --file prompt.md --wait 20m
bin\san_remote.exe gemini ask --keep "Mulai topik baru"             # == chat https://gemini.google.com/app/...
bin\san_remote.exe gemini ask --chat https://gemini.google.com/app/... "Lanjutkan"
```

- **ask brings Chrome to the front** for as long as it asks: Gemini sends a
  prompt only while its page is shown (a Send pressed in a background tab
  waits until the tab comes to the front). status reads in a background tab.
- `--wait` (default 10 minutes) bounds a long answer.
- Gemini's interface must be in English.

## Claude

`san_remote claude` asks Claude on claude.ai through your browser's
claude.ai session, as `chatgpt` does ChatGPT:
- **ask** starts an **incognito chat** (kept out of your history and search),
  writes the prompt, presses Send message, waits for the answer, and prints
  it as **markdown**: the answer's own Copy button gives it, caught in the
  page before it reaches your clipboard.
- `--keep` asks in a normal chat instead, and prints its link to stderr.
- `--chat LINK` goes on in a kept chat (2026-10-27), opened by its link.
- **status** says who is signed in, and on which plan.

```powershell
bin\san_remote.exe claude status                       # signed in as sease (Free)
bin\san_remote.exe claude ask "Dua kota besar di Indonesia, sebagai daftar"
Get-Content prompt.md | bin\san_remote.exe claude ask -
bin\san_remote.exe claude ask --keep "Mulai topik baru"             # == chat https://claude.ai/chat/...
bin\san_remote.exe claude ask --chat https://claude.ai/chat/... "Lanjutkan"
```

- Both commands work in a background tab, which is closed afterwards.
- A lapsed claude.ai session sends the tab to the sign-in page: `not signed
  in`; sign in to claude.ai in that browser.
- claude.ai's interface must be in English.
- Claude Code on this computer is another way to ask Claude: the tower's
  Claude Ask (CLI) step runs `claude -p`; this is claude.ai in the browser.

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
| `api --listen` | address the API listens on, loopback only | `127.0.0.1:7478` |
| `api --log-level` | debug (each site step), info, warn, error | `info` |
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
- The one way onto the network is a program that runs the servers itself,
  through the `server` package, and asks for it: `LANHandler` and `LANAPI`
  (`hub.GuardLAN`) also take a private network's address or this machine's
  name as `Host`, never another name, with the same `Origin` rules. The tower
  does so when its Browser Remote Server is set to All networks (the user,
  2026-10-28); anyone on that network can then act as you.
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
| `no browser is attached` | A command waits up to 45 s for a browser to attach (one that comes back after the server restarted) before saying this. Open the extension's popup. It retries at once and shows why it can't connect: a wrong server URL, or `serve` not running. |
| popup: **Allow User Scripts is off** | Use its button to open the details page and turn the toggle on. |
| `several browsers are attached, pick one` | Add `--browser <name or id>`; `san_remote browsers` lists them. |
| `Frame with ID 0 is showing error page`, or cannot access a page | Chrome won't let extensions run code on its error pages, `chrome://` pages or the Web Store. Load a normal page first. |
| `the code did not finish in 30000ms` | Raise `--timeout`. The code may still be running in the page. |
| `the page did not finish loading` | The page kept loading past the timeout. Raise `--timeout`, or `exec` anyway: the tab is there. |
| it ran, but nothing showed up | The tab opened in a window behind your other apps. Use `open --focus`, or look at the debug panel. |
| Chrome's taskbar button flashes: `bringing the window back to the front` | A step that needs its page on screen (posting, X's and Threads' scheduled lists, Gemini) asks for Chrome, and Windows won't hand it over while you use another window. Click Chrome: the step asks again every 5 s for up to 3 minutes (the user's choice, 2026-10-25: wait, never force the window over your work, where your typing would land in the page), and Windows lets it through on its own once you have been away a while. After that it stops with `waited 3m0s for the window to be shown`. |
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
| vidara: `no such folder: "Promo/2026"` | A folder in a folder is not made on the Upload page. Make it in My videos (New folder), or give a top-level folder. |
| vidara download: `waited … for the player's stream` | The video may still be encoding, or the player changed. Try later, and look at the screenshot. |
| facebook: `the browser is not signed in to Facebook` | Sign in to facebook.com in that browser. |
| facebook: `Facebook's interface is not in Indonesian` | The tool finds Facebook's controls by their Indonesian names. Set Facebook's language to Bahasa Indonesia. |
| facebook: `nothing matches: no Page "…" (yours: …)` | The error lists your Pages or groups, as `status` finds them; name one of those. |
| facebook: `the post is still in … after Hapus` | Facebook was slow to drop it. Run `posts` again: it is usually gone a minute later. |
| x: `X refused the post: …` | X's own reason. "Your account is suspended" and "Some of your media failed to upload" come from the account, not the tool: check it on x.com. |
| x: `X's interface is not in English` | Set X's language to English (Settings → Accessibility, display and languages → Languages). |
| x: `waited … for the composer to take the thread; X may count a post as too long` | X keeps its Post button off. Run `--dry-run` to see each post's count. |
| `aborted: No current window` | Chrome is running with no window open. Open a Chrome window. |
| medium: `the browser is not signed in to Medium` | Sign in to medium.com in that browser. |
| medium: `Medium does not suggest the topic …` | The error lists what Medium suggests; give one of those. |
| medium: `the page shows "…", not …; nothing was scheduled` | The picker's time differs from the one asked for, such as Chrome in another time zone. |
| linkedin: `the browser is not signed in to LinkedIn` | Sign in to linkedin.com in that browser. |
| tiktok: `Studio shows "Something went wrong"` | Studio's page broke; press Retry in that tab, and run the post again. |
| tiktok: `after Schedule Studio asks: …` | Studio's question is left to you; answer it in the tab. |
| chatgpt: `ChatGPT gave no answer: …` | ChatGPT's own words, such as a usage limit. |
| gemini: `Gemini gave no answer: …` | Gemini's own words, such as a usage limit. |
| claude: `the browser is not signed in to claude.ai` | Sign in to claude.ai in that browser (a lapsed session lands on the sign-in page). |
| claude: `Claude gave no answer: …` | claude.ai's own words, such as a usage limit. |
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
- `go test ./...` runs the hub, the CLI, the MCP server and the API
  against fake browsers (`internal/hubtest`) that attach over Connect as the
  extension does. The API's tests check the pass-through and that what a
  site would refuse never reaches a browser.
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
- `TestXSchedule` needs `SAN_REMOTE_E2E_X=1` too. It schedules a
  two-post test thread 30 days ahead with an image on the second post,
  finds it in the Scheduled tab, moves it a day later, deletes it, and
  checks that it is gone. Its cleanup deletes the test thread even when a
  step fails. Nothing is posted now.
- `TestFacebookSchedule` needs `SAN_REMOTE_E2E_FACEBOOK=1` too. It schedules
  an Only-me test post with an image 25 days ahead on your profile, finds it
  in the Content Library's Terjadwal, deletes it, and checks that it is
  gone. Its cleanup deletes the test post even when a step fails. Nothing is
  posted now.
- `TestMediumDraft` (`SAN_REMOTE_E2E_MEDIUM=1`) writes a test article with
  two captioned images into a draft, schedules it 30 days ahead with a topic,
  finds it in the Scheduled tab, unschedules it and deletes it.
- `TestLinkedInDraft` (`SAN_REMOTE_E2E_LINKEDIN=1`) does the same on
  LinkedIn, with a cover and a two-line post text, then moves the article back
  to the drafts and deletes it.
- `TestTikTokSchedule` (`SAN_REMOTE_E2E_TIKTOK=1`, ffmpeg) schedules a 6 s test
  video 9 days ahead for Everyone, checks Studio's Posts, and deletes it.
- `TestChatGPTAsk` (`SAN_REMOTE_E2E_CHATGPT=1`) asks a short question in a
  Temporary chat.
- `TestGeminiAsk` (`SAN_REMOTE_E2E_GEMINI=1`) does the same on Gemini.
- `TestClaudeAsk` (`SAN_REMOTE_E2E_CLAUDE=1`) does the same on claude.ai, in
  an incognito chat.

  Each cleanup removes what its test made, even when a step fails.

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
  - One video per upload. Not done: URL upload, clone, thumbnails,
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
- **X:**
  - Its interface must be in English, for the composer's "Will send on"
    line and its Schedule button. Most controls are found by X's
    `data-testid` attributes.
  - `delete` of a published post has not been run on a real post yet.
  - Not done: Drafts (unsent posts), polls, GIF search, location, reply
    settings, alt text, and Premium's longer posts.
- **Medium:** its classic editor's markup (`.postArticle-content`,
  `graf--*`); a redesign stops the tool with a screenshot. Not done:
  publishing at once, publications, the preview's title and image, the
  canonical link, and changing an existing draft's text.
- **LinkedIn:** `get` has not been run live. Not done: publishing at once,
  newsletters, posting as a company page, changing who the post is for.
- **TikTok:** not done: Studio's other settings (sound, cover, comments,
  duet), drafts, photos, and editing a scheduled post. `--now` (Only you) has
  not been run live.
- **ChatGPT:** text prompts only; no files, images, model choice or
  regenerate. `--keep` and `--chat` have not been run live.
- **Gemini:** text prompts only; no files, images, mode choice, continuing a
  chat or regenerate. `--keep` has not been run live.
- **Claude (claude.ai):** text prompts only; no files, model choice,
  projects, continuing a chat or retry. `--keep` has not been run live.

## See also

- [docs/external_repo/san_remote.md](../../../../docs/external_repo/san_remote.md):
  the design and the reasons for it
- [typescripts/san_remote_extension/README.md](../../../../typescripts/san_remote_extension/README.md):
  the extension's files
- [docs/external_repo/san_gpt.md](../../../../docs/external_repo/san_gpt.md):
  the CDP approach this is the alternative to
