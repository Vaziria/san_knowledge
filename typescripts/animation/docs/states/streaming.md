# Streaming state

Updated 2026-09-25 by task 17 (the first state record), from the code as it is today.

## What works

- **The stream.** The Stream tab ([StreamTab.tsx](../../src/panel/StreamTab.tsx)) streams the scene live to YouTube, or to any `rtmp://` or `rtmps://` server. The panel is not in the picture. It takes the stream URL (YouTube's is filled in), the key, and 720p (3 Mbit/s) or 1080p (6 Mbit/s) at 30 fps, with Go live and Stop. The full rule is [rules.md, Control panel](../../../../docs/3d_modelling/rules.md#control-panel) rule 10.
  - **How:** a page records the canvas with `MediaRecorder` (WebM, H.264 where the browser can, else VP8, 8 Mbit/s). It posts quarter-second pieces, in order, to `/__stream` ([stream.ts](../../src/stream.ts)). [stream-bridge.ts](../../stream-bridge.ts) gives each recording its own decoder ffmpeg. One encoder ffmpeg per stream gets the pictures at a steady 30 a second from a 0.4 s buffer. It makes H.264 with a keyframe every 2 s, adds AAC sound and pushes FLV. It connects with the first pictures and stays connected until Stop.
  - **Render** (kept in `localStorage`, locked while live):
    - **web + stream** (the default): the page's own drawing streams. A hidden tab freezes the stream.
    - **stream only:** the bridge starts a headless Chrome or Edge with a throwaway profile. It opens the page as `?renderer=1280x720` plus the panel's settings ([streamRenderer.ts](../../src/streamRenderer.ts)). The panel steers it through `/follow`: settings, behaviour runs, Restart demo and held walking moves. The page shows a JPEG of the stream every second in place of the scene. A page opened later takes on the stream's settings. The hidden page's errors go to the terminal.
    - **web only:** nothing streams.
  - **GPU** (on by default): ffmpeg tries each hardware encoder it has once, on a second of test picture, in this order: NVENC, Quick Sync, AMF, VideoToolbox, Media Foundation, Vulkan, VA-API. It uses the first that works, or libx264 if none does. With stream only, the hidden browser also draws on the graphics card; with GPU off it draws in SwiftShader. The tab says what draws the pictures and what encodes them.
  - **Status**, asked once a second: connecting; live with the time, new pictures a second and the bitrate; how long the picture has stood still, and why; or ffmpeg's last words when it gives up, which also stops the recording.
  - **The key:** the URL, key, quality, Render and GPU are remembered in `localStorage`, and emptying the key field forgets the key. The key is a password field and is masked (`••••`) in the terminal and in every message. It must be letters, digits, `-` and `_`.
  - **Through code changes:**
    - When no new picture comes, the last one goes again, so YouTube's connection stays open.
    - A reloaded web + stream page carries on. It names the stream's session, kept in `sessionStorage`. Other pages are refused (409).
    - A broken save with stream only freezes the picture until the fix.
    - A stream ends only on Stop, ffmpeg failing, the hidden browser closing, no first picture within 15 s (40 s for the hidden browser), or a web + stream page silent for 60 s.
    - A dev server restart carries a running stream over. It keeps the code it started with until it ends. A close of the dev server ends it.
- **Wiring.** [vite.config.ts](../../vite.config.ts) adds `oscBridge()`, `streamBridge()`, `audioBridge()` and `chatBridge()`. All four are `apply: 'serve'`, so only `npm run dev` (port 8087) has them. They are plain HTTP, so they work through a tunnel. The OSC bridge is in [pianos.md](pianos.md).
- **The sound.** The Audio tab ([AudioTab.tsx](../../src/panel/AudioTab.tsx), [audio.ts](../../src/audio.ts)) picks the source: none (the default), one file, every file in turn, or an input device. It also has Loop, Volume (0–100 %) and Play here. The full rule is [rules.md, Control panel](../../../../docs/3d_modelling/rules.md#control-panel) rule 12.
  - **Files and devices:** [audio-bridge.ts](../../audio-bridge.ts) lists and serves the files in `typescripts/animation/audio` (git-ignored) or `AUDIO_DIR`, with ranges. Add files uploads into that folder: at most 300 MB, plain names only, written to `.part` first. It lists input devices as ffmpeg opens them.
  - **In the stream,** the dev server makes the sound, so it is the same with either Render. `AudioFeed` writes 44.1 kHz stereo PCM into the encoder's pipe 3 by its own clock, with silence where the source has nothing. The sound changes while live (`POST /__stream/audio`) without the stream stopping. A source it can't play leaves silence, and the tab says why.
  - **Here:** Play here plays the file in an `<audio>` element; a device's sound stays on the dev server's machine. After a reload the page waits for a click before it plays. A page opened during a stream takes on the stream's sound.
- **The chat reader.** [chat-bridge.ts](../../chat-bridge.ts) reads a YouTube live chat for the lake meeting. The full rule is [rules.md, Stories](../../../../docs/3d_modelling/rules.md#stories) rule 12.
  - **san_youtube:** the bridge runs `san_youtube -json -new -lang id -poll 1s <channel>`, from `golang/packages/san_youtube/bin` or `SAN_YOUTUBE`. [san_youtube](../../../../golang/packages/san_youtube/README.md) is our Go tool that reads a live chat the way youtube.com's chat window does, with no API key.
  - **Messages:** text, Super Chats, Super Stickers, memberships, gift purchases and deletions go to `meeting.ts`. What the meeting does with them is in [lake_meeting.md](lake_meeting.md).
  - **Retries:** 30 s after "not live" or a network error. 2 minutes after a rate limit, chat off, members only or video not found. It gives up only when the channel or video doesn't exist.
  - **The Story tab's** [ChatControls.tsx](../../src/panel/ChatControls.tsx), shown while the lake meeting plays:
    - the channel, kept in `localStorage`;
    - Read chat / Stop reading, with its status;
    - made-up messages through `POST /__chat/say`: a viewer, the owner, a Super Chat of Rp 200.000 or a new member.
  - **To every page:** the status goes out as the Vite event `chat:status`, and the meeting's events as `meeting:event` ([chat.ts](../../src/chat.ts)).
  - **Restarts:** a dev server restart hands the running `san_youtube` and the saved meeting to the new plugin (`HANDOVER` 1). A shape the new code doesn't know is refused, and reading stops.

## How it was last checked

- 2026-09-25, task 17 (this record): `npm run typecheck` is clean at 23:10, with other sessions' unfinished work in the tree (earlier that evening it failed for a few minutes on the dock's half-built lines, task 09). No stream was run for this record.
- 2026-09-25, [task 05](../../../../tasks/done/05-stream-through-code-changes.md), on a copy of the project with a local RTMP listener standing in for YouTube:
  - The lake meeting with stream only, with a real chat read: one connection lasted through four saves.
    - A normal save: at most 4 s of the same picture.
    - A syntax error kept for 45 s: 49 s of the same picture, then new pictures after the fix.
    - A save of `meeting.ts`, then one of `stream-bridge.ts`.
    - The same `san_youtube` process read the chat throughout.
  - A web + stream reload kept the session. A second tab was refused. A closed page ended the stream after 60 s.
  - The key and 1080p were still filled in after a reload.
  - `npm run typecheck` was clean.
- 2026-09-24/25, before the task queue (rules.md Control panel rule 12, "Checked"): the Audio tab end to end, measured with ffmpeg's `volumedetect`: a file uploaded and played here; looping at 50 % with no dip across the loop points, then silence picked live; every file with no loop, then the WO Mic device picked live; stream only with silence live in 3 s at 30 fps.
- 2026-09-25, before the task queue (rules.md Stories rule 12): with `-poll 1s`, a message took 1.3–2.4 s from being sent to reaching the bridge. At YouTube's own pace it took 1.6–10.3 s.
- 2026-09-24, commit 278ad67 (rules.md Control panel rule 10, "Checked without YouTube"), in headless Edge:
  - each Render mode, and stream only with the GPU on and off;
  - the hidden page at 1280×720, following a theme and a behaviour, surviving a reload and the panel's page closing, and closing at Stop;
  - the listener received H.264 1280×720 at 30 fps, with AAC.

## Known issues and left for the user

- The lake scene's own hitches in the first seconds after a page loads show as short still stretches (task 05). A hidden web + stream tab freezes the stream, by design.
- The Stream tab's note for web + stream still says "with silent sound" ([StreamTab.tsx](../../src/panel/StreamTab.tsx), `NOTES.both`). That dates from before the Audio tab: the stream plays the Audio tab's sound with either Render.
- With stream only, the bridge lets any new `/__stream/recording` take the stream over, with no session check; only web + stream checks one. Today only the hidden browser posts one.
- Only Windows closes the hidden browser when the dev server's process is killed. Elsewhere a hard kill can leave it running.
- Nothing since commit 278ad67 is committed: `audio-bridge.ts`, `chat-bridge.ts`, `meeting.ts`, `src/audio.ts`, `src/chat.ts`, `AudioTab.tsx` and `ChatControls.tsx` are untracked, and task 05's changes and san_youtube's `-poll` are uncommitted.
- Task 05 noted that the first bridge save couldn't hand over. That was a one-off: the dev server has run the new code since.

## Open questions

- **On this machine** (RTX 3050, driver 572.83), ffmpeg 9's NVENC needs driver 610 or newer, so Media Foundation's NVIDIA encoder is used. A newer driver would bring NVENC. Without the graphics card, the fox scene draws at about 12 pictures a second instead of 30.
- **Sonic Pi into the stream** needs a device that records the machine's output (Stereo Mix, or VB-CABLE). The only input here today is "Microphone (WO Mic Device)".
- **The numbers are Claude's picks** and open to change: the bitrates (3 and 6 Mbit/s out, 8 Mbit/s for the recording), the 0.4 s buffer, the 15 s, 40 s and 60 s limits, and the chat's 30 s and 2 min retries.
- **The chat stays on polling** every second, as the user chose. The official push (gRPC) needs a Google Cloud API key and quota. This machine gets 429 from YouTube after a few quick page loads, so keep live tests short.
- **No queued, in-progress or blocked task changes this area.** Tasks 09, 11, 14 and 16 change the meeting, the lake and the panel, and the stream picks those up through the settings.

## History

- 2026-09-25 [task 05, stream through code changes](../../../../tasks/done/05-stream-through-code-changes.md): one encoder per stream, a decoder per recording, the last picture repeated; the session in `sessionStorage`, and an end after 60 s without the page; the stream, the chat reader and the meeting handed over a dev server restart; the key and the quality remembered.
- 2026-09-25, before the task queue (not committed): the chat reader for the lake meeting (`chat-bridge.ts`, `chat.ts`, `ChatControls.tsx`), polling every second (san_youtube's `-poll`).
- 2026-09-24/25, before the task queue (not committed):
  - the Audio tab and `audio-bridge.ts`: the stream's sound, which had been silence;
  - held walking moves passed to the hidden browser.
- 2026-09-24 commit 278ad67: the Stream tab.
  - RTMP through the dev server's ffmpeg, with Render (web + stream, stream only, web only) and GPU.
  - The key was not stored then.
- 2026-09-24 commit 47c6752: san_youtube, the Go chat reader.
