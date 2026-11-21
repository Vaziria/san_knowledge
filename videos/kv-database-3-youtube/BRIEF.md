---
workflow: general-video
flow: automation
storyboard: no
message: "Build your own key-value database in Go, part 3: the B+tree in a file of fixed 16 KB pages, the meta page and node page layout, ReadAt vs mmap and why kvku does not map yet, the Sync order and the risk of overwriting pages in place, a Get that reads three pages of a million-key file"
destination: youtube
aspect: 1920x1080
language: id
audience: "Indonesian backend developers who use databases and want to know how one works inside"
length: 198s
angle: long-form explainer, captions only
series: "Bikin database key-value sendiri, bagian 3 of the article series (TikTok cut: videos/kv-database-3)"
---

## Intent

The YouTube version of the article docs/articles/membuat-database-key-value-sendiri/seri-membuat-database-key-value-sendiri-bagian-3-menyimpan-btree-di-halaman-16-kb-lalu-menimbang-mmap.md, made like the message broker
series videos (videos/message-broker-*-youtube). Captions only (no voice), about 3 minutes and over 3:00, 16:9, because
YouTube counts vertical videos up to 3 minutes as Shorts (the user, 2026-10-21: "for youtube, we dont use short ... we
use long about 3 minute").

## Notes

- Built by make.mjs from a spec (scene kit: videos/kv-database-shared/wide.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML. The diagrams redraw the article's own (assets/*.svg) as animated scenes.
- Educational only, like the article: the program is `kvku`, never the user's own database or tools. Numbers, logs and
  test names come from the article.
- The first frame is complete at 0 s (no entrance), so it works as a thumbnail.
- Sound: the VPN videos' SFX only (pops on cuts, keys/ting), no music.
- Post: youtube.md (chapters match the scene start times that make.mjs prints; YouTube refuses < and > in descriptions;
  `{{MEDIUM_URL}}` is replaced with the article's Medium link once its draft exists).
- Design truth: frame.md (Broadside, its primary 16:9 frame).
