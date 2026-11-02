# Social Posting Plan

How videos go out on YouTube and TikTok: one a day, checked against what is already scheduled,
and never public at once.

## Rules

- **One video a day on each platform.** YouTube and TikTok each get at most one video per
  calendar day (WIB), counting posts made by hand as well as by the tools.
- **Check the schedule before posting.** Before scheduling a video, list what the platform
  already has published and scheduled, and pick a day that has nothing yet (the user,
  2026-10-15: "before post check the last schedule first, our plan is one video in one day").
- **Never public at once.** A video is either scheduled, or saved as Private (YouTube) / Only you
  (TikTok). Both tools refuse anything else.
- **Show the post first.** Title, caption, hashtags and the day go to the user for approval
  before any upload.
- **Texts.** Indonesian, a catchy title (not a file name), hashtags first, no emoji or icons.
- **YouTube is long-form only.** No Shorts: they hurt the channel's organic and continuity
  traffic (the user, 2026-10-21: "for youtube, we dont use short ... we use long about 3
  minute"). A YouTube video is about 3 minutes and **16:9**, because YouTube counts any vertical
  or square video up to 3 minutes as a Short. The 9:16 cuts go to TikTok only.

## Every platform (2026-11-02)

The user wants every platform kept up to date daily, with an AI preparing the posts and the user
approving them. The week's schedule and its state live in `docs/social_plan/<monday>.md`.

- **Daily, 19:00 WIB:** Threads (@archetipe__), through `san_remote` in the user's own Chrome.
- **Articles, 3 a week, 09:00 WIB:** Medium and LinkedIn get the same three articles, LinkedIn a
  day after Medium (the user, 2026-11-02: "for medium and linkedin, 3 article per week is okay").
- **When a video is ready:** TikTok (9:16) and YouTube (16:9, long only), still one a day at most.
- **Not used:** Facebook ("leave Facebook, that's account not ready", 2026-11-02). X ("leave the
  X", 2026-11-02): X's upload API answers "Your account is suspended and is not permitted to
  access this feature" for @archetipe_. The user says it was suspended before any automation.
- **Instagram is back** (the user, 2026-11-02: "manage Instagram too, it's okay"). Scheduling
  goes through Meta Business Suite, which must be switched to @archetipe__, not the Uni_Q Page.
- **No Blender content** ("I don't like blender", 2026-11-02).
- **Approval:** the drafts for a day go to the user on Telegram at 20:00 WIB the evening before.
  Nothing is scheduled before the user approves it.
- **The browser server:** `san_remote serve --idle 15m`, which stops by itself after 15 minutes
  unused (the user, 2026-11-02). Start it again before the next command.
- **One browser: `san_remote` for every platform** (the user, 2026-11-02: "why you always open
  different browser and not using San remote?"). Threads, Instagram, TikTok, Medium, LinkedIn
  and YouTube all go through `san_remote <site> ...` in the user's own Chrome. The chromedp tools
  below (`san_youtube` upload with `-profile basic`; `san_tiktok`, `san_medium`, `san_linkedin` were deleted on 2026-11-02) open a
  second Chrome and are no longer used for posting. Checked on 2026-11-02, the user's Chrome is
  signed in as @archetipe__ (Threads, Instagram), @archetipe_ (TikTok), Heri Santoso (@archetipe)
  on Medium and Heri Santoso on LinkedIn. YouTube's `status` timed out in a background tab; check
  its account before the first upload.

## Checking the schedule

Every listing only reads; it posts and changes nothing. All of them go through `san_remote` in the
user's own Chrome (`serve` must be running). Run them from the repo root:

```powershell
golang\apps\san_remote\bin\san_remote.exe youtube videos      # Heri Santoso Dev: KIND, VISIBILITY, DATE, URL, TITLE
golang\apps\san_remote\bin\san_remote.exe tiktok posts        # @archetipe_: WHEN, LENGTH, PRIVACY, CAPTION
golang\apps\san_remote\bin\san_remote.exe threads scheduled   # AT, TEXT
golang\apps\san_remote\bin\san_remote.exe instagram scheduled # WHEN, KIND, CAPTION (Business Suite on @archetipe__)
golang\apps\san_remote\bin\san_remote.exe medium scheduled    # ID, AT, TITLE
golang\apps\san_remote\bin\san_remote.exe linkedin scheduled  # WHEN, TITLE
```

A day is free on a platform when its list shows no post published or scheduled on it.

## Posting

Only through `san_remote` (the user, 2026-11-02: "don't use old tools, use San remote"). The old
chromedp tools `san_tiktok`, `san_medium` and `san_linkedin` were deleted on 2026-11-02 (they are
in git history). `san_youtube` stays, because the radio and the animation's chat bridge read
YouTube live chat with it, but its uploader is not used: uploads go through `san_remote youtube`.

1. List the platforms (above) and choose the first free day for each.
2. Write the post files next to the video: `youtube.md` (title, description, tags, `at`) and
   `tiktok.md` (caption, `at`). Formats: the YouTube and TikTok sections of
   [san_remote's docs](../golang/apps/san_remote/docs/readme.md#youtube).
3. Check the account with `san_remote <site> status`, show the user the preview
   (`--dry-run`), and wait for approval.
4. Upload: `san_remote youtube upload youtube.md`, `san_remote tiktok post tiktok.md`.
5. List the platforms again to confirm the new post sits on its day.

TikTok runs a "Content check lite" of about 10 minutes after the upload, and the post waits for it
before Schedule is pressed, so a TikTok post takes 10 to 15 minutes. Chrome comes to the front
while it works.

## Moving a scheduled post

Neither Studio lets a scheduled time be changed from the tools, so a post that sits on a taken
day is deleted and scheduled again on a free one:

- TikTok: `san_remote tiktok delete --caption "<start of the caption>" --when "Oct 16, 7:00 PM"`
  deletes the one post whose row holds both (the date as the Posts list shows it); it refuses when
  none or several match. Then run `post` again with the new `at`.
- YouTube: change the date in Studio (the video's Visibility), by hand.

The first move (2026-10-15): the Blender material tutorial on TikTok went from 16 Oct 19:00,
which already had the radio post, to 18 Oct 19:00.
