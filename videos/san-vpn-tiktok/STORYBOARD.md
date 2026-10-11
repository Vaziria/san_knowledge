---
format: 1080x1920
duration: 28s
message: "Remote your office PC from home with no public IP — san_vpn; tip on Saweria and DM for setup help"
arc: PAS (chat-thread form) — hook question → pain → product reply + value claim → demo → proof → benefit → CTA
audience: Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls
mode: autonomous
music: none
language: id
---

## Video direction

One continuous fiction across four frames: a DM thread between a fictional friend, **dimas**
(left), and the viewer's host, **me** (right, no name). Frames 1 and 4 are the chat; frames 2 and
3 are what "me" shows — the real commands and the real output. Series twin of
`san-medium-tiktok` (same chat UI, same palette, same rhythm).

**Palette (frame.md, Broadside, dark register only).** Ground `ink-black`. Panels, friend bubbles,
terminal windows `ink-black-alt` with a 1px `border-dark` hairline. Text `cream`; secondary text
`cream-muted`; faint `cream-hint`. `fire-orange` is reserved for **"me" and the tool**: my bubbles
are fire-orange fields with `ink-black` text (ink-on-fire, never cream on orange); the terminal
prompt glyph `>`, step labels, "online" and ✓ marks are fire-orange. No second accent, no gradient,
no shadow, **0 radius everywhere** (the only round things are the three typing dots and the online
dot).

**Type.** Barlow for every chat line (lowercase casual Indonesian as written; acronyms and names keep
their case: PC, IP, ISP, CGNAT, DM, Saweria, WireGuard). Chat body ≈ 4.2cqw, weight 600 (friend) /
700 (me), line-height ~1.3. IBM Plex Mono for the header chrome (uppercase, tracked) **and, as the
one sanctioned exception, for terminal/command content, which keeps its exact case** (commands are
case-sensitive; this is content, not chrome). Terminal text ≥ 3.3cqw. Nothing load-bearing under
2.6cqw.

**Chat UI (frames 1 and 4 must match exactly).**
- Header band at the top (starts ~8% down, below TikTok's top tabs): square avatar (0 radius,
  `ink-black-alt`, 1px `border-dark`) with a fire-orange Barlow 800 "d"; name "dimas" in Barlow 700
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
real camera move in the whole video: the payoff push-in onto the CTA bubble in frame 4 (plus one
small punch-in in frame 3). Holds are still; subtle jitter at most.

**Rhythm.** Frame 1 builds (4 bubbles). Frame 2 is the busiest (typing). Frame 3 ends on a held
read of the proof. Frame 4 builds to the CTA and holds still to the end.

**Negative list.** No app or brand logos, no WhatsApp/Telegram styling, no rounded bubbles, no
shadows or gradients, no second accent colour, no bounce/elastic/back eases, no breathing loops, no
drifting camera in the back half, no Math.random / Date.now, no CSS transitions or keyframes, no
slideshow (everything dumped at t=0 then frozen), no screensaver (elements floating on their own),
no invented numbers (every IP, name and command below is from the san_vpn README).

## Frame 1 — The ask

- scene: dimas asks how to reach the office PC from home with no public IP; I answer with san_vpn
- voiceover: ""
- duration: 8s
- transition_in: cut
- status: animated
- src: compositions/frames/01-the-ask.html
- type: hook
- persuasion: Pain validation (a question the viewer has asked themselves)
- beat: frustration → relief
- blueprint: agent-progress-theater (Adapt)
- focal: the chat thread
- roles: none (no assets)
- asset_candidates:
- sfx: host — ting @0.2, ting @2.2, pop @4.9, pop @6.3

narrativeRole: open on the viewer's own problem in the first second, and land the value claim (no
public IP needed) by the second beat.
keyMessage: san_vpn reaches the office PC from home without a public IP.

Adapt: keep sub-shape (B) message-thread building bubble by bubble; the "machine" is me answering.
The payoff push-in is saved for frame 4.
Scene 1 (0.0–0.2s): dark ground; chat header in place (avatar "d", "dimas", "ONLINE"); thread empty.
Scene 2 (0.2s): friend bubble pops in — "bro, PC kantor bisa diremote dari rumah gak?"
Scene 3 (2.2s): friend bubble pops in below — "kantor gak ada IP publik, ISP rumah juga CGNAT 😭"
Scene 4 (4.0–4.9s): my typing indicator appears right-aligned below; dots pulse.
Scene 5 (4.9s): the indicator is replaced in place by my bubble — "bisa. pakai san_vpn 😎"
  (the word "san_vpn" in IBM Plex Mono, same ink colour).
Scene 6 (6.3s): my second bubble pops in — "gak butuh IP publik sama sekali"
Scene 7 (6.3–8.0s): hold the thread still.

## Frame 2 — The commands

- scene: a full-screen terminal types the real setup commands, step by step
- voiceover: ""
- duration: 7s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/02-the-commands.html
- type: feature_showcase
- persuasion: Show-don't-tell proof
- beat: curiosity → clarity
- blueprint: prompt-type-submit-generate (Adapt)
- focal: the terminal window
- roles: none (no assets)
- asset_candidates:
- sfx: host — keys @0.6, keys @1.5, keys @2.6, keys @4.3, keys @5.3

narrativeRole: show how little it takes — real commands, in order, on the two machines.
keyMessage: set up the relay on the office PC, invite the home PC, join, up.

Adapt: keep the typed-command-is-the-show signature (caret types each command in a quick burst);
no machine answer here beyond the invite string — the answer is frame 3.
Layout: one terminal window fills the frame inside the safe zone (≈ 6% side margins, top ~9% to
~80% height), `ink-black-alt`, 1px `border-dark`; a mono header bar inside it reading "TERMINAL"
(left, cream-muted). Lines left-aligned, generous line spacing, step labels in Barlow 700
fire-orange (~3.6cqw), commands in IBM Plex Mono cream with a fire-orange "> " prompt.
Scene 1 (0.0–0.4s): empty window, caret blinking at the first prompt (finite, timeline-driven).
Scene 2 (0.4s): step label "1 · di PC kantor" fades up. 0.6–1.3s types `san_vpn setup init`;
  1.5–2.1s types `san_vpn relay run` on the next line.
Scene 3 (2.4s): step label "2 · undang PC rumah". 2.6–3.4s types `san_vpn relay invite home`;
  3.6s an output line appears under it in cream-muted mono: `sanvpn1_...`
Scene 4 (4.1s): step label "3 · di PC rumah". 4.3–5.1s types `san_vpn join sanvpn1_...`;
  5.3–5.8s types `san_vpn up`.
Scene 5 (5.8–7.0s): hold; caret rests after `san_vpn up`.

## Frame 3 — It works

- scene: status shows the other machines online by name; ping office.vpn answers
- voiceover: ""
- duration: 6s
- transition_in: crossfade
- status: animated
- src: compositions/frames/03-it-works.html
- type: benefit_highlight
- persuasion: Show-don't-tell proof
- beat: confidence
- blueprint: agent-progress-theater (Adapt)
- focal: the ping card
- roles: none (no assets)
- asset_candidates:
- sfx: host — keys @0.2, keys @2.4, done @3.3

narrativeRole: the proof — machines in different places on one network, reachable by name.
keyMessage: from home, office.vpn answers.

Adapt: keep the receipt-cascade signature (rows arrive one by one and flip to a done state);
two stacked cards instead of a checklist.
Layout: two stacked terminal cards (same styling as frame 2), status card in the upper half
(~12–45% height), ping card below (~50–75%). Card header bars in mono: "PC RUMAH".
Scene 1 (0.0–0.2s): both cards' frames present, empty; status card caret blinking.
Scene 2 (0.2–0.8s): types `san_vpn status` in the status card.
Scene 3 (1.0–1.8s): output rows cascade in one by one (slide-up + fade, staggered), mono:
  `home  10.77.0.2/24  connected`
  `NAME             IP         RELAY` (cream-muted)
  `office.vpn       10.77.0.1  online`
  `jakarta-vps.vpn  10.77.0.3  online`
  ("online" and "connected" in fire-orange; columns aligned).
Scene 4 (2.4–3.0s): types `ping office.vpn` in the ping card.
Scene 5 (3.3s, 3.7s, 4.1s): three lines `Reply from 10.77.0.1` arrive one by one, each with a
  fire-orange ✓ at its end.
Scene 6 (4.3–4.9s): one small punch-in (coordinate-target-zoom, ~1.12×) centring the ping card.
Scene 7 (4.9–6.0s): hold still.

## Frame 4 — Mau dong

- scene: back in the chat — the key benefit, dimas wants it, the Saweria/DM ask lands
- voiceover: ""
- duration: 7s
- transition_in: zoom-through
- status: animated
- src: compositions/frames/04-mau-dong.html
- type: cta
- persuasion: Risk reversal (setup help, not just a download)
- beat: aspiration → urgency-to-act
- blueprint: agent-progress-theater (Adapt)
- focal: the CTA bubble
- roles: none (no assets)
- asset_candidates:
- sfx: host — pop @0.3, ting @1.9, pop @3.8

narrativeRole: turn the demo into a reason to act, inside the conversation the viewer is watching.
keyMessage: tip on Saweria (link in bio), DM, and get help setting it up.

Adapt: keep the payoff push-in signature — static through the build, then ONE push-in tight on the
confirmation (here: the CTA bubble), easing to a still hold. No end card.
Scene 1 (0.0–0.3s): the same chat UI as frame 1 (header identical). At the top of the thread, my
  earlier bubble "bisa. pakai san_vpn 😎" sits dimmed to ~40% opacity (the thread scrolled on).
Scene 2 (0.3s): my bubble pops in — "tembus CGNAT & firewall kantor. WireGuard, terenkripsi ujung
  ke ujung 🔒"
Scene 3 (1.9s): friend bubble pops in — "anjir mantap 🔥 mau dong, gimana dapetnya?"
Scene 4 (3.0–3.8s): my typing indicator; dots pulse.
Scene 5 (3.8s): replaced in place by the CTA bubble — "sawer di Saweria (link di bio), terus DM
  aku. aku bantu setup sampai jalan 🙏" ("Saweria" and "DM aku" in Barlow 900).
Scene 6 (4.4–5.4s): payoff push-in: camera scales ~1.3× and pans to centre the CTA bubble (power3);
  it must stay inside the safe zones after the push.
Scene 7 (5.4–7.0s): hold still to the end (final frame).
