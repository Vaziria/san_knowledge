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

## Checking the schedule

Both listings only read; they post and change nothing. Run them from the repo root:

```powershell
golang\packages\san_youtube\bin\san_youtube.exe videos -profile basic   # Heri Santoso Dev: Videos and Shorts, with "Scheduled" and the date
golang\packages\san_tiktok\bin\san_tiktok.exe posts -profile basic      # @archetipe_: Studio's Posts, a scheduled one shows its publish time
```

A day is free on a platform when neither list shows a post published or scheduled on it.

## Posting

1. List both platforms (above) and choose the first free day for each.
2. Write the post files next to the video: `youtube.md` (title, description, tags, `at`) and
   `tiktok.md` (caption, `at`). Formats: [san_youtube README](../golang/packages/san_youtube/README.md#the-post)
   and `san_tiktok post -h`.
3. Show the user the preview (`upload -dry-run` / `post -dry-run`) and wait for approval.
4. Upload: `san_youtube upload -profile basic youtube.md`, `san_tiktok post -profile basic tiktok.md`.
5. List both platforms again to confirm the new post sits on its day.

TikTok runs a "Content check lite" of about 10 minutes after the upload; `san_tiktok post`
waits for it before pressing Schedule, so a post takes about 12 minutes.

## Moving a scheduled post

Neither Studio lets a scheduled time be changed from the tools, so a post that sits on a taken
day is deleted and scheduled again on a free one:

- TikTok: `san_tiktok delete -profile basic -caption "<start of the caption>" -when "Oct 16, 7:00 PM"`
  deletes the one post whose row holds both (the date as the Posts list shows it); it refuses when
  none or several match. Try it with `-dry-run` first: it shows the row and the "…" menu and
  clicks nothing. Then run `post` again with the new `at`.
- YouTube: change the date in Studio (the video's Visibility), by hand.

The first move (2026-10-15): the Blender material tutorial on TikTok went from 16 Oct 19:00,
which already had the radio post, to 18 Oct 19:00.
