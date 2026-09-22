# GO GPT

GO GPT holds a conversation on chatgpt.com from Go, driving a real logged-in
Chrome session rather than an API key. It exists for the case where the thing
being paid for is a ChatGPT subscription and the thing that needs to use it is
a program.

The question the first draft asked — doable or not — is answered: **yes**,
provided the browser stays in the loop for the request itself and not only for
the login. "Why the browser cannot be dropped" below is the whole reason this
document has a design section at all; without those two walls it would be a
hundred lines of `net/http`.

## General.
1. its written by golang
2. package is `github.com/wargasipil/go_gpt`
3. folder is `./golang/packages/go_gpt`. Unlike `san_tunnels` it is an ordinary
   directory for now, not a submodule: that needs the GitHub repository to
   exist and be pushed first
4. we use `github.com/chromedp/chromedp`, plus `chromedp/cdproto` directly for
   the CDP domains chromedp does not wrap (`page`, `runtime`, `network`)
5. Chrome runs **headful** against a persistent profile, and is **attached to**,
   not launched per run
6. we never construct a request to the backend. The app's own JavaScript sends
   it; we supply the prompt and read the response stream back out

## Layout.

```
golang/packages/go_gpt/
  client.go            Client, browser attachment, options, NewConversation
  conversation.go      Send/Receive/Read/Err/ID/Close, the turn state machine
  tab.go               the tab seam, chromedp implementation, selectors, chunk pump
  inject.js            the fetch wrapper, embedded and installed at document start
  stream.go            SSE parser: snapshot and delta_encoding v1
  errors.go            ErrNeedsHuman and the rest
  cmd/spike/main.go    attach, send, stream an answer back, turn-taking on stdin
  build.sh / build.ps1 vet, test, build both binaries
```

Verified end to end in a real Chrome against a local fake that speaks the same
protocol (`integration_test.go`): the script installs before app code and the
app's own `fetch` is the one that runs, the tee feeds both the page and Go
without either stalling, the v1 continuation rule survives a real socket, text
typed into a contenteditable composer submits on Enter, the generating state
makes the second `Send` wait, and two turns keep one conversation id. The
parser is covered separately against both encodings, reasoning traces, the
user echo, patch batches, malformed payloads and byte-at-a-time chunking.

What that does **not** cover is the real site: its markup, and the bot
protection in front of it. Those need a logged-in browser and a human at it
once, which is what `cmd/spike` is for:

```
chrome --remote-debugging-port=9222 --user-data-dir=<profile>   # log in by hand
bin/spike -remote http://127.0.0.1:9222 -prompt "halo"
```

## The three steps.
1. we use golang chromedp
2. open chatgpt.com
3. start conversation by calling with golang

Step 3 is the load-bearing one, and it is satisfied by Go driving the browser,
not by Go issuing the HTTP request. The distinction is invisible at the call
site and decisive everywhere else.

## Goals.
1. the goals is i can start conversation in golang like this:
```go

conver := go_gpt.NewConversation(...)

for conver.Receive() {

    msg := conver.Read()

    conver.Send(..)
}

```

This is the `bufio.Scanner` idiom and it survives contact with the design:
`Receive()` blocks until the response stream reports completion, `Read()`
returns the turn that just landed, `Send()` types the next prompt. The
proposed surface under "API" is this sketch with an error path, a context, a
`Close` and a seeded first turn added — nothing that changes its shape.

## Why the browser cannot be dropped.

The obvious plan is to use Chrome once to log in, lift the session token, and
then speak to `/backend-api/conversation` from Go directly. It fails on two
independent walls, either of which is fatal alone.

**The TLS and HTTP fingerprint.** chatgpt.com sits behind Cloudflare. Go's
`crypto/tls` ClientHello does not look like Chrome's, and neither does its
HTTP/2 settings frame. A valid session cookie arriving on a non-Chrome
fingerprint is exactly the signal that protection exists to catch. This is
beatable with a fingerprint-mimicking transport, and that is a treadmill.

**The sentinel proof of work.** `/backend-api/conversation` requires a token
from `/backend-api/sentinel/chat-requirements` plus a proof-of-work answer
computed over it, bound to the device id and user agent. The solver lives in
the app's bundle. Reimplementing it is possible and has been done publicly
more than once; it has also broken repeatedly, without notice, because there
is no compatibility promise on an internal endpoint.

**The corollary that catches people:** staying inside the browser does not buy
the second wall for free. Calling `fetch('/backend-api/conversation')` from
injected JavaScript satisfies Cloudflare completely — same origin, same
cookies, same TLS — and still leaves the proof of work unsolved, because the
solver is bundled and minified and is not reachable from `window`. That path
is the same reimplementation in a different language.

So the rule this design follows is narrower than "use a browser":

> The page's own code issues every request. We steer the input and observe the
> output, and construct nothing.

Everything below is a consequence of that.

## Concept.

```
Go (go_gpt)                             Chrome (headful, real profile)
───────────                             ──────────────────────────────
Send(ctx, prompt) ───── type + Enter ──► composer (DOM)
                                               │
                                         the app's own fetch
                                               │
                                               ▼
                                        /backend-api/conversation
                                               │ SSE
                                         wrapped fetch tees
                                          │              │
                                          │              └──► app (untouched,
                                          │                    UI still works)
  <-chan Token ◄── EventBindingCalled ── sanGPT(chunk)
```

The tee is what makes this non-invasive: the app receives its own response
body unmodified and renders normally, while a second reader forwards the same
bytes to Go. Nothing is intercepted or rewritten, so there is no behaviour for
the page to notice.

## CDP pieces.

- **`page.AddScriptToEvaluateOnNewDocument`** installs the `window.fetch`
  wrapper before any application code runs. On a conversation URL it does
  `response.body.tee()`, returns one half to the caller inside a new
  `Response` carrying the original init, and reads the other half itself. Both
  halves must be consumed promptly or the stream stalls on backpressure.
- **`runtime.AddBinding`** exposes a function to the page whose calls arrive
  in Go as `*runtime.EventBindingCalled`, picked up with
  `chromedp.ListenTarget`. This is the transport for the teed chunks: raw SSE
  deltas, in order, with no DOM parsing anywhere.
- **`network.StreamResourceContent`** is the no-injection alternative on
  recent Chrome — it streams a response body to CDP with nothing added to the
  page. Cleaner where available. Start with the wrapper, which works on any
  version, and move if it proves stable.
- **Sending** is the one place we touch the DOM: locate the composer, then
  `chromedp.KeyEvent`. Assigning `.value` does not work — React listens for
  input events, not property writes — and synthetic key events have the
  additional merit of going through the app's own handlers.

The DOM coupling is therefore confined to two selectors, the composer and the
send affordance. When the UI moves, that is a selector edit. The alternative
designs put the fragility in the protocol instead, where a change is a
reverse-engineering session.

## API.

```go
type Client struct{ … }                 // owns the browser attachment
func New(ctx context.Context, opts ...Option) (*Client, error)
func (c *Client) NewConversation(ctx context.Context, opts ...ConvOption) (*Conversation, error)

type Conversation struct{ … }           // owns one tab
func (c *Conversation) Send(ctx context.Context, text string) error
func (c *Conversation) Receive() bool
func (c *Conversation) Read() Message
func (c *Conversation) Err() error
func (c *Conversation) ID() string
func (c *Conversation) Close() error
```

`Client` and `Conversation` are split because one browser holds many tabs, and
because the health check, the reattach-after-crash logic and the human-needed
signal belong in one place rather than smeared across every conversation.

Contract points that are forced by the browser and are not visible in the
signatures:

- **`Send` returns when the prompt is accepted, not when it is answered.**
  `Receive` is what waits for the answer.
- **`Send` blocks while a response is generating,** because the composer is
  disabled during generation. The turn-taking loop under "Goals" respects this
  by construction; a concurrent caller does not, and must be made to wait.
- **`ID()` is empty until the first send lands.** The conversation does not
  exist server-side before then. Once it does, the tab URL becomes
  `chatgpt.com/c/<uuid>`, which is what lets a session be resumed or audited
  later.
- **`Receive()` returning false is not necessarily the end.** Check `Err()`.
  The distinction that matters to a caller is `ErrNeedsHuman` — an expired
  session, a fresh Cloudflare interstitial, a verification checkbox — because
  it is the one failure that retrying cannot fix.

Streaming is an option rather than a change to `Read`, so the simple loop
stays simple:

```go
conver := go_gpt.NewConversation(ctx,
    go_gpt.WithPrompt("halo"),                            // seeds the first Receive
    go_gpt.WithDelta(func(s string) { fmt.Print(s) }),    // live tokens
)
```

`WithPrompt` exists because the loop as written opens with `Receive()`, and a
conversation with nothing sent yet has nothing to receive. Seeding it keeps
the call site exactly as the goal states it.

## Operating it.

- **Headful needs a display.** On a server that means Xvfb. `--headless=new`
  is *more* distinguishable than old headless, not less, so it is not the
  shortcut it appears to be.
- **Attach, do not launch.** Chrome is started once with
  `--remote-debugging-port` and `--user-data-dir`, logged in by hand, and left
  running; `chromedp.NewRemoteAllocator` connects to it. A profile that has
  been used by a human is materially less interesting to bot detection than a
  fresh one, and the manual login becomes a one-time cost rather than an
  automation problem.
- **Budget 400–600 MB per browser,** plus each tab.
- **Concurrency is capped by the account, not by the code.** Tabs give
  parallel conversations, but rate limiting and proof-of-work throttling are
  per account, so beyond two or three concurrent sends the work queues
  regardless. A serialized queue with a small worker count is simpler and
  honest about the real ceiling.
- **Plan for the human-needed state.** It will happen. The package needs a
  health check — is the composer present? — and a way to surface "someone has
  to look at this browser" rather than hanging. This is the most important
  piece of operational design here and the easiest to leave until it hurts.
- **Chrome dies.** Supervise it and reattach.

## Alternative: a Chrome extension.

The other shape that satisfies the same rule is an extension living on
chatgpt.com, holding a WebSocket back to the Go process: Go sends prompts, the
extension drives the page and streams back. No debugging port and no
automation surface to detect at all, and it tolerates Chrome updates better.

It costs an unpacked extension to install and maintain, MV3 service workers
that get killed and need reconnect logic, and worse debugging.

CDP is the faster way to prove the idea. The extension is the stronger choice
if this ends up running unattended for a long time, or if detection ever
becomes the binding problem.

## Risk.

This is against OpenAI's terms of use, which prohibit programmatic access to
the ChatGPT interface outside the API and circumventing protective measures.
The realistic exposure is the account, not a lawsuit. If the requirement is
only "call a model from Go", the platform API costs a few lines and carries
none of this. The design above is worth building only when the requirement is
specifically to use the subscription from code.

## Open questions.

- **Model selection.** In the browser this is a dropdown or a `?model=` query
  parameter, both more fragile than the rest of the design. Unresolved whether
  it is worth exposing at all.
- **Stop and regenerate.** Not in the goal API. Stop is straightforward, the
  button exists; regenerate changes what `Receive` means for a turn that has
  already been read.
- **Attaching to an existing conversation** at `chatgpt.com/c/<uuid>` works via
  `WithConversation`, but whether prior history should be replayed to the
  caller, and how, is unsettled. Today `Receive` waits for the next new turn.
- **`network.StreamResourceContent` exists** in the pinned cdproto, so the
  no-injection path is open. The fetch wrapper stays for now because it is
  version-independent and works; this is worth revisiting only if injecting
  ever becomes the thing that gets noticed.
- **The selectors are the untested surface.** `#prompt-textarea` and
  `[data-testid="stop-button"]` are what the current app uses, but nothing here
  has confirmed them against the live site. If the first spike run fails, this
  is the first place to look, and `WithSelectors` is the fix.
