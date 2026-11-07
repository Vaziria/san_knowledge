---
workflow: general-video
flow: automation
storyboard: no
message: "Part 5: head-of-line blocking, a worker pool instead of one webhook sender or a goroutine per subscriber, order kept by at most one worker per subscriber, fair turns starting from the first waiting subscriber, pull streams never waiting, the 1/2/4/8-worker benchmark, the try-out"
destination: youtube
aspect: 1920x1080
language: id
audience: "Indonesian backend developers who use message brokers and want to know how one works inside"
length: 185s
angle: long-form explainer, captions only
series: "Bikin message broker sendiri, bagian 5 of the article series (TikTok cut: videos/message-broker-5)"
---

## Intent

The YouTube version of the article docs/articles/membuat-message-broker-sendiri/seri-membuat-message-broker-sendiri-bagian-5-worker-pool-agar-webhook-yang-lambat-tidak-menahan-subscriber-lain.md, made on 2026-11-07 like the
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
- Design truth: frame.md (Broadside, its primary 16:9 frame).
