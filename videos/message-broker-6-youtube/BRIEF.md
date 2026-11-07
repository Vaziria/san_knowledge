---
workflow: general-video
flow: automation
storyboard: no
message: "Part 6, the series finale: one broker package run three ways (embedded, mounted under /broker/ in an application's HTTP server, standalone with a CLI), the series recap, the weaknesses of the whole design, when RabbitMQ, NATS or Kafka is better; the end card closes the series"
destination: youtube
aspect: 1920x1080
language: id
audience: "Indonesian backend developers who use message brokers and want to know how one works inside"
length: 186s
angle: long-form explainer, captions only
series: "Bikin message broker sendiri, bagian 6 of the article series (TikTok cut: videos/message-broker-6)"
---

## Intent

The YouTube version of the article docs/articles/membuat-message-broker-sendiri/seri-membuat-message-broker-sendiri-bagian-6-menjalankan-broker-dan-menutup-seri.md, made on 2026-11-07 like the
Bagian 1 to 3 videos (videos/message-broker-1-youtube ... -3-youtube). The user chose "6 YouTube + 6 TikTok, one per
part" for the series; captions only (no voice), about 3 minutes, 16:9, because YouTube counts vertical videos up to 3
minutes as Shorts (the user, 2026-10-21: "for youtube, we dont use short ... we use long about 3 minute").

## Notes

- Built by make.mjs from a spec (scene kit: videos/message-broker-shared/wide.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML. The diagrams redraw the article's own (assets/*.svg) as animated scenes.
- Educational only, like the article: the program is `brokerku` (with `pembaca` and `penerima`), never the user's own
  broker or tools. Numbers, logs and test names come from the article.
- The first frame is complete at 0 s (no entrance), so it works as a thumbnail.
- Sound: the VPN videos' SFX only (pops on cuts, keys/ting), no music.
- Post: youtube.md (chapters match the scene start times that make.mjs prints; YouTube refuses < and > in descriptions;
  `{{MEDIUM_URL}}` is replaced with the article's Medium link once its draft exists).
- Bagian 6 is the last part: its end card closes the series instead of teasing a next part.
- Design truth: frame.md (Broadside, its primary 16:9 frame).
