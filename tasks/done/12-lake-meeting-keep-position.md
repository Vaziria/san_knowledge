After: 07-lake-meeting-roaming.md

# Lake meeting: roaming keeps each new position

The user asked (2026-09-25): "in lae meeting, animal roaming, position -> roaming -> keep new position -> roaming -> keep new position, and so on". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Task 07 made the animals roam the lake's land. Read its Done section (`tasks/done/07-lake-meeting-roaming.md`), then `src/story/lake_meeting/Meeting.ts` and `Member.ts`. Before task 07, each animal had a fixed place at the meeting (`Member.home`, facing `facing`), and a stroll went out and came back to it.

## What the user wants

Roaming is a chain with no way back:

1. The animal stands where it is.
2. It roams: it walks, or sometimes runs, to a new spot.
3. It keeps that new spot: it stays there, and that is its place now.
4. It roams on from there to the next spot, keeps that one, and so on.

It never walks back to an earlier place, and nothing puts it back at a fixed place at the meeting.

## What to do

- **Check what task 07 built.** If it already works this way everywhere, this task is the check below, plus a line in the spec and in rules.md.
- **Otherwise make every move end at the new spot and keep it:**
  - the roaming itself;
  - `!walk` and `!run`;
  - a bot's own doings;
  - a turn: after its bubble, it goes on from where it is;
  - `!stop`: it stays where it stopped;
  - a kind switch, where the new figure stands where the old one was.

  A leftover `home` must not pull it back. Make its place follow it, or take `home` out for the roaming animals.
- **At each stop it keeps the heading it arrived with (default),** rather than turning to face a meeting that is no longer there.
- The host stays at the landing, as task 07 left it. An animal that leaves still walks away.
- **The spec:** say plainly in the draft `lake_meeting.md` (item 11, and item 5 for the commands) that an animal stays where it stops and walks on from there. Keep its note at the top that it is a draft for the user to correct.

Tasks 08–11 change the meeting too, and other sessions are changing `src/previews.ts` and `src/Stage.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** in Node over a long run (30 min of the meeting, with 7 bots, some viewers from test messages, and commands and comments among them). For every animal, record where each walk starts and ends:
  - each walk starts where the last one ended, with no jumps from frame to frame;
  - no walk ends back at the animal's first place, or at any old place;
  - after `!walk`, `!run`, `!stop`, a comment and a kind switch, it stays where it ended up;
  - the host never leaves the landing.
- **In a browser session,** watch a bot for a few of its walks: it walks, stays, and walks on from there.
- Add what you learned to rules.md (Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 19:13

## Done

**What task 07 already did, and what changed**

Task 07's roaming already worked as a chain. A roaming animal's walk ends where it stops, with no settling back to a place, and it keeps the heading it arrived with. The next walk starts from there, for bots and viewers alike. Commands, comments, `!stop` and kind switches all leave it where it is.

This task made that structural, and made "never back" hold:

- `Member.home` is now `null` for everyone but the host (`HOST_HOME`, set in `Meeting.ts`), and settling only happens with a home. A leftover home can't pull a roaming animal anywhere.
- `Roam.ts` remembers where each animal has stood: its first place, always, and its last 40 stands (at a spot, talking, told to stop, waiting, a way not found). A new spot keeps 1 m more than it needs from all of them. With nowhere new left within 12 m, the animal walks anywhere on the land.
- Also fixed: while a runner walks round a corner, it turns at the walk's rate.
- `lake_meeting.md` items 5, 11 and 12 say it plainly; the draft note is kept.
- rules.md Stories rule 17 has a "kept" bullet and the checks.

**How it was checked**

- `npm run typecheck`. During one run it failed on the task-08 session's work in progress in `Meeting.ts`; it was clean once that went on.
- **Numerically in Node**, six runs of 30 min: 7 bots, viewers coming and going, a comment every 25 s, commands and switches. Recording every walk over 0.5 m (600–700 a run):
  - every walk started where the last ended;
  - nothing jumped in a frame;
  - the host never left the landing;
  - no walk ended at the animal's first place.
  - In one run, an animal waiting for another stood as it passed over one of its old spots. A few walks a run end within 0.3 m of a stand older than the 40 remembered, by chance, minutes later.
- **In a real page** (headless Edge, the figures' places read each quarter second): seven bots over 90 s made 12 walks of 4–47 m. Each started from where the last ended, none went back to an earlier stand, none drifted while standing, and the host stayed at its place.

**Left for the user**

- Nothing to decide. Nothing is committed.
