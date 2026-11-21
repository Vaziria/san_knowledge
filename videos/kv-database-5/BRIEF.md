---
workflow: general-video
flow: automation
storyboard: no
message: "Build your own key-value database in Go, part 5: the free list with a one-commit wait, the 75% compaction threshold, compaction that does not stop readers, an LRU cache, 9.3 GB of data on a ~7.4 GB RAM laptop, and mmap weighed again with numbers"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian backend developers who use databases and want to know how one works inside"
length: 38s
angle: diagram beats with captions
series: "Bikin database key-value sendiri, bagian 5 of the article series (YouTube version: videos/kv-database-5-youtube)"
---

## Intent

The TikTok cut of the article docs/articles/membuat-database-key-value-sendiri/seri-membuat-database-key-value-sendiri-bagian-5-compaction-dan-data-yang-lebih-besar-dari-ram.md, made like the message broker
series cuts (videos/message-broker-*: 36 to 38 s, captions only). No voice, readable muted on a phone; the user may add a
trending sound in the app.

## Notes

- Built by make.mjs from a spec (scene kit: videos/kv-database-shared/tall.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML.
- Educational only, like the article: the program is `kvku`, never the user's own database or tools.
- The hook is complete at 0 s (no entrance), so the first frame works as the cover.
- TikTok safe zones: content below the top ~8%, above the bottom 17%, and left of x 901 below 40% (tall.mjs keeps the
  diagrams in x 59..901 and refuses captions that would start below y 1420). Diagram labels are at least 30 px.
- Sound: the VPN cuts' SFX only (pops on cuts, keys/ting), no music.
- Post: tiktok.md (hashtags first, Indonesian, no emoji).
- Design truth: frame.md (Broadside).
