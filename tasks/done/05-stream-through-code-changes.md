# The stream and the chat carry on while code changes

The user asked (2026-09-25): "add task, and do it 1, 2, and when youtube stream ke pasted, its cached", then "cached quality option too". Items 1 and 2 are the fixes proposed after measuring what a code change does to a live stream (below). Those lines are the user's; the rest of this file was written by Claude from them. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read `docs/3d_modelling/rules.md` first: Control panel rules 10 (the Stream tab) and 12 (the Audio tab), and Stories rule 12 (the chat).

## What happens now

Measured on a copy of the project, streaming the lake meeting with "stream only" to a local RTMP listener standing in for YouTube:

- **A save in `src/`** reloads the pages. The hidden browser's new recording starts ffmpeg again, so YouTube sees the stream reconnect: 2.9 s without a picture, the new connection 4.1 s after the save. With web + stream, the page's reload ends the stream (`sendBeacon` to `/stop` on `pagehide`).
- **A save with a syntax error:** the hidden page can't load, and the stream ends itself after 40 s without a picture.
- **A save of `vite.config.ts`, a `*-bridge.ts`, `meeting.ts` or `src/story/lake_meeting/events.ts`** restarts the dev server: the stream ends at once, the chat reader stops (Read chat must be pressed again), and the meeting's viewers are gone, bots in their place.
- **The stream key** is typed again after every reload, and the quality goes back to 720p.

## 1. YouTube's connection stays open, whatever the page does

- **One ffmpeg for the whole stream,** fed pictures by the bridge at a steady 30 a second, raw, in the encoder's pixels. It connects once, at the first picture, and only Stop, an error on YouTube's side or the dev server ending closes it.
- **Each recording has its own decoder** (a second ffmpeg: WebM in, scaled to the stream's size, 30 fps raw frames out). A new recording, from a reloaded page, gets a new decoder; the encoder never notices.
- **No new picture, the last one again:** while a page reloads, is broken, or its tab is hidden, the stream shows its last picture, with the sound going on. A short buffer (about 0.4 s) smooths the pieces' arrival.
- **Stream only (default):** the stream waits for the hidden browser as long as it runs, however long the page stays broken; a save that fixes it brings the pictures back. Only the hidden browser closing, or no first picture within 40 s of Go live, ends it.
- **Web + stream (default):** a reload of the page that streams carries on. The page keeps the stream's session in `sessionStorage`, which a reload keeps and another tab doesn't have, and records again as it loads. Another tab can't take the stream over. A page that doesn't come back within 60 s ends the stream, as a closed tab should.
- **The status** says when the picture is frozen, and for how long, in the Stream tab and once in the terminal.
- **The frame rate** shown is still the page's new pictures a second, now read from the decoder.

## 2. A dev server restart hands everything to the new one

- Vite 8 runs a plugin's `closeServer` hook with `reason: 'restart'`, after the new server's plugins have started. On a restart the old bridges hand over instead of stopping; on a real close they stop as now.
- **The stream (default):** a stream that runs keeps the code it started with until it ends: the new bridge passes `/__stream/*` to it. The next stream uses the new code.
- **The chat reader:** the new bridge takes over the running `san_youtube`, so no comment is missed and Read chat isn't needed again.
- **The meeting:** its state (the animals, their viewers, when each last wrote, the event numbers) goes into a new `Meeting` built from the new code, so an edit of `meeting.ts` applies at once.
- The pages still reload when the dev server comes back (Vite's client does that). With 1, the stream shows its last picture through it.

## 3. The key and the quality are remembered

- The stream key and the quality are kept in `localStorage` as they are typed or picked, like the stream URL. An emptied key field forgets the key. The key stays a password field and stays masked in the terminal.

## Checks

On a copy of the project with its own port and a local RTMP listener, as before:

- a save in `src/`: one connection throughout, and the longest gap between new pictures;
- a save with a syntax error, kept for 60 s: still live, frozen, then new pictures after the fix;
- a save of `meeting.ts`: the stream, the chat reader (`san_youtube`'s process) and the meeting's viewers are all still there;
- web + stream: the page reloaded, one connection throughout; another tab refused; the tab closed, the stream ends after 60 s;
- the key and the quality still filled in after a reload.
- `npm run typecheck`, and the meeting's tests if there are any.

Update rules.md (Control panel rule 10, Stories rule 12) and the bridges' header comments. Don't commit.

Taken: 2026-09-25 18:08

## Done

**What changed**

- `stream-bridge.ts`:
  - One encoder ffmpeg per stream takes raw pictures on stdin, at 30 a second by the bridge's own clock. It connects with the first 0.4 s of pictures and stays connected until Stop.
  - Each recording gets a decoder ffmpeg (WebM in; fitted, 30 fps raw frames out).
  - With no new picture, the last one is sent again. A picture that comes after its turn is skipped, so a hitch in the drawing shows no longer than it was.
  - The start answers `{ session }`. A new recording takes over when it comes from the hidden browser, or from a page naming the session; anything else gets 409.
  - The stream ends only when:
    - the first pictures don't come (15 s, or 40 s for the hidden browser);
    - the hidden browser closes;
    - a web + stream page sends nothing for 60 s.
  - The status has `frozen` (seconds of the same picture). The terminal is told at 5 s, and again when pictures come back.
  - The `closeServer` hook carries a running stream over a dev server restart (a `globalThis` slot). The stream keeps its code until it ends.
- `chat-bridge.ts`: on a restart the old plugin hands the new one the running `san_youtube`, with its line readers, and the meeting. The handover is versioned (`HANDOVER`); one the new code doesn't know is refused, and the reading stops.
- `meeting.ts`: `save()`, and `new Meeting(emit, now, random, saved)` to go on from a saved meeting.
- `src/stream.ts`:
  - The session is kept in `sessionStorage`, and a reloaded page records again.
  - There is no stop on `pagehide` any more.
  - `stored`/`store` take a storage and can forget a value.
- `src/panel/StreamTab.tsx`:
  - The key and the quality are kept in `localStorage`; an emptied key field forgets the key.
  - A line says how long the picture has stood still, and why.
- `docs/3d_modelling/rules.md`: Control panel rule 10 (How, stream only, An action, Status, the new Through code changes) and Stories rule 12.

**How it was checked** (on a copy of the project with its own ports, and a local RTMP listener standing in for YouTube)

- **Lake meeting, stream only, with @LofiGirl's chat being read and test viewers in the meeting.** One connection lasted from Go live to Stop through:
  - a normal save: at most 4 s of the same picture;
  - a save with a syntax error kept for 45 s: the same picture for 49 s, then new pictures after the fix;
  - a save of `meeting.ts`;
  - a save of `stream-bridge.ts`.

  The same `san_youtube` process read the chat throughout. Real viewers from that chat joined and stayed through the restarts, a test viewer joined after them, and the event numbers kept counting. Before this change, the normal save cost 2.9 s and a reconnect, the broken save ended the stream after 40 s, and a restart ended it at once.
- **After a restart, the carried stream still steers its hidden browser:** a settings change (night) reached the hidden page's URL.
- **A moving test picture streamed as a web + stream page by a script:**
  - no still stretch in steady streaming, 30 fps;
  - a 3 s reload seen as 2.2–2.6 s of one picture;
  - a page holding its pieces back 0.8 s seen as 0.2–0.3 s;
  - another page, and a wrong session, refused with 409;
  - a closed page ends the stream after 60 s.
- **The panel's own page in headless Edge (web + stream):**
  - the URL, key and 1080p typed or picked, and kept in `localStorage`;
  - live, then reloaded: the same session, new pictures within a second, one connection, and the key and 1080p still filled in;
  - a second tab doesn't take the stream;
  - Stop forgets the session;
  - no page errors.
- `npm run typecheck` is clean. There are no meeting tests to run.

**Left for the user**

- The lake scene's own hitches in the first seconds after its page loads show as short still stretches, as before.
- The first save of a bridge after this change came from the old code, which couldn't hand over. Your dev server has run the new code since then: your 18:30 stream to @GetResolved runs on it.
- Nothing is committed.
