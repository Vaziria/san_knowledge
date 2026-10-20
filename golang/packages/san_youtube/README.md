# san_youtube

Read the chat of a YouTube live stream from Go, in real time, and upload
videos such as Shorts (see [Uploading videos](#uploading-videos)).

`san_youtube` reads chat the same way the chat window on youtube.com does. It
needs no API key, no OAuth and no quota. You give it a video URL, a video ID or
a channel handle. You get every message from "Live chat" (not the filtered
"Top chat") as typed Go values: text messages, Super Chats, Super Stickers,
memberships, gifted memberships, deletions and replacements.

Module `github.com/wargasipil/san_youtube`, Go 1.26. The chat reader, package
`san_youtube`, uses the standard library only. The uploader, package
`studio`, also needs [chromedp](https://github.com/chromedp/chromedp),
[yaml.v3](https://gopkg.in/yaml.v3) and
[san_profile_manager](../san_profile_manager/README.md), which is wired in
with a `replace` line in `go.mod`. A program that only reads chat, such as
san_radio, does not carry them.

## Usage

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	san_youtube "github.com/wargasipil/san_youtube"
)

func main() {
	ctx := context.Background()

	client := san_youtube.New()
	chat, err := client.Open(ctx, "@KompasTV") // or a video URL, or a video ID
	if err != nil {
		log.Fatal(err) // e.g. san_youtube.ErrNoLiveStream, ErrChatDisabled
	}

	for msg, err := range chat.Messages(ctx) {
		if errors.Is(err, san_youtube.ErrEnded) {
			break // the stream is over
		}
		if err != nil {
			log.Fatal(err)
		}
		switch m := msg.(type) {
		case *san_youtube.TextMessage:
			fmt.Printf("%s: %s\n", m.Author.Name, m.Text)
		case *san_youtube.SuperChat:
			fmt.Printf("%s paid %s (%v %s): %s\n",
				m.Author.Name, m.Amount.Text, m.Amount.Value, m.Amount.Currency, m.Text)
		case *san_youtube.Membership:
			fmt.Printf("%s: %s %s\n", m.Author.Name, m.Header, m.Subtext)
		}
	}
}
```

`Open` accepts:

| Input | Example |
|---|---|
| video ID | `DOOrIxw5xOw` |
| video URL | `https://www.youtube.com/watch?v=DOOrIxw5xOw`, `youtu.be/…`, `/live/…`, `/shorts/…`, `/embed/…`, `m.youtube.com` |
| handle | `@KompasTV`, `https://www.youtube.com/@KompasTV/live` |
| channel ID or URL | `UC5BMIWZe9isJXLZZWPWvBlg`, `https://www.youtube.com/channel/UC…` |

For a handle or channel, `Open` uses the stream that channel is live with now.

Options:

| Option | What it does |
|---|---|
| `WithHTTPClient(c)` | your own `*http.Client` (proxy, tests); the default has a 30 s timeout |
| `WithLanguage("id")` | the language of YouTube's own texts. With `"id"` a Super Chat reads `Rp 20.000`; with the default `"en"` it reads `IDR 20,000`. Messages are never translated. |
| `WithRetries(n)` | retries after network errors, 5xx and 429 (default 5; waits 1 s, 2 s, 4 s … up to 30 s) |
| `WithUserAgent(ua)`, `WithBaseURL(u)` | override the browser User-Agent, or point at a fake server |
| `WithoutHistory()` (per chat) | skip the recent messages a chat starts with. `Open` reads them itself, so everything sent after `Open` still comes. |
| `WithPollDelay(d)` (per chat) | the wait between polls when YouTube does not give one (default 5 s) |
| `WithPollInterval(d)` (per chat) | poll at least every `d`, sooner than YouTube asks (about 10 s), so messages arrive within about `d`, for that many more requests. After YouTube answers 429, the chat waits as YouTube asks for 10 minutes. |

### Messages

Every message embeds `Base`: `ID`, `Time`, `Author` and `Raw` (the action JSON
exactly as YouTube sent it). `msg.Meta()` returns it for any type. `Author` has
`Name`, `ChannelID`, `PhotoURL`, the flags `Owner`, `Moderator`, `Verified`,
`Member`, and the full `Badges` list (with tooltips such as
"Member (6 months)").

| Type | Kind (`"type"` in JSON) | Main fields |
|---|---|---|
| `*TextMessage` | `text` | `Text`, `Runs` |
| `*SuperChat` | `super_chat` | `Amount{Text, Value, Currency}`, `Text`, `Runs` |
| `*SuperSticker` | `super_sticker` | `Amount`, `Sticker` (description), `StickerURL` |
| `*Membership` | `membership` | new member: `Subtext` ("Welcome to …!"); milestone: `Milestone`, `Months`, `Header`, `Text` |
| `*GiftPurchase` | `gift_purchase` | `Count`, `Text` (the author is the buyer) |
| `*GiftRedemption` | `gift_redemption` | `Gifter`, `Text` (the author received the gift) |
| `*Deletion` | `deletion` | `TargetID`, `Text` ("[message deleted]", "[message retracted]", or empty when removed) |
| `*AuthorDeletion` | `author_deletion` | `Author.ChannelID`, `Text`: every message by that author is gone |
| `*Replacement` | `replacement` | `TargetID`, `Message` (a held message released from review) |

`Text` keeps Unicode emoji as they are and writes custom emoji as their
shortcut, for example `:yt:`. `Runs` has every piece separately: text, emoji
(`ID`, `Shortcuts`, `Custom`, `ImageURL`) and links (`URL`, already unwrapped
from YouTube's redirect). Every type has JSON tags, and its JSON has a `"type"`
field. `ParseAmount` is exported, for amounts from elsewhere.

### Errors

| Error | When |
|---|---|
| `ErrChatDisabled` | the stream has its chat turned off |
| `ErrMembersOnly` | the video or chat is for channel members only |
| `ErrNotLive` | a regular video, or a scheduled stream whose chat is not open |
| `ErrEnded` | the stream is over: at `Open`, or as the last value of `Messages` |
| `ErrVideoUnavailable` | no such video, or it is private |
| `ErrNoLiveStream` | the channel is not live now (a scheduled stream is named in the message) |
| `ErrChannelNotFound` | the handle or channel does not exist |
| `ErrInvalidInput` | not a YouTube URL, ID or handle |
| `ErrRateLimited` | YouTube kept answering 429 after all retries |
| `ErrConsent` | the EU cookie consent page could not be passed |
| `*StatusError` | another HTTP status (a 4xx, or a 5xx after all retries) |

Errors that come from YouTube's answers wrap one of these, so use
`errors.Is` and `errors.As`. Two kinds of error do not:

- A network failure that outlasts the retries wraps the `net/http` error
  (`*url.Error`).
- An answer that is not valid JSON wraps the `encoding/json` error.

When the context is cancelled, `Messages` ends with `ctx.Err()`.

## Command line

```
san_youtube [flags] <video URL | video ID | @handle | channel URL>

  -json      one JSON object per line instead of text
  -new       skip the recent history the chat starts with
  -for 1m    stop after this long (default: until the stream ends or Ctrl+C)
  -lang id   language of YouTube's own texts
  -poll 1s   poll at least this often, instead of as YouTube asks (about every 10 s)
```

```
$ bin/san_youtube -new @KompasTV        # the format; names are made up
== reading the live chat of https://www.youtube.com/watch?v=DOOrIxw5xOw
02:13:54 @viewer [mod, member]: Selamat pagi semua!
02:13:57 [Super Chat IDR 50,000] @viewer: semangat
02:14:02 [New member] @viewer: Welcome to Kenalan!
^C
== 3 messages
```

Messages go to stdout and status lines (`== …`) go to stderr, so
`san_youtube -json @KompasTV > chat.jsonl` saves only the messages.

Build with `pwsh build.ps1` (Windows) or `bash build.sh` (Linux). Each runs
`go vet` and `go test` first, then writes `bin/san_youtube.exe` and
`bin/san_youtube`.

## Examples

Small programs in `examples/`, each run from this folder with `go run`:

| Example | What it does |
|---|---|
| `go run ./examples/basic @KompasTV` | prints the chat: text, Super Chats, memberships, gifts |
| `go run ./examples/superchats -for 30m @KompasTV` | prints each Super Chat and Super Sticker, then the total per currency and the most frequent supporters |
| `go run ./examples/keywords -for 10m @KompasTV shopee tokopedia "tiktok shop"` | prints the messages that mention a word, then how many messages and viewers mentioned each |
| `go run ./examples/multi -for 5m @KompasTV DOOrIxw5xOw` | reads several streams at once with one shared `Client`, one goroutine per `Chat` |

## How it works

1. **Channel to video.** For a handle, `GET /@name/live`. YouTube serves the
   watch page of the stream that is on, and its canonical link gives the
   video ID. The player data on that page confirms it is live now. A channel
   that is not live gets its channel page instead, which gives
   `ErrNoLiveStream`.
2. **Chat page.** `GET /live_chat?is_popout=1&v=<id>`. From `ytcfg` it reads
   `INNERTUBE_API_KEY`, `INNERTUBE_CLIENT_VERSION` and `INNERTUBE_CONTEXT`
   (there are fallbacks if they are missing). From `ytInitialData` it reads the
   view selector, and takes the "Live chat" continuation. The page itself opens
   on "Top chat", so its own messages are not used.
3. **Polling.** `POST /youtubei/v1/live_chat/get_live_chat` with the page's
   context and the continuation, like the browser. Each answer has `actions`
   (the messages) and the next continuation: `invalidationContinuationData` or
   `timedContinuationData` with a `timeoutMs` to wait, or
   `reloadContinuationData`. The first answer repeats recent history, and so
   does every reload, so messages are deduplicated by ID (the last 10,000 IDs
   are remembered). A replacement that brings a message you already have is
   dropped. An author deletion comes again only if that author wrote
   something after the last one.
4. **End.** An answer without a continuation means the stream is over:
   `ErrEnded`.
5. **Why no chat.** If the chat page has no chat, one extra request to the
   watch page tells which error it is: disabled, regular video, ended, not
   started, members only, unavailable or private. This request is only made on
   that error path.

Every request sends a desktop Chrome User-Agent, an `Accept-Language`, and the
cookie `SOCS=CAI` (the "reject all" answer to the EU consent page), so the
consent page is normally never shown. If it is shown anyway, the "Reject all"
form is submitted once, the cookies it sets are kept, and the page is fetched
again. Network errors, 5xx and 429 are retried with backoff. A 4xx on a
continuation that worked before (for example, after the computer slept)
reloads the chat page once, to get a fresh continuation.

## Uploading videos

`san_youtube upload` puts a video on YouTube, such as a Short, and saves it as
private, or schedules it to be made public. It never publishes a video at
once: a video either stays private, or goes public at the time you set.

There is no "Short" setting. YouTube makes a video a Short when it is
vertical or square and at most 3 minutes long, such as the 1080×1920 renders
in `videos/*/renders/video.mp4`.

YouTube's Data API can upload too, but it needs a Google Cloud project and
OAuth. A project Google has not audited also keeps every video it uploads
private. So `san_youtube` uses YouTube Studio, the way a person does, in a
real Chrome that is signed in to YouTube. That Chrome runs on the profile the
san_* tools share (see [san_medium](../san_medium/README.md) and
[san_profile_manager](../san_profile_manager/README.md)).

```powershell
pwsh build.ps1                                        # vet, test, build bin/san_youtube.exe and bin/san_youtube
bin\san_youtube.exe login -profile basic              # once: sign in to Google in the window that opens, then close it
bin\san_youtube.exe status -profile basic             # signed in (D:\research\marketing\browser_profiles\basic)
bin\san_youtube.exe upload -dry-run youtube.md        # what would be uploaded; opens nothing
bin\san_youtube.exe upload -profile basic youtube.md  # upload, and save as private
bin\san_youtube.exe upload -profile basic -at "2026-10-20 19:00" youtube.md   # upload, and schedule
bin\san_youtube.exe videos -profile basic             # list Studio's Videos and Shorts, with what is scheduled
```

Check `videos` before scheduling: the plan is one video a day
([docs/social_posting.md](../../../docs/social_posting.md)). It only reads: it opens the upload
dialog the way `upload` does, closes it, and reads the content list's Videos and Shorts tabs.

The Google session is in the `basic` profile (2026-10-13). That account's
channel is **Heri Santoso Dev** (@herisantoso1239), not @GetResolved. A video
goes to the channel the profile is signed in to, and `upload` prints that
channel's name before it chooses the file. To upload to another channel, sign
in to it on its own profile, for example `login -profile getresolved`, and
upload with `-profile getresolved`.

The video's link goes to stdout and the status lines go to stderr. This is
the test run of 2026-10-13, with the test video since deleted:

```
== uploading …\post\san-youtube-test.mp4 as "Uji san_youtube (hapus saja)"
== opening YouTube's home page
== looking at the home page for a moment
== opening Create, then Upload video
== channel: Heri Santoso Dev
== choosing san-youtube-test.mp4
== Studio made video IVvRoPlVkOk; it is private until saved
== typing the title
== typing the description (87 characters)
== answering: not made for kids
== adding 3 tags
== Next
== Next
== Next
== choosing Private
== pressing Save
== closing Studio's dialog: Video processing Your private video is still processing standard definition (SD). …
== finding it in Studio's content list
https://youtube.com/shorts/IVvRoPlVkOk
== saved as private, a Short; Studio lists it as "Pending"
== edit it in Studio: https://studio.youtube.com/video/IVvRoPlVkOk/edit
```

Studio lists a video as "Pending" while it processes it, and as "Private"
afterwards.

Studio makes the video as soon as it has the file. If a later step fails,
the link is still printed, and Studio keeps the video as private, to finish or
delete in Studio.

### The post

A post is a markdown file, usually next to the video
(`videos/<name>/youtube.md`):

```markdown
---
video: renders/video.mp4
tags: [raspberry pi, radio 24 jam, golang]
at: 2026-10-20 19:00
---

# Radio 24 jam dari Raspberry Pi #shorts

Musik sendiri, 24 jam, dari Raspberry Pi 3B+.
Dengarkan: https://www.youtube.com/@GetResolved

#raspberrypi #radio
```

- **Title:** the first `# ` heading. At most 100 characters.
- **Description:** everything after the title, as plain text with its line
  breaks. YouTube does not render markdown, so write a link as a bare URL. At
  most 5,000 bytes.
- **`video`** (required): the video file, relative to the markdown file.
- **`tags`:** a list, or one line with commas. At most 500 characters, counted
  as Studio counts them: a tag with a space counts its two quotes, and there
  is a comma between tags.
- **`at`:** when YouTube makes the video public, local time, as
  `"2006-01-02 15:04"` or RFC 3339. It must be at least 15 minutes ahead.
  Without it, the video stays private.
- YouTube refuses `<` and `>` in a title or description, so the tool does
  too, before Chrome opens.

`upload` also takes a video file instead of a markdown file, with `-title`.

| Flag | What it does |
|---|---|
| `-title`, `-description` | replace the file's title or description |
| `-tag <t>` | a tag; may be repeated, or hold commas. Replaces the file's tags. |
| `-at <time>` | the publish time, replacing the file's. `-at ""` keeps the video private. |
| `-dry-run` | print what would be uploaded, and open nothing |
| `-timeout` | give up after this long (default 30m) |
| `-profile`, `-remote`, `-chrome` | the browser, as for san_medium |

Every check runs before Chrome opens: the video file is there, there is a
title, the limits above hold, and the time is far enough ahead.

### How an upload goes

All of it happens in the Chrome window, which you leave alone while it works.

1. **Home page, Create, Upload video.**
   - The tool opens youtube.com. The page has to show the account button and
     Create. A Sign in button instead means `ErrNotLoggedIn`.
   - It waits until the page has stopped loading, then waits another 5–10
     seconds.
   - It clicks Create, then Upload video, which takes the tab to Studio with
     its upload dialog open. A menu that does not open is tried again, up to
     3 times.
2. **The file.**
   - The tool clicks Select files. Chrome hands the file chooser to the tool
     (`Page.setInterceptFileChooserDialog`), which gives it the file.
   - Studio makes the video at once, as private, and shows its link. The link
     gives the video id.
3. **Details.**
   - Studio fills the title with the file name. The tool selects it and types
     the title over it.
   - It types the description, if there is one.
   - It answers "No, it's not made for kids". YouTube asks this of every
     video.
   - For tags, it opens Show more, types them with commas between, and
     presses Enter, which makes each one a chip. Typed commas alone do not.
   - Text goes in with `Input.insertText`, as an input method or a paste
     would. Each field is read back.
4. **Next**, until the Visibility step shows. Video elements and Checks are
   left as they are.
5. **Visibility.**
   - Without `at`: Private.
   - With `at`: Schedule. The date is typed into the date picker (`Nov 13,
     2026`) and the time into the time box (`7:05 PM`). Both are read back.
   - Studio schedules in its default time zone, "Local Time", which is
     Chrome's. The tool first checks that Chrome's offset at that moment is
     this computer's.
   - Right before pressing Save (or Schedule), the tool checks that Private,
     or the schedule, is still the choice. If not, it stops, so a video is
     never published at once.
6. **Upload finished.** Every request to `upload.youtube.com` carries part of
   the file. The tool waits until all of them have ended before it saves,
   since closing the tab any sooner would end the upload.
7. **Save.** Studio fades the upload dialog out; it stays in the page at
   opacity 0, so the tool counts a faded-out element as hidden.
   - When the video is saved before standard definition is ready, Studio
     shows "Video processing" (`ytcp-uploads-still-processing-dialog`). The
     tool closes it with its Close button.
   - Any other Studio dialog that follows is closed the same way, by its
     `#close-button`.
8. **Check.** The tool finds the video in Studio's content list, under Shorts
   or else under Videos, and compares its title. It reports what the list
   shows: "Private" or "Scheduled", and whether the video is a Short.

The tool clicks and types into elements found by id and name, not by label.
So Studio's language does not matter, except for the schedule's date and time,
which are typed in English. With Studio in another language, the read-back
does not match, and the tool stops before anything is scheduled.

### The package

```go
p, err := studio.ParseFile("videos/san-radio-tiktok/youtube.md")
if err != nil {
	log.Fatal(err) // studio.ErrInvalidPost, studio.ErrTooSoon
}
c, err := studio.New(ctx, studio.WithProfile("basic"))
if err != nil {
	log.Fatal(err)
}
defer c.Close()
v, err := c.Upload(ctx, p, studio.WithProgress(func(s string) { log.Println(s) }))
if v != nil {
	fmt.Println(v.URL) // https://youtube.com/shorts/<id>; v.Studio is where to edit it
}
if err != nil {
	log.Fatal(err) // e.g. studio.ErrNotLoggedIn, studio.ErrStudio
}
```

`Login`, `LoggedIn`, `WithRemote` and `WithExecPath` work as in san_medium.
`LoggedIn` reads the profile's cookies and loads no page: it looks for
Google's session cookie `SID` on youtube.com. Google's sign-in sets that
cookie before YouTube is ever visited; YouTube's own `LOGIN_INFO` only comes
with a first visit.

### Limits of uploading

- **Studio's markup.** The ids, names and tags the tool uses are in
  `studio/upload.go` and `studio/studio.js`. A Studio redesign breaks the
  tool with `ErrStudio`, which names the step and saves a screenshot. The
  video Studio has made by then stays private.
- **Left as Studio sets them:**
  - playlists and the thumbnail
  - "altered or synthetic content". Disclose realistic AI-made content in
    Studio yourself.
  - paid promotion, comments and the other "Show more" settings
  - the channel's upload defaults, if it has any. The tool replaces the
    title, and replaces the description and adds tags only when the post has
    them.
- **Not done by the tool:** publishing at once, unlisted, and changing or
  deleting a video after its upload.
- **Links in a description** are plain text until the channel has completed
  YouTube's one-time verification. Studio says so under the description box.
- **One upload at a time per `Client`.** The tab stays in Studio afterwards.
  The next upload starts at the home page again.

## Tests

```
go test ./...                                                # offline, uses testdata/
SAN_YOUTUBE_LIVE=@KompasTV go test -run Integration -v .     # against a real live stream
SAN_YOUTUBE_LIVE=<id> SAN_YOUTUBE_LIVE_FOR=2m go test -run Integration -v .
```

The unit tests use an `httptest` fake of YouTube and fixtures in `testdata/`.
The fixtures are cut down from real responses. All viewer names, channel IDs,
photos, message IDs, the visitor ID and the IP are replaced with fake ones.
The tests cover:

- every message type, and continuation polling with every continuation type
- dedupe, and the end of a stream
- every error case, retries, the consent page, and context cancellation

The integration test runs only when `SAN_YOUTUBE_LIVE` is set, and never
with `-short`.

The uploader's unit tests (`studio/`, `cmd/san_youtube/upload_test.go`)
cover:

- reading the post file: frontmatter, tags as a list or a line, times, the
  title and description, a byte order mark, and bad files
- YouTube's limits, and the publish time being far enough ahead
- the sign-in cookie
- `upload -dry-run`, flags over the file, and every check failing before
  Chrome opens

There is no offline test of the browser part. On 2026-10-13 it was checked
against the real Studio, on the Heri Santoso Dev channel, with a 6-second
1080×1920 test clip and your OK. Each test video was private, and was deleted
afterwards.

- **Private, whole flow: works.**
  - Two runs went through the whole flow and were listed under Shorts. One
    was timed: 71 s.
  - One of them got Studio's "Video processing" dialog, and closed it. The
    other got no dialog.
  - Studio's edit page then showed the title, the description with its line
    breaks, "not made for kids", the three tags, and Private.
- **Fixed along the way:**
  - The tags stayed one line of text until Enter was added.
  - The tool waited for a dialog that Studio only fades out.
  - "Video processing" was not closed.
- **Schedule: not run yet.** The date picker, the time box and the "Local
  Time" zone were looked at by hand: they take typed text, and read back
  `Nov 13, 2026` and `7:05 PM`. The tool's schedule step itself has not run,
  since scheduling makes a video public later and needs its own OK.
- **The Chrome.** The runs used `-remote` on a Chrome started on the `basic`
  profile with `--remote-debugging-port`. The usual start, where the tool
  starts Chrome itself, is san_profile_manager's, as for san_medium. It was
  not run with this tool.

## Limits

- **Internal API.** This is the web client's own API, and YouTube can change
  it at any time. The parser skips anything it does not know, instead of
  failing on it. When the page layout changes, `go test` still passes, so run
  the integration test to find out.
- **Delay.** The browser gets push notifications that tell it when to poll.
  This package has no push, so it polls at the `timeoutMs` YouTube gives,
  about every 10 seconds for a viewer who is not logged in. Messages arrive in
  batches at that pace, with their real timestamps. `WithPollInterval` (`-poll`
  on the command line) polls sooner, for more requests; YouTube's answer to too
  many is a 429, and after one the chat keeps to YouTube's pace for 10 minutes.
- **No login.** It cannot read members-only streams and it cannot send
  messages. Chats of finished streams (replays) are not read; they give
  `ErrEnded`.
- **What is skipped.** System notices, polls, pinned banners, the Super Chat
  ticker, placeholders for held messages, and reactions are not returned.
  `Raw` on each message keeps the full action for anything that is not
  modelled.
- **Amounts** are read from the displayed text, and `Value` is a `float64`.
  `Currency` is empty when the symbol is ambiguous (`kr`) or unknown.
- **English wording.** `Membership.Months` and the "Live chat" title are read
  from English text. With another language, the title falls back to its
  position in the menu and `Months` is 0.
- **Rate limits.** Fetching many pages quickly from one IP gets a 429, and
  YouTube sends you to `google.com/sorry`. The package retries with backoff,
  then returns `ErrRateLimited`. Reading one chat is the same load as one
  browser tab.
- **Times.** Deletions and replacements have no timestamp of their own. Their
  `Time` is when the answer arrived, by the local clock.
- A `Chat` is for one goroutine. A `Client` can be shared.
