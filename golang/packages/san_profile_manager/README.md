# san_profile_manager

The Chrome profiles that the san_* tools drive, kept in one place, and Chrome
started on them the way those tools need it. `san_medium`, `san_tiktok` and
`san_gpt` use it, and so should every tool that drives a signed-in browser.

A profile is a Chrome user-data folder, and one profile can be signed in to
Medium, TikTok and any other site at once. So every tool uses the same
profile unless told otherwise. Sign in to each site once, with any tool's
`login`, and all tools see it.

Module `github.com/wargasipil/san_profile_manager`, Go 1.26. It needs
[chromedp](https://github.com/chromedp/chromedp), plus Chrome, Edge or Chromium
on the machine.

## Where profiles live

Profiles live in the workspace, in `browser_profiles/` at its root
(`D:\research\marketing\browser_profiles`). Git ignores everything in it but
its `.gitkeep`, which keeps the folder in a fresh clone: the profiles hold
signed-in sessions, and must never be committed.

| Profile | Folder |
|---|---|
| `""` or `"default"` | `browser_profiles/default` |
| a name, such as `"work"` | `browser_profiles/work` |
| a path (absolute, or with a `/` or `\` in it) | that folder |

`Dir(profile)` resolves these. `Root()` finds `browser_profiles`: the nearest
one looking up from the working directory, then from the program's folder.
So a tool built into the workspace (`golang/packages/<tool>/bin`) finds it
from anywhere, even when run from outside the workspace. Without one,
`Root()` gives `ErrNoRoot`, and a profile given as a folder still works.

A tool's `-profile` flag takes the same values, so a second account on a site
is `-profile work`, signed in with `login -profile work`.

## Use from a tool

```go
import san_profile_manager "github.com/wargasipil/san_profile_manager"

// Sign in, once: a plain Chrome on the profile at the site's sign-in page.
// Returns when the person closes that window.
err := san_profile_manager.Login(ctx, san_profile_manager.Config{}, "https://www.tiktok.com/login")

// Work: a tab of the tool's own in Chrome on the profile.
b, err := san_profile_manager.Start(ctx, san_profile_manager.Config{Profile: "default"})
if err != nil {
	return err // e.g. san_profile_manager.ErrProfileInUse
}
defer b.Close()
err = chromedp.Run(b.Tab(), chromedp.Navigate("https://www.tiktok.com/tiktokstudio/upload"))

// Signed in? Read the cookies; no page is loaded.
cs, err := b.Cookies(ctx) // []Cookie{Name, Domain, Value}, for every site

// More tabs in the same Chrome, such as one per conversation (san_gpt).
tab, closeTab, err := b.NewTab()
defer closeTab() // b.Close also closes any left open
```

`Config`:

| Field | What it does |
|---|---|
| `Profile` | a name or a folder, as above; empty is `default` |
| `Remote` | attach to a Chrome you started with `--remote-debugging-port`, such as `http://127.0.0.1:9222`, instead of using a profile. That Chrome is never closed. |
| `ExecPath` | the browser binary, when `FindChrome` does not find it |
| `Headless` | start Chrome without a window, for reading cookies only |

The tool keeps its site-specific parts: the sign-in URL, the session cookie
that means "signed in", and what it does in the tab. In `san_medium` and
`san_tiktok` that is all of `client.go` that is left.

## How Chrome is run

- **Login** starts Chrome on the profile as a plain process, with no
  automation and no debugging port, because Google's sign-in, which most sites
  offer, turns away a browser that is being driven.
- **Start** starts Chrome as a plain process with `--remote-debugging-port=0`,
  as a shortcut would start it. Chrome picks the port and writes it into the
  profile's `DevToolsActivePort`. The tool then attaches over that port and
  works in a tab of its own; the blank tab Chrome starts with is closed.
  - chromedp never launches Chrome itself. A Chrome it launches carries
    automation flags, and Medium's Cloudflare refused every such browser
    (san_medium's README, "Cloudflare").
  - Background throttling is off, so a window behind other windows keeps
    working.
  - Extensions are off for the run, even if the profile has some installed.
- In the tool's tab, a page's "leave this page?" question is answered
  "stay", so nothing can throw away unsaved work. Other dialogs are accepted.
- Chrome sync is off (`--disable-sync`), and so is "Allow Chrome sign-in".
  The library writes that setting into the profile before every start. It is
  `signin.allowed_on_next_startup`, which Chrome copies into `signin.allowed`
  at each start. Signing in to a site with Google then signs only that site
  in, not Chrome. Your extensions, history and settings from other browsers
  stay out of the profile, and its activity stays out of your Google account.

### Two tools at once

Only one Chrome can have a profile open. When a tool already has Chrome
running on it, `Start` attaches to that Chrome and opens a tab of its own, so
`san_medium` and `san_tiktok` can run at the same time.

- It finds that Chrome through the profile's lock file and
  `DevToolsActivePort`, and checks that the port answers. Chrome leaves the
  file behind when it exits, so the file alone proves nothing.
  - A Chrome that has only just started gets 5 seconds to open its port.
  - A headless Chrome, which another tool's `status` runs for a moment, is
    waited out for up to 15 seconds when a window is wanted.
  - If that Chrome closes meanwhile, a new one is started.
- `Close` closes the tool's tab, and those `NewTab` opened, and waits until
  Chrome no longer lists them.
  Then, if no tab is left, it closes Chrome: the last tool out closes it, and
  a tab you opened yourself in that window keeps it open. Counting after
  closing also catches two tools that finish at the same moment.
- The window `Login` opened, or a Chrome you started on the profile yourself,
  has no debugging port. Then `Start` gives `ErrProfileInUse` ("the profile is
  already open in another Chrome window").

## Tests

```
go test ./...
```

- `Dir` for names and paths, finding `browser_profiles` from a folder deep
  below it, reading `DevToolsActivePort`, and the sign-in setting written into
  a new profile, an existing one, and one in use.
- `TestShare` runs a real headless Chrome on a fresh profile. A second
  `Start` attaches to it and opens two more tabs, closing one, the first
  `Close` leaves Chrome running for the second, and the second `Close`
  closes the tab left open and stops Chrome. It is skipped with `-short` or
  without Chrome. It passed 20 times in a row on Windows on 2026-10-11, with `NewTab`.
- Checked with the tools on 2026-10-11: `san_medium status` and
  `san_tiktok status`, started together, both used the shared profile, and
  Chrome was gone afterwards. `san_gpt`'s test against a fake chat app in
  real Chrome passed with its conversation tabs opened by `NewTab`.

## Moving a profile from before

`san_medium` used to keep its profile in `%AppData%\san_medium\profile`.
Moving a profile folder keeps its sign-ins, even to another drive: that one
was moved to `browser_profiles/default` on 2026-10-11, and `san_medium status`
still read it as signed in to Medium, from the workspace root, from a package
folder and from outside the workspace.
