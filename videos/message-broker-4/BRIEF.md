---
workflow: general-video
flow: automation
storyboard: no
message: "Part 4: webhook subscribers (POST with Brokerku- headers, 5 s timeout, no redirects), which failures are retried (timeouts, broken connections, 5xx, 408, 429) and which are skipped, waits from 1 s doubling to at most 5 min, skipping after 10 failures, retry state in the subscriber slot, the same limit for pull streams, the try-out, one slow webhook holding up the others"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian backend developers who use message brokers and want to know how one works inside"
length: 37.0s
angle: diagram beats with captions
series: "Bikin message broker sendiri, bagian 4 of the article series (YouTube version: videos/message-broker-4-youtube)"
---

## Intent

The TikTok cut of the article docs/articles/membuat-message-broker-sendiri/seri-membuat-message-broker-sendiri-bagian-4-webhook-dan-percobaan-ulang-dengan-jeda-yang-makin-panjang.md, made on 2026-11-07 like the
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
