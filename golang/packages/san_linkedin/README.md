# san_linkedin

Write a markdown article into LinkedIn as an article draft, schedule it to be
published, and list, read and delete drafts, from Go or the command line. It
is [san_medium](../san_medium/README.md) for LinkedIn.

LinkedIn's API publishes articles only for approved partner apps, so
`san_linkedin` uses the article editor, like a person. It drives a real Chrome
that is logged in to LinkedIn. It opens "Write article" from the feed, types
the title, uploads the cover, pastes the text, and puts each image in its
place with the editor's own image button. Then it fills in the image's alt
text and caption. LinkedIn saves the draft itself. The tool then reloads the
draft and checks it against the article. Publishing at once is never done:
an article goes out only at a time set with `schedule`.

Module `github.com/wargasipil/san_linkedin`, Go 1.26. It needs
[chromedp](https://github.com/chromedp/chromedp),
[goldmark](https://github.com/yuin/goldmark) and
[san_profile_manager](../san_profile_manager/README.md) (a sibling folder,
wired in with a `replace` line in `go.mod`), plus Chrome, Edge or Chromium on
the machine.

## The tool

```powershell
pwsh build.ps1                                   # vet, test, build bin/san_linkedin.exe and bin/san_linkedin
bin\san_linkedin.exe login -profile basic        # once: sign in to LinkedIn in the window that opens, then close it
bin\san_linkedin.exe status -profile basic       # signed in (D:\research\marketing\browser_profiles\basic)
bin\san_linkedin.exe draft -dry-run article.md   # how the article will be cut; opens nothing
bin\san_linkedin.exe draft -profile basic -cover assets\cover.png article.md   # make the draft; prints its URL
```

LinkedIn is signed in on the `basic` profile (2026-10-13), the one san_medium
uses too. The examples below leave `-profile basic` out.

```
== writing "Uji san_linkedin 2 (hapus saja)" into a new draft
== opening LinkedIn's feed
== reading the feed for a moment
== opening a new article with "Write article"
== typing the title
== cover image: delivery-path-anatomy.png
== pasting the text (1 KB)
== image 1 of 2: delivery-path-anatomy.png
== image 2 of 2: invite-join-sequence.png
== waiting for LinkedIn to save
== reloading the draft to check it
https://www.linkedin.com/article/edit/7515804761766535168/
```

The draft URL goes to stdout and the status lines go to stderr. If a step
fails after LinkedIn has created the draft, the URL is still printed. That
draft holds everything written up to the failure.

`-cover <image>` sets the cover, which LinkedIn shows above the title and in
the post that shares the article. Without it the article has no cover.

Drafts already on LinkedIn:

```powershell
bin\san_linkedin.exe drafts                      # 7515804761766535168  Last modified on Oct 13, 2026    Uji san_linkedin 2 (hapus saja)
bin\san_linkedin.exe drafts -json                # {"ID":"7515804761766535168","URL":"…/article/edit/7515804761766535168/","Title":"…","Modified":"Last modified on Oct 13, 2026"}
bin\san_linkedin.exe get 7515804761766535168     # status, title, cover, images (URL, alt, caption), then the text
bin\san_linkedin.exe get -json <id | url>
bin\san_linkedin.exe delete 7515804761766535168  # asks: Delete the draft "…" (7515804761766535168)? This cannot be undone. [y/N]
bin\san_linkedin.exe delete -yes <id | url>      # no question, for scripts
```

A draft can be named by its id or by its edit URL,
`https://www.linkedin.com/article/edit/<id>/`.

- **`drafts`** reads the Drafts tab of the editor's Articles dialog (Manage,
  then Drafts), as LinkedIn orders it. It scrolls the list until no more rows
  come.
- **`get`** opens the draft by clicking its title in that tab, and reads it,
  changing nothing.
- **`delete`** deletes the draft the way a person does in that tab: the
  row's menu, "Delete", then "Delete" in the question LinkedIn asks in the
  row itself ("Delete draft? You can't undo this action.").
  - It returns once a reload of the tab no longer lists the draft.
  - Only rows of the Drafts tab are looked at, so a scheduled or published
    article is never touched; it gives "no such draft".

### Scheduling

LinkedIn publishes a scheduled article by itself, at the time set, whether
this computer is on or not. The tool sets that time through the editor's
Next, as a person does.

```powershell
bin\san_linkedin.exe schedule -at "2026-11-20 09:00" -text "Bagian 1 dari seri VPN." <id | url>
bin\san_linkedin.exe scheduled                   # Fri, Nov 20 at 9:00 AM   Uji san_linkedin 2 (hapus saja)
bin\san_linkedin.exe unschedule "Uji san_linkedin 2 (hapus saja)"   # back to the drafts, same id
bin\san_linkedin.exe draft -at "2026-11-20 09:00" -text "…" -cover c.png article.md   # make the draft and schedule it
```

- **`-at`** is local time, as `"2006-01-02 15:04"`, or RFC 3339 with its own
  offset. It must be at least 15 minutes ahead and on a quarter hour: the
  time list offers 12:00 AM to 11:45 PM in 15-minute steps.
- **`-text`** is the post that shares the article when it goes out ("Tell
  your network what your article is about…"). A line break in it starts a new
  paragraph. Without it the post is the article alone.
- **`unschedule`** takes the article's **title**, not its id: the Scheduled
  tab shows no ids. When two scheduled articles share a title, `-when "Fri,
  Nov 20 at 9:00 AM"` (as `scheduled` prints it) picks one. It prints the
  draft's id, which is the id the article had before.

How `schedule` fills the share box:

1. The draft is opened from the Drafts tab, and the tool waits until
   LinkedIn has saved it. A Next pressed while a draft just opened is still
   saving only shows "Sorry, your article is still saving."; then the tool
   waits again and presses Next once more, up to three times.
2. Next opens the share box. `-text` is typed into its text box key by key.
   - A line break is one Enter keyDown that carries its `\r`. chromedp's
     Enter adds a separate char event, and that made two empty lines and sent
     the caret to the start of the box (2026-10-13).
   - Pasting does not work there: the box ignores a paste event that does
     not come from the clipboard.
3. The clock ("Schedule post") opens the schedule view.
   - The date box opens a calendar. Its "Next month" arrow is clicked until
     the month shows, then the day, found by its label ("Friday, November 20,
     2026.").
   - The time box opens the list of times, and the time is clicked
     ("9:00 AM").
4. The schedule view states the time it took: "Fri, Nov 20, 9:00 AM Western
   Indonesia Time, based on your location". The tool reads it back and checks
   the day, the time, and the zone. The zone has to be the browser's own zone
   name at that time, and the browser's offset has to be this computer's.
   Any difference stops the tool there, and nothing is scheduled.
5. Next goes back to the share box, which now says "Posting Fri, Nov 20 at
   9:00 AM" and whose main button reads Schedule instead of Publish. Both are
   checked, and the post's text too.
6. "Schedule" is pressed. LinkedIn says "Your article post has been
   scheduled". The proof is the article in the Scheduled tab at that time,
   and no longer in the Drafts tab.

The tool only ever presses a button that reads exactly "Schedule". It never
presses "Publish", nor the Scheduled tab's "Post now".

`unschedule` clicks the article's title in the Scheduled tab. LinkedIn then
asks in the row: "To edit this scheduled article, move it to drafts." The
tool presses "Move to drafts". The proof is the article back in the Drafts
tab and gone from the Scheduled tab. LinkedIn's other way to change a
schedule, the row menu's "Modify post schedule", only moves the time.

Those proofs read the lists at most 4 times, 5 seconds apart, never in a
tight loop.

Flags for every command:

| Flag | What it does |
|---|---|
| `-profile <name \| dir>` | the Chrome profile: a name kept by san_profile_manager, or a folder. The default, `default`, is the profile every san_* tool shares, in `browser_profiles/` at the workspace root, which git ignores. LinkedIn is signed in on `basic`. |
| `-remote <url>` | use a Chrome you started with `--remote-debugging-port`, such as `http://127.0.0.1:9333`, instead of launching one. That Chrome must be signed in to LinkedIn. Only the tab the tool opens is closed afterwards. |
| `-chrome <path>` | the browser binary, when it is not found in the usual places |

`draft` also takes `-timeout` (default 10m).

Set `SAN_LINKEDIN_DEBUG=1` to see how long each step took, and after each
image step a line with the article's blocks and images, as the page has
them.

### The profile

The tool uses the Chrome profile the san_* tools share, apart from your
everyday one; [san_profile_manager](../san_profile_manager/README.md) keeps it
and starts Chrome on it. `login` opens Chrome on that profile at LinkedIn's
sign-in page, with no automation attached. Sign in there, then close the
window. The session stays in the profile until LinkedIn logs you out.
`status` reads the profile's cookies (LinkedIn's `li_at`) and loads no page.

While a command runs:

- A Chrome window opens and fills itself in. Leave it alone: a click or a
  keystroke in it lands in the article. It may sit behind other windows:
  Chrome is started with san_profile_manager's usual flags, which keep a
  covered window working. The plain start san_medium needs for Medium's
  Cloudflare freezes a covered window, and LinkedIn does not need it; every
  run of 2026-10-13 and 14 was in a Chrome started the usual way.
- If another san_* tool already has Chrome running on the profile, the tool
  works in a tab of its own in that Chrome.
- A Chrome started on the profile with a fixed `--remote-debugging-port`
  may not be found that way: one started with port 9333 left the profile's
  `DevToolsActivePort` naming an older port (2026-10-13), and the tool gave
  "the profile is already open in another Chrome window". Give such a
  Chrome with `-remote` instead.

## The package

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	san_linkedin "github.com/wargasipil/san_linkedin"
)

func main() {
	ctx := context.Background()

	a, err := san_linkedin.ParseFile("docs/articles/topic/article.md")
	if err != nil {
		log.Fatal(err) // e.g. san_linkedin.ErrNoTitle, or a missing image file
	}
	if err := a.SetCover("docs/articles/topic/assets/cover.png"); err != nil {
		log.Fatal(err)
	}

	c, err := san_linkedin.New(ctx, san_linkedin.WithProfile("basic")) // or san_linkedin.WithRemote("http://127.0.0.1:9333")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	d, err := c.NewDraft(ctx, a, san_linkedin.WithProgress(func(s string) { log.Println(s) }))
	if d != nil {
		fmt.Println(d.URL) // https://www.linkedin.com/article/edit/<id>/
	}
	if err != nil {
		log.Fatal(err)
	}

	at := time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)
	if err := c.Schedule(ctx, d.ID, at, san_linkedin.WithText("Bagian 1 dari seri VPN.")); err != nil {
		log.Fatal(err)
	}
}
```

`Login(ctx, opts...)` and `LoggedIn(ctx, opts...)` are the `login` and
`status` commands.

```go
ds, err := c.Drafts(ctx)                         // []Draft{ID, URL, Title, Modified}
d, err := c.GetDraft(ctx, "7515804761766535168") // *DraftContent{Status, Title, Cover, Text, Images}
err = c.DeleteDraft(ctx, ds[0].ID)               // cannot be undone; ErrDraftNotFound if not a draft
ss, err := c.ScheduledArticles(ctx)              // []Scheduled{Title, When}
d, err = c.Unschedule(ctx, "Title", "")          // the draft again; ErrNotScheduled if no such title
id, err := san_linkedin.DraftID("https://www.linkedin.com/article/edit/7515804761766535168/")
```

`Schedule` returns `ErrTooSoon` for a time less than `MinLead` (15 minutes)
ahead, `ErrNotQuarterHour` for a time off the quarter hour, and
`ErrDraftNotFound` for anything that is not a draft.

The calls on one `Client` run one at a time, in its one tab, so a program can
write, schedule, list and delete in a row without starting Chrome again.

### What the markdown becomes

The article is read the way [the article guideline](../../../docs/articles/guideline.md)
writes it, as in san_medium:

- **Title:** the first `# ` heading, typed into the title. LinkedIn keeps 150
  characters; a longer title fails before any browser opens.
- **Subtitle:** a paragraph right under the title that is only italic text.
  LinkedIn has no subtitle field, so it opens the body as an italic
  paragraph.
- **Headings:** the editor has two sizes, Heading and Subheading. `##`
  becomes a Heading and `###` (and deeper) a Subheading.
- **Text, lists, quotes, links, code blocks, `---`:** pasted as HTML into
  the editor (ProseMirror), which converts it itself. `---` becomes the
  editor's divider. Each paragraph of a quote becomes a quote of its own.
- **Images:** an image on a paragraph of its own (`![alt](assets/x.png)`) is
  uploaded from the local file. Its alt text becomes the image's alt text
  (up to 1,000 characters). An italic paragraph right after it becomes the
  caption (up to 250).
- **Frontmatter** is skipped.

Parsing fails, before any browser opens, when there is no title, an image
file is missing, or a title, alt text or caption is too long. A remote image
(`https://…`) is refused too: save it next to the article. `Article.Warnings`
names what will not survive: an image inside a line of text (it is left
out), and links to local files.

What LinkedIn drops on save (2026-10-13): inline `code` becomes plain text,
since LinkedIn's articles have no inline code, and a code block loses its
language.

### How a draft is filled

1. **Feed, then Write article.** The tool opens LinkedIn's home page, which
   is the feed for a member. It waits for the feed's "Write article" link
   and for the feed's requests to settle (for up to 20 seconds, since the
   feed keeps polling), then reads it for 5–10 seconds. Then it clicks Write
   article, once more when the tab has not moved on 20 seconds later.
   - When two clicks open nothing, the tool opens the link's address,
     `/article/new/`, as a person opens a link that does not respond. In a
     Chrome just started, a click loaded the editor into the feed's hidden
     iframe but the feed never showed it, three clicks in a row
     (2026-10-14).
   - Reached this way, the editor runs in a same-origin iframe
     (`/preload/?_bprMode=vanilla`) that fills the page. The feed loads that
     iframe ahead of time, transparent and behind the page, so the tool
     looks inside it only once it is shown. Loaded by its own URL, the editor
     is the page itself. The helpers in `editor.js` work on either.
   - LinkedIn's pages fetch `chrome-extension://` URLs about every 100 ms,
     looking for extensions. Those are left out of every wait for requests.
2. **Title.** A click, then real key events, in one burst. LinkedIn makes the
   draft once the title is in, and the tab moves to its edit URL.
3. **Cover.** "Upload from computer" opens a file chooser, which Chrome hands
   to the tool (`Page.setInterceptFileChooserDialog`). LinkedIn's media
   dialog opens; its Next puts the cover in.
4. **Text.** A click in the body, then one `paste` event with the HTML,
   dispatched to the editor. ProseMirror's paste handler builds the blocks.
   Each image is a placeholder paragraph in the paste,
   `san-linkedin-image-<n>`.
5. **Each image.**
   - The placeholder is clicked, selected and deleted with a real Backspace,
     which leaves the caret in the empty paragraph.
   - The toolbar's image button ("Add image or video") opens a file chooser,
     then the media dialog. Its "Alternative text" button opens the alt text
     box, where the alt text is typed and added. Next puts the image in.
   - The caption box under the image is clicked, and the caption goes in as
     one insertion (`Input.insertText`). Each key typed there redraws the
     image and the caret leaves the box: typed key by key, only the first
     key stayed, and the rest of one caption landed in the image before.
   - The editor leaves an empty paragraph after the image. A Backspace in
     it removes it, and the tool checks that it stays gone.
6. **Save.** Five seconds after the last change (LinkedIn saves about 2
   seconds after one), the tool waits until no save request
   (`/voyager/api/voyagerPublishing…`) has run for three seconds and the
   status in the corner reads "Draft - saved".
7. **Check.** The draft is reloaded, as F5 does: images show from the
   computer until then, so only a reload proves they were uploaded. It is
   compared with the article: the title, the cover, the subtitle, no
   placeholder left, the number of images, each image's upload, alt text and
   caption, no empty paragraph after one, and the last paragraph present.

Every click moves the mouse there first, along a slightly bent path, and
waits until its target has stopped moving. The element is found again each
time the tool looks, since LinkedIn may draw a button again after it was
found.

## Tests

```
go test ./...
```

The unit tests cover:

- parsing: title, subtitle, frontmatter, image and caption pairing, missing
  or remote images, and the length limits, counted in UTF-16 units as the
  page counts them
- the HTML LinkedIn is given: heading sizes, quotes, loose lists, dividers,
  code blocks without their last newline, dropped inline images, and the
  one-paste body with its placeholders
- all articles in `docs/articles`, each with a subtitle and every image
  captioned
- the cover, `DraftID`, the session cookie, reading the schedule view's
  stated time, and the checks `Schedule` makes before a browser opens

There is no offline test of the browser part. Here is what was checked
against the real editor on 2026-10-13 and 2026-10-14, on the `basic`
profile, with `-remote` to a Chrome another tool had open there:

The test article held every block type: a subtitle, both heading sizes, a
list, a quote of two paragraphs, a Go code block and two captioned images,
plus a cover.

- **`draft`: works.** It passed its reload check in 82 s. The draft also
  looked right to the eye.
  - Before that, runs found the bugs this README describes: the feed's
    hidden iframe, the media dialog's buttons coming late, captions typed
    key by key, and a selector that counted the editor itself as an image.
- **`drafts`, `get` and `delete`** were checked on the test drafts.
  - The list (20 s) and the read (23 s) matched the drafts.
  - `delete` asked first, deleted after "y", and its reload check passed
    (about 33 s). Deleting the same id again gave "no such draft".
- **`schedule`, `scheduled` and `unschedule`** were checked on a test draft,
  with your OK, 38 days ahead, with a post text of two lines.
  - `schedule` read the time back as asked and passed its check (57 s).
  - `scheduled` listed it as "Fri, Nov 20 at 9:00 AM" (40 s).
  - `unschedule` brought it back to Drafts with the same id (48 s).
  - `draft -at` made a draft and scheduled it in one command (1 min 52 s);
    it was unscheduled the same way.
- Every test draft was deleted afterwards. The Drafts, Scheduled and
  Published tabs were then all empty: nothing was ever published.
- **Both article series, 14 articles, were drafted and scheduled** on
  2026-10-14 (one a day, Oct 15–28 at 09:00 WIB), in one browser session,
  with a pause of 1.5–2.5 minutes between articles. Each took 2–3 minutes.
  The run found two bugs, both fixed:
  - In the Chrome the tool had just started, "Write article" opened nothing,
    three clicks in a row; the tool now opens the link's address after two
    (see "How a draft is filled"). Later in the same Chrome, one click was
    enough, except once.
  - The Scheduled tab shows 10 rows, then "Show more results". The tool took
    that button for a list still loading, and waited out the 11th
    article's proof, although the article was scheduled. It now clicks the
    button, and `scheduled` listed all 14.

## Limits

- **LinkedIn's markup.** The selectors are those of LinkedIn's article editor
  (Ember, with ProseMirror for the body) and its Articles dialog
  (`#article-editor-headline__textarea`, `.article-editor-figure-image`,
  `.article-editor-article-management-modal`, `data-test-icon="image-medium"`,
  …), and are kept in `editor.go` and `editor.js`. A redesign breaks the tool
  with `ErrEditor`, which names the step that failed.
- **Bot detection.** LinkedIn's pages load HUMAN (PerimeterX) and invisible
  reCAPTCHA, and its terms do not allow automated use. The tool moves like a
  person: the feed first, clicks only, the mouse moved to each target, pauses
  before clicks. Still, use it for your own articles, one at a time, and not
  in bursts. A security check is reported; clear it in the window.
- **Not done by the tool:**
  - publishing at once (only scheduled)
  - posting to a newsletter or as a company page
  - changing who the post is for (it stays "Anyone")
  - updating an existing draft's text (a draft is always new)
- **Scheduled articles** are known by title: LinkedIn's Scheduled tab shows
  no ids.
- **Time zone.** LinkedIn schedules "based on your location". The tool checks
  that this is the browser's zone, at the computer's offset, and stops
  otherwise rather than set a wrong time.
- **Times** are on the quarter hour, and only as far ahead as LinkedIn's
  calendar goes.
- **Typing.** The title, alt text and post text are typed as key events,
  captions inserted at once, and everything else pasted.
- One article at a time per `Client`.
