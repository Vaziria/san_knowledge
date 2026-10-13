---
workflow: product-launch-video
flow: automation
storyboard: no
message: "Radio musik santai 24 jam — tulis komentar, namamu diketik di layar"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian TikTok viewers who play calm music while they work, study or relax at night"
length: 20-30s
angle: terminal-is-the-video
---

## Intent

A TikTok that brings people to the user's 24/7 music live stream on YouTube (@GetResolved),
which runs on a Raspberry Pi 3B+. The goal is new viewers and subscribers, not revenue.

The whole video is the radio's own terminal screen, re-laid vertically for a phone: neofetch
(Raspberry Pi 3B+, the track count), `eza --tree` of the real song files, then the chat pane
typing `play auntum.wav`, then `greet @kamu "…"` answered by `Hello, @kamu!` (the hook: comment
and your name is typed on screen), and the CTA typed as a command. It shows exactly what a
viewer gets when they open the stream.

User's choices, verbatim from the intent questions: goal "Visit the live radio", format "The
radio's own terminal", storyboard "No, one finished video", flow "Automation", sound "Keyboard
clicks only", brief "Yes, build it".

## Assets

- golang/packages/san_radio/background/terminal.html — the stream's real background (night
  scene, sakura, torii, translucent purple-bordered terminal, pixel-art card); the video's look.
- golang/packages/san_radio/fonts/DejaVuSansMono.ttf — the stream's font (free licence).
- golang/packages/san_radio/assets/*.wav — the real track names for the tree (the audio itself
  is not used).

## Customizations

- Sound: keyboard clicks while the terminal types; no music bed (the user adds a trending sound
  in TikTok).
- CTA: "cari @GetResolved di YouTube" (not "link di bio" — unknown whether the bio links it).

## Notes

- Claims only what the stream does: 24/7 music, Raspberry Pi 3B+, the user's own tracks, live
  chat comments greeted on screen as `greet @name "comment"` → `Hello, @name!`.
- Colours from terminal.html: pink #ff7ac6, blue #5ccfe6, green #9ece6a, yellow #e0af68,
  white #e6e6f0, muted #8b8fb5, ground #0d0b1e; font DejaVu Sans Mono.
- Sibling TikToks: videos/san-vpn-tiktok, videos/san-medium-tiktok (DM-thread style; this one
  deliberately uses the stream's terminal look instead).
- Never show the stream key or any real viewer's name; `@kamu` stands for the viewer.
