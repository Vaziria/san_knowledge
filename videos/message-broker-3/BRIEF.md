---
workflow: general-video
flow: automation
storyboard: no
message: "Part 3: a subscriber that pulls its own messages over one HTTP/2 request (hello, messages in the response body, acks in the request body), one message in flight, the ack timeout, one stream per subscriber with HTTP/2 pings, the delivery loop, the try-out"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian backend developers who use message brokers and want to know how one works inside"
length: 36.4s
angle: diagram beats with captions
series: "Bikin message broker sendiri, bagian 3 of the article series (YouTube version: videos/message-broker-3-youtube)"
---

## Intent

The TikTok cut of the article docs/articles/membuat-message-broker-sendiri/seri-membuat-message-broker-sendiri-bagian-3-subscriber-yang-menarik-pesannya-sendiri.md, made like the VPN series cuts (videos/vpn-sendiri-1-kenapa ...
-3-sambung: 26 to 39 s, captions only). The user, 2026-11-07, chose "6 YouTube + 6 TikTok, one per part". No voice,
readable muted on a phone; the user may add a trending sound in the app.

## Notes

- Built by make.mjs from a spec (scene kit: videos/message-broker-shared/tall.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML.
- Educational only, like the article: the program is `brokerku`, never the user's own broker or tools.
- The hook is complete at 0 s (no entrance), so the first frame works as the cover.
- TikTok safe zones: content below the top ~8%, above the bottom 17%, and left of x 901 below 40% (tall.mjs keeps the
  diagrams in x 59..901 and refuses captions that would start below y 1420).
- Sound: the VPN cuts' SFX only (pops on cuts, keys/ting), no music.
- Post: tiktok.md (hashtags first, Indonesian, no emoji).
- Design truth: frame.md (Broadside).
