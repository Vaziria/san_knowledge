---
format: 1080x1920
duration: 28s
message: "One command turns your markdown article into a Medium draft with images and captions — san_medium; tip on Saweria and DM to get it"
arc: PAS (chat-thread form) — pain → pain → product reply + value claim → demo → proof → benefit → CTA
audience: Indonesian writers and devs who publish on Medium
mode: autonomous
music: none
language: id
---

## Video direction

One continuous fiction across four frames: a DM thread between a fictional friend, **rina**
(left), and the viewer's host, **me** (right, no name). Frames 1 and 4 are the chat; frames 2 and
3 are what "me" shows — the real command with its real progress lines, then the draft filling
itself in. Series twin of `san-vpn-tiktok` (same chat UI, same palette, same rhythm).

**Palette (frame.md, Broadside, dark register only).** Ground `ink-black`. Panels, friend bubbles,
terminal windows `ink-black-alt` with a 1px `border-dark` hairline. Text `cream`; secondary text
`cream-muted`; faint `cream-hint`. `fire-orange` is reserved for **"me" and the tool**: my bubbles
are fire-orange fields with `ink-black` text (ink-on-fire, never cream on orange); the terminal
prompt glyph `>`, the draft URL, status chips and ✓ marks are fire-orange. No second accent, no
gradient, no shadow, **0 radius everywhere** (the only round things are the three typing dots and
the online dot). **One sanctioned exception, frame 3 only:** the editor mock is a `cream` page
(a prop showing Medium's white editor) with `ink-black` text, sitting inside the dark frame.

**Type.** Barlow for every chat line (lowercase casual Indonesian as written; acronyms and names keep
their case: API, DM, Medium, Saweria). Chat body ≈ 4.2cqw, weight 600 (friend) / 700 (me),
line-height ~1.3. IBM Plex Mono for the header chrome (uppercase, tracked) **and, as the one
sanctioned exception, for terminal/command content, which keeps its exact case** (commands are
case-sensitive; this is content, not chrome). Terminal text ≥ 3.3cqw. Nothing load-bearing under
2.6cqw. The editor mock uses Barlow too (no serif anywhere).

**Chat UI (frames 1 and 4 must match exactly).**
- Header band at the top (starts ~8% down, below TikTok's top tabs): square avatar (0 radius,
  `ink-black-alt`, 1px `border-dark`) with a fire-orange Barlow 800 "r"; name "rina" in Barlow 700
  cream (~4.6cqw); under it a mono "ONLINE" label in `cream-muted` with a small fire-orange dot.
  1px `border-dark` rule under the header. No app logos, no WhatsApp/Telegram look.
- Thread column: left edge ~6% of width, right edge ~84% (keeps clear of TikTok's right-side
  buttons). Bubbles stack **downward** from under the header, gap ~1.6cqw; nothing ever reflows.
- Friend bubble: left-aligned, max ~72% width, `ink-black-alt` + 1px `border-dark`, cream text.
- My bubble: right-aligned (right edge at the 84% line), max ~72% width, fire-orange field, ink text.
- Typing indicator: a small my-side bubble outline (1px fire-orange) with three fire-orange dots that
  pulse phase-offset (finite, timeline-driven); it is replaced in place by the bubble it announces.
- Bubble entry: rises ~2.5cqh and scales 0.94→1 with opacity 0→1 on a smooth long-tail settle
  (power3, ~0.35s). No overshoot.

**Safe zones.** All content in the top ~83% (bottom 17% is TikTok's caption band) and left of the 84%
line on rows below ~40% height (TikTok's right rail).

**Motion grammar.** power3 long-tail settles; every element enters on its own cue time (the cue
times in each frame are the script — they also drive the sound effects, so keep them exact). One
real camera move in the whole video: the payoff push-in onto the CTA bubble in frame 4. Holds are
still; subtle jitter at most.

**Rhythm.** Frame 1 builds (4 bubbles). Frame 2 is the busiest (log lines). Frame 3 is the "it
fills itself" payoff and ends on a held read. Frame 4 builds to the CTA and holds still to the end.

**Negative list.** No Medium logo or wordmark, no app logos, no WhatsApp/Telegram styling, no
rounded bubbles, no shadows or gradients, no second accent colour, no bounce/elastic/back eases, no
breathing loops, no drifting camera in the back half, no Math.random / Date.now, no CSS
transitions or keyframes, no slideshow (everything dumped at t=0 then frozen), no screensaver
(elements floating on their own), no invented numbers or output (every command and log line below
is from the san_medium README; the article is the real docs/articles Bagian 1).

## Frame 1 — Capek

- scene: rina complains that posting to Medium is tedious and there's no API; I answer with san_medium
- voiceover: ""
- duration: 8.5s
- transition_in: cut
- status: animated
- src: compositions/frames/01-capek.html
- type: hook
- persuasion: Pain validation
- beat: frustration → relief
- blueprint: agent-progress-theater (Adapt)
- focal: the chat thread
- roles: none (no assets)
- asset_candidates:
- sfx: host — ting @0.2, ting @1.9, pop @4.9, pop @6.2

narrativeRole: open on the viewer's own chore in the first second, and land the value claim (one
command, markdown → Medium draft) by the second beat.
keyMessage: san_medium turns a markdown file into a Medium draft with one command.

Adapt: keep sub-shape (B) message-thread building bubble by bubble; the "machine" is me answering.
The payoff push-in is saved for frame 4.
Scene 1 (0.0–0.2s): dark ground; chat header in place (avatar "r", "rina", "ONLINE"); thread empty.
Scene 2 (0.2s): friend bubble pops in — "posting ke Medium capek bgt 😩"
Scene 3 (1.9s): friend bubble pops in below — "copas teks, upload gambar satu-satu, isi caption…
  API token-nya juga udah gak dikasih"
Scene 4 (4.1–4.9s): my typing indicator appears right-aligned below; dots pulse.
Scene 5 (4.9s): the indicator is replaced in place by my bubble — "aku tinggal 1 perintah 😎"
Scene 6 (6.2s): my second bubble pops in — "san_medium: file markdown → draft Medium, lengkap sama
  gambar & caption" ("san_medium" in IBM Plex Mono, same ink colour).
Scene 7 (6.2–8.5s): hold the thread still.

## Frame 2 — Satu perintah

- scene: a full-screen terminal runs san_medium draft and its real progress lines stream in
- voiceover: ""
- duration: 6.5s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/02-satu-perintah.html
- type: feature_showcase
- persuasion: Show-don't-tell proof
- beat: curiosity → clarity
- blueprint: prompt-type-submit-generate (Adapt)
- focal: the terminal window
- roles: none (no assets)
- asset_candidates:
- sfx: host — keys @0.3, done @4.9

narrativeRole: show the one command and the tool doing the work by itself.
keyMessage: one command; the tool writes the draft and hands back its URL.

Adapt: keep the full loop — the command types, then the machine answers with an action log that
streams line by line, ending on the result (the draft URL).
Layout: one terminal window fills the frame inside the safe zone (≈ 6% side margins, top ~9% to
~80% height), `ink-black-alt`, 1px `border-dark`; a mono header bar inside it reading "TERMINAL"
(left, cream-muted). Lines left-aligned, generous spacing; command in IBM Plex Mono cream with a
fire-orange "> " prompt; log lines in IBM Plex Mono cream-muted.
Scene 1 (0.0–0.3s): empty window, caret blinking at the prompt (finite, timeline-driven).
Scene 2 (0.3–1.2s): types `san_medium draft artikel.md`.
Scene 3 (1.5s): `== opening a new story`
Scene 4 (2.0s): `== typing the title`
Scene 5 (2.5s): `== pasting the text`
Scene 6 (3.0s): `== image 1 of 3`; the counter swaps in place to `2 of 3` at 3.4s and `3 of 3` at
  3.8s (discrete text swap, no slide).
Scene 7 (4.3s): `== reloading the draft to check it`
Scene 8 (4.9s): the result line `https://medium.com/p/…/edit` in fire-orange, with a fire-orange ✓.
Scene 9 (4.9–6.5s): hold.

## Frame 3 — Ngisi sendiri

- scene: the Medium draft fills itself in — title, text, the image, its caption — then "Saved"
- voiceover: ""
- duration: 6.5s
- transition_in: crossfade
- status: animated
- src: compositions/frames/03-ngisi-sendiri.html
- type: benefit_highlight
- persuasion: Show-don't-tell proof
- beat: delight
- blueprint: compose
- focal: assets/networks-without-and-with-vpn.png
- roles: networks-without-and-with-vpn = supporting (the image that drops into the draft, shown whole, never cropped)
- asset_candidates: assets/networks-without-and-with-vpn.png — the real first image of the article (diagram: two panels "Tanpa VPN" / "Dengan VPN", warm-grey boxes on near-white, ~1.6:1 landscape)
- sfx: host — keys @0.3, pop @3.7, done @5.0

narrativeRole: the "nobody is typing" moment — the viewer sees the result they'd normally build by
hand appear by itself, image and caption included.
keyMessage: title, text, image and caption land in the draft without you touching it.

Compose: an editor page that fills top to bottom, each part on its own cue (no front-loading).
Layout: a `cream` page fills the frame inside the safe zone (≈ 6% side margins, ~9% to ~80%
height), 0 radius, flat. A thin mono status row at the page top: "DRAFT" left, a status chip right
that reads "SAVING…" (cream-hint on cream → ink at 55%) and later "SAVED" (fire-orange). Content
left-aligned with comfortable page margins. Title in Barlow 800 ink (~5cqw, wraps to 2–3 lines);
subtitle in Barlow 400 at 55% ink; body in Barlow 400 ink (~3.2cqw); caption in Barlow 400 at 55%
ink, centred under the image.
Scene 1 (0.0–0.3s): the empty page with its status row; a caret blinks in the title line.
Scene 2 (0.3–1.6s): the title types in one fast burst — "[Seri Membuat VPN Sendiri Bagian 1] Apa
  itu VPN (Virtual Private Network) ?"
Scene 3 (1.9s): the subtitle appears — "Sebelum membuatnya, kita pahami dulu apa yang sebenarnya
  dilakukan VPN."
Scene 4 (2.5s): the body paragraph appears at once as a pasted block (quick fade) — "Dalam seri ini,
  kita membangun VPN dari nol. VPN ini adalah alat kecil yang menggabungkan komputer di rumah, di
  kantor, dan di sebuah VPS ke dalam satu jaringan privat."
Scene 5 (3.3s): a small "+" circle button (1px ink outline) appears at the left of the empty line
  below; 3.7s the image drops in at full column width (fades + settles up), the "+" disappears.
Scene 6 (4.5s): the caption appears under the image — "Tanpa VPN, koneksi masuk berhenti di jalan.
  Dengan VPN, kedua PC punya alamat yang bisa saling dijangkau."
Scene 7 (5.0s): the status chip swaps "SAVING…" → "SAVED" with a fire-orange ✓.
Scene 8 (5.0–6.5s): hold still.

## Frame 4 — Mau dong

- scene: back in the chat — scheduling as a bonus, rina wants it, the Saweria/DM ask lands
- voiceover: ""
- duration: 7s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/04-mau-dong.html
- type: cta
- persuasion: Value stacking → direct ask
- beat: aspiration → urgency-to-act
- blueprint: agent-progress-theater (Adapt)
- focal: the CTA bubble
- roles: none (no assets)
- asset_candidates:
- sfx: host — pop @0.3, pop @1.1, ting @2.4, pop @4.1

narrativeRole: add one more reason (it schedules too), then turn the demo into an action inside the
conversation the viewer is watching.
keyMessage: tip on Saweria (link in bio) and DM to get san_medium.

Adapt: keep the payoff push-in signature — static through the build, then ONE push-in tight on the
confirmation (here: the CTA bubble), easing to a still hold. No end card.
Scene 1 (0.0–0.3s): the same chat UI as frame 1 (header identical). At the top of the thread, my
  earlier bubble "aku tinggal 1 perintah 😎" sits dimmed to ~40% opacity (the thread scrolled on).
Scene 2 (0.3s): my bubble pops in — "bisa dijadwalin juga, nanti Medium yang publish sendiri 📅"
Scene 3 (1.1s): my code bubble pops in right under it — an `ink-black-alt` card with a 1px
  fire-orange border, IBM Plex Mono cream, two lines: `san_medium schedule` /
  `-at "2026-11-10 09:00" <id>` (right edge on the 84% line).
Scene 4 (2.4s): friend bubble pops in — "mau dong 🥺 gimana dapetnya?"
Scene 5 (3.3–4.1s): my typing indicator; dots pulse.
Scene 6 (4.1s): replaced in place by the CTA bubble — "sawer di Saweria (link di bio), terus DM
  aku 🙏" ("Saweria" and "DM aku" in Barlow 900).
Scene 7 (4.6–5.6s): payoff push-in: camera scales ~1.3× and pans to centre the CTA bubble (power3);
  it must stay inside the safe zones after the push.
Scene 8 (5.6–7.0s): hold still to the end (final frame).
