# san_medium

Write a markdown article into Medium as a draft, schedule it to be
published, and list, read and delete drafts, from Go or the command line.

Medium no longer issues API tokens, so `san_medium` uses the editor, like a
person. It drives a real Chrome that is logged in to Medium. It opens a new
story, types the title, pastes the text, and puts each image in its place with
the editor's own image button. Then it fills in the image's alt text and
caption. Medium saves the draft itself. The tool then reloads the draft and
checks it against the article. Publishing stays a manual step.

The same steps run in your own everyday browser through San Remote's
extension, with no Chrome of their own: `san_remote medium` (and the
tower's Medium Article step), see
[san_remote's docs](../../apps/san_remote/docs/readme.md#medium).

Module `github.com/wargasipil/san_medium`, Go 1.26. It needs
[chromedp](https://github.com/chromedp/chromedp),
[goldmark](https://github.com/yuin/goldmark) and
[san_profile_manager](../san_profile_manager/README.md) (a sibling folder,
wired in with a `replace` line in `go.mod`), plus Chrome, Edge or Chromium on
the machine.

## The tool

```powershell
pwsh build.ps1                                  # vet, test, build bin/san_medium.exe and bin/san_medium
bin\san_medium.exe login                        # once: sign in to Medium in the window that opens, then close it
bin\san_medium.exe status                       # signed in (D:\research\marketing\browser_profiles\default)
bin\san_medium.exe draft -dry-run article.md    # how the article will be cut; opens nothing
bin\san_medium.exe draft article.md             # make the draft; prints its URL
```

```
== writing "Uji san_medium 2 (hapus saja)" into a new draft
== opening a new story
== typing the title
== pasting the text (1 KB)
== image 1 of 2: vpn-anatomy.png
== image 2 of 2: relay-message-format.png
== waiting for Medium to save
== reloading the draft to check it
https://medium.com/p/cef75548da76/edit
```

The draft URL goes to stdout and the status lines go to stderr. If a step
fails after Medium has created the story, the URL is still printed. That
draft holds everything written up to the failure.

Drafts already on Medium:

```powershell
bin\san_medium.exe drafts                       # a379bedbcab6  Updated just now  Uji san_medium 3 (hapus saja)
bin\san_medium.exe drafts -json                 # {"ID":"a379bedbcab6","URL":"…/p/a379bedbcab6/edit","Title":"…","Updated":"Updated just now","Words":59}
bin\san_medium.exe get a379bedbcab6             # status, title, subtitle, images (URL, alt, caption), then the text
bin\san_medium.exe get -json <id | url>
bin\san_medium.exe delete a379bedbcab6          # asks: Delete the draft "…" (a379bedbcab6)? This cannot be undone. [y/N]
bin\san_medium.exe delete -yes <id | url>       # no question, for scripts
```

A draft can be named by its id or by any of its URLs: `…/p/<id>/edit`, or a
story URL ending in `-<id>`.

- **`drafts`** reads the Drafts tab of Medium's stories page, newest first. It
  scrolls until all of the tab's count are loaded.
- **`get`** opens the draft in the editor and reads it, changing nothing.
- **`delete`** deletes the draft the way a person does on the stories page:
  the row's menu, "Delete story", then "Delete" in Medium's "this action
  cannot be undone" question.
  - It returns once a fresh load of the page no longer lists the draft.
  - Only rows of the Drafts tab are looked at, so a published story is never
    touched; it gives "no such draft".

### Scheduling

Medium publishes a scheduled story by itself, within 5 minutes of the time
set, whether this computer is on or not. The tool sets that time through
Medium's own publish page.

```powershell
bin\san_medium.exe schedule -at "2026-11-10 09:00" -topic Programming -topic Golang <id | url>
bin\san_medium.exe scheduled                    # 0f2769b537b0  Nov 10, 9:00 AM  Uji jadwal san_medium (hapus saja)
bin\san_medium.exe schedule -at "2026-11-10 09:00" -remove-topic "Raspberry Pi" <id | url>   # take a topic off, keep the time
bin\san_medium.exe unschedule <id | url>        # cancels the time; the story stays a draft
bin\san_medium.exe draft -at "2026-11-10 09:00" -topic Programming article.md   # make the draft and schedule it
```

- **`-at`** is local time, as `"2006-01-02 15:04"`, or RFC 3339 with its own
  offset. It must be at least 10 minutes ahead.
- **`-topic`** may be repeated, up to five times. Topics the story already has
  are kept.
- **`-remove-topic`** may be repeated. Each topic is taken off with the Remove
  button of its chip, before any `-topic` is added; a topic the story does
  not have is reported ("the story has no topic …") and skipped.
- **`schedule`** works on any draft, including one already scheduled, which
  gets the new time.

How `schedule` fills the publish page (`/p/<id>/submission?submitType=publishing-post`):

1. Each `-remove-topic` chip's Remove button is clicked, and the chip is
   waited for until it is gone.
2. Each topic is typed into the topic box key by key. Medium then lists
   suggestions, such as "Programming (14M)".
   - The tool presses the down arrow until it reaches the suggestion that is
     exactly the topic, then Enter. Medium saves the topics at once.
   - With the suggestions open, Enter alone adds nothing, and a click on a
     suggestion only closes the list (2026-10-11). A topic Medium does not
     suggest gets Enter alone.
   - A topic that does not show up stops the tool with a screenshot.
3. "Schedule for later" opens the form. The date button opens a picker.
   - The picker's next-month arrow is clicked until the month shows.
   - Then the day, the hour (00–23) and the minute (00–59) are clicked.
   - The picker works in the browser's time zone, which is this computer's.
4. The picker is closed, and the time the page now shows ("11/10/2026, 9:00
   AM GMT+7", offset included) is read back. If it is not the time asked
   for, the tool stops there, and nothing is scheduled.
5. "Schedule to publish" is pressed. The proof is the story in the Scheduled
   tab, which the tool reads as a table (`view=table`), since it opens as a
   calendar.

`unschedule` presses "Cancel scheduling" on the same page; Medium asks
nothing. The proof is the story gone from the Scheduled tab and listed under
Drafts.

Both proofs reload the stories page at most 4 times, 5 seconds apart, never in
a tight loop.

Flags for every command:

| Flag | What it does |
|---|---|
| `-profile <name \| dir>` | the Chrome profile: a name kept by san_profile_manager, or a folder. The default, `default`, is the profile every san_* tool shares, in `browser_profiles/` at the workspace root, which git ignores. |
| `-remote <url>` | use a Chrome you started with `--remote-debugging-port`, such as `http://127.0.0.1:9222`, instead of launching one. That Chrome must be signed in to Medium. Only the tab the tool opens is closed afterwards. |
| `-chrome <path>` | the browser binary, when it is not found in the usual places |

`draft` also takes `-timeout` (default 10m).

### The profile

The tool uses the Chrome profile the san_* tools share, apart from your
everyday one; [san_profile_manager](../san_profile_manager/README.md) keeps it
and starts Chrome on it. Chrome does not allow automation on its default
profile anyway. `login` opens Chrome on that profile at Medium's sign-in page,
with no automation attached, because Google sign-in turns away a browser that
is being driven. Sign in there by any method, then close the window. The
session stays in the profile, so this is needed once, until Medium logs you
out. san_tiktok and the other tools see the same profile, signed in to their
own sites.

While `draft` runs:

- A maximized Chrome window opens and fills itself in. Leave it alone: a click
  or a keystroke in it lands in the draft.
- Chrome is started as a plain process with a debugging port, as a shortcut
  would start it. The tool then attaches over that port and works in a tab of
  its own. Do not let chromedp launch Chrome itself: see "Cloudflare" under
  Limits.
- If another san_* tool already has Chrome running on the profile, the tool
  works in a tab of its own in that Chrome. The last tool to finish closes
  Chrome; a tab you opened yourself keeps it open.
- The window may sit behind other windows, and extensions, Chrome sync and
  "Allow Chrome sign-in" are off. Signing in to Medium with Google then signs
  only Medium in, not Chrome. The san_profile_manager README has the details.
- The profile cannot be used while `login`'s window, or a Chrome you started
  on it yourself, is open: `draft` stops with "the profile is already open in
  another Chrome window".

## The package

```go
package main

import (
	"context"
	"fmt"
	"log"

	san_medium "github.com/wargasipil/san_medium"
)

func main() {
	ctx := context.Background()

	a, err := san_medium.ParseFile("docs/articles/topic/article.md")
	if err != nil {
		log.Fatal(err) // e.g. san_medium.ErrNoTitle, or a missing image file
	}

	c, err := san_medium.New(ctx) // or san_medium.WithRemote("http://127.0.0.1:9222")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	d, err := c.NewDraft(ctx, a, san_medium.WithProgress(func(s string) { log.Println(s) }))
	if d != nil {
		fmt.Println(d.URL) // https://medium.com/p/<id>/edit
	}
	if err != nil {
		log.Fatal(err)
	}
}
```

`Login(ctx, opts...)` and `LoggedIn(ctx, opts...)` are the `login` and
`status` commands. `LoggedIn` reads the profile's cookies and loads no page:
from the Chrome a san_* tool has running on it, or from a headless one it
starts for that. The other calls check by the page instead: the first call
on a `Client` opens Medium's home page, which has to show a signed-in header
(see "How a draft is filled").

Drafts already on Medium:

```go
ds, err := c.Drafts(ctx)                  // []Draft{ID, URL, Title, Updated, Words}
d, err := c.GetDraft(ctx, "a379bedbcab6") // *DraftContent{Status, Title, Subtitle, Text, Images}
err = c.DeleteDraft(ctx, ds[0].ID)        // cannot be undone; ErrDraftNotFound if not a draft
id, err := san_medium.DraftID("https://medium.com/p/a379bedbcab6/edit")
```

Scheduling:

```go
at := time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)
err := c.Schedule(ctx, d.ID, at, san_medium.WithTopics("Programming", "Golang"))
ss, err := c.ScheduledStories(ctx) // []Scheduled{ID, URL, Title, When}
err = c.Unschedule(ctx, d.ID)      // ErrNotScheduled if it is not scheduled
```

`Schedule` returns `ErrTooSoon` for a time less than `MinLead` (10 minutes)
ahead, and `ErrDraftNotFound` for anything that is not a draft.

The calls on one `Client` run one at a time, in its one tab, so a program can
list, read and delete in a row without starting Chrome again.

### What the markdown becomes

The article is read the way [the article guideline](../../../docs/articles/guideline.md)
writes it:

- **Title:** the first `# ` heading, typed into the title.
- **Subtitle:** a paragraph right under the title that is only italic text.
  Medium shows it as the subtitle.
- **Headings:** `##` becomes Medium's large heading, and `###` (and deeper)
  the small one. A further `#` becomes a large heading.
- **Text, lists, quotes, links, inline code, code blocks:** pasted as HTML.
  The editor converts the paste itself; code blocks get its syntax colours.
  Each paragraph of a quote becomes a quote of its own.
- **Images:** an image on a paragraph of its own (`![alt](assets/x.png)`) is
  uploaded from the local file. Its alt text becomes the image's alt text. An
  italic paragraph right after it becomes the caption.
- **Frontmatter** is skipped.

Parsing fails, before any browser opens, when there is no title or an image
file is missing. A remote image (`https://…`) is refused too: save it next to
the article. `Article.Warnings` names what will not survive: an image inside a
line of text (it is left out), and links to local files.

### How a draft is filled

1. **Home page, then Write.** The tool opens Medium's home page and waits
   until it is signed in and filled in. These are the waits of
   `san_profile_manager/cmd/example`, which held in each of the user's runs
   on 2026-10-11:
   - Signed in means a Write link (`/new-story`) is shown in the header.
     Signed out, Write goes through `/m/signin`, so that link does not count.
   - Filled in means no XHR or fetch request is running, and none started or
     ended for 2 seconds.

   It reads the page for 5–10 seconds, then clicks Write and waits for the
   editor. A click on Write now and then opens nothing, so when the tab has
   not moved on 20 seconds later, Write is clicked again, up to 3 times. A sign-in page gives `ErrNotLoggedIn`, as does a page with a shown
   sign-in link and no Write link. A Cloudflare block page gives `ErrBlocked`.
   For a Cloudflare check, it says so and waits up to two minutes for you to
   clear it in the window.
2. **Title.** A click, then real key events, in one burst. The editor does
   not see text put in any other way: `Input.insertText` leaves its
   placeholder in place, and nothing is saved. The tool then reads the title
   back, and stops before the body if it is not the article's.
   - Typed one key at a time instead, a long title was still going when
     Medium created the story, and its first 44 characters were lost
     (2026-10-11).
3. **Text.** A click in the body, then one `paste` event with the HTML,
   dispatched to the editor. Medium's own paste handler builds its paragraphs
   from it and saves them. Each image is a placeholder paragraph in the paste,
   `san-medium-image-<n>`.
4. **Each image.**
   - The placeholder is selected and deleted with a real Backspace, which
     brings up the editor's + button beside the empty paragraph.
   - The tool clicks + and then "Add an image". Chrome hands the file chooser
     to the tool (`Page.setInterceptFileChooserDialog`), which gives it the
     file.
   - Once Medium has uploaded the image and saved, the alt text is pasted into
     the image toolbar's "Alt text" box, and the caption into the caption line.
     Each paste happens only when the right field has the caret, so nothing can
     spill into the title or the text.
5. **Save.** Done when the page has had nothing unsaved for three seconds in a
   row (its `beforeunload` handler is checked) and the status reads "Saved".
6. **Check.** The draft is reloaded and compared with the article: the title,
   the subtitle, the number of images, each image's upload, alt text and
   caption, no placeholder left, and the last paragraph present. Medium's
   curly quotes count as straight ones. A reload would stop at Medium's "leave
   this page?" if anything were unsaved; the tool always answers "stay".

Every click waits until its target has stopped moving, because Medium's menus
slide and scale into place.

## Tests

```
go test ./...
```

The unit tests cover parsing and the HTML Medium is given:

- title, subtitle and frontmatter
- image and caption pairing, and missing or remote images
- heading sizes, quotes, loose lists, and dropped inline images
- the one-paste body with its placeholders
- all articles in `docs/articles`, each with a subtitle and every image
  captioned

There is no offline test of the browser part. Here is what was checked
against the real editor on 2026-10-11:

The test article held every block type: two images, a list, a numbered list,
a quote and a Go code block.

- **Default mode (the tool starts Chrome): works.** On a fresh profile, the
  draft passed the tool's reload check in 24 s, and Chrome was closed
  afterwards.
- **`-remote`: works.** Same result, against a Chrome started by hand with the
  same flags. The draft also looked right to the eye.
- **`login` and `status`** work, as does the "profile is already open" error.
  After `login`, Chrome was not signed in and had no extensions, though the
  Medium sign-in had used Google.
- **`drafts`, `get` and `delete`** were checked on the test draft above.
  - The list (4.6 s) and the read (2.2 s) matched the draft.
  - `delete` asked first, deleted it after "y" (9 s), and its reload check
    passed. The next `drafts` listed none, and deleting it again gave "no such
    draft".
  - `DraftID` has unit tests.
- **`schedule`, `scheduled` and `unschedule`** were checked on a test draft,
  with your OK, 30 days ahead.
  - `schedule` read the time back as asked and passed its check (9.9 s).
  - `scheduled` listed it as "Nov 10, 9:00 AM".
  - `unschedule` brought it back to Drafts (7.6 s), twice.
  - The test draft was then deleted.
  - Reading the page's time, offsets included, has unit tests.
  - `draft -at` was not run as one command. It is `draft` followed by
    `schedule` on the same browser, and each was checked on its own.
- **The page-based sign-in check and the arrow-key topics** were checked
  later that day, on the `basic` profile, with the Plain start.
  - `draft` of a two-line test article took 31 s and passed its reload check.
    Its short title was typed key by key then; a long one later lost its
    start that way, so the title went back to one burst.
    `delete` removed it (28 s), and `drafts` no longer listed it.
  - `schedule` of a real draft, 30 days ahead with four topics, took 40 s.
    `scheduled` listed it as "Nov 10, 9:00 AM", and `unschedule` (26 s) made
    it a draft again. The draft was then deleted, as you asked.
  - Two runs before the arrow keys stopped at the topic "Programming". A
    diagnostic showed that a click on a suggestion sent no request, while the
    down arrow and Enter sent Medium's `SetPostTagsMutation`.

## Limits

- **Medium's markup.** The selectors are Medium's classic editor
  (`.postArticle-content`, `graf--*`, `data-action="inline-menu-image"`, …),
  and are kept in `editor.go` and `editor.js`. A redesign of the editor breaks
  the tool with `ErrEditor`, which names the step that failed.
- **Cloudflare.** Cloudflare in front of Medium answers "Sorry, you have been
  blocked" to a Chrome that chromedp launched itself (`NewExecAllocator`,
  working in Chrome's first tab).
  - In such a run the editor opens and takes the paste, but Medium's saves are
    refused. Every later page load is then blocked.
  - The block stays with the Medium sign-in session. Clearing Cloudflare's
    cookies and waiting 10 hours did not lift it, and a fresh profile on the
    same connection was still let in. The fix is to sign in again on a new
    profile.
  - It happened 3 times out of 3 that way, and never in 5 runs through a
    Chrome started as a plain process and attached to. That is why `New`
    starts Chrome itself and attaches.
  - What exactly Cloudflare sees is not known.

  Write drafts one at a time anyway. On the block page the tool stops with
  `ErrBlocked`.
- **Not done by the tool:**
  - publishing at once (only scheduled)
  - submitting to a publication
  - the preview title, subtitle and image
  - the canonical URL
  - removing topics
  - updating an existing draft's text (a draft is always new)
- **Time zone.** The picker uses Chrome's time zone, and the tool assumes it is
  the computer's. If they differ, the read-back check stops the schedule
  rather than setting a wrong time.
- **Typing speed.** The title is typed as one burst of key events, and
  topics key by key. Everything else is pasted.
- **Code block languages.** Medium guesses the language of a pasted code block
  and ignores the markdown's (`go`). The test article's Go block was guessed as
  "Less". `get` shows that label, "Auto (Less)", as a line of the text after
  the code.
- **The stories page** has generated class names, so it is read by structure:
  table rows, their `/p/<id>/edit` links, and the "Toggle actions menu"
  button. A redesign of that page breaks `drafts` and `delete` with
  `ErrEditor`, never with a wrong deletion. `delete` only acts on the row
  whose link carries the draft's id.
- One draft at a time per `Client`.
