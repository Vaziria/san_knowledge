---
workflow: general-video
flow: automation
storyboard: no
message: "Part 5: head-of-line blocking, a worker pool instead of one webhook sender or a goroutine per subscriber, order kept by at most one worker per subscriber, fair turns starting from the first waiting subscriber, pull streams never waiting, the 1/2/4/8-worker benchmark, the try-out"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian backend developers who use message brokers and want to know how one works inside"
length: 37.0s
angle: diagram beats with captions
series: "Bikin message broker sendiri, bagian 5 of the article series (YouTube version: videos/message-broker-5-youtube)"
---

## Intent

The TikTok cut of the article docs/articles/membuat-message-broker-sendiri/seri-membuat-message-broker-sendiri-bagian-5-worker-pool-agar-webhook-yang-lambat-tidak-menahan-subscriber-lain.md, made on 2026-11-07 like the
Bagian 1 to 3 cuts (videos/message-broker-1 ... -3: about 37 s, captions only). The user chose "6 YouTube + 6 TikTok,
one per part". No voice, readable muted on a phone; the user may add a trending sound in the app.

## Notes

- Built by make.mjs from a spec (scene kit: videos/message-broker-shared/tall.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML.
- Educational only, like the article: the program is `brokerku`, never the user's own broker or tools.
- The hook is complete at 0 s (no entrance), so the first frame works as the cover.
- TikTok safe zones: content below the top ~8%, above the bottom 17%, and left of x 901 below 40% (tall.mjs keeps the
  diagrams in x 59..901 and refuses captions that would start below y 1420). Diagram labels are 30 px or larger.
- Sound: the VPN cuts' SFX only (pops on cuts, keys/ting), no music.
- Post: tiktok.md (hashtags first, Indonesian, no emoji).
- Design truth: frame.md (Broadside).
