# san_youtube

Read the chat of a YouTube live stream from Go, in real time.

`san_youtube` reads chat the same way the chat window on youtube.com does. It
needs no API key, no OAuth and no quota. You give it a video URL, a video ID or
a channel handle. You get every message from "Live chat" (not the filtered
"Top chat") as typed Go values: text messages, Super Chats, Super Stickers,
memberships, gifted memberships, deletions and replacements.

Module `github.com/wargasipil/san_youtube`, Go 1.26, standard library only.

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

## Limits

- **Internal API.** This is the web client's own API, and YouTube can change
  it at any time. The parser skips anything it does not know, instead of
  failing on it. When the page layout changes, `go test` still passes, so run
  the integration test to find out.
- **Delay.** The browser gets push notifications that tell it when to poll.
  This package has no push, so it polls at the `timeoutMs` YouTube gives,
  about every 10 seconds for a viewer who is not logged in. Messages arrive in
  batches at that pace, with their real timestamps.
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
