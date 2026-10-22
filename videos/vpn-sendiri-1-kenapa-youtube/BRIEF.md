---
workflow: general-video
flow: automation
storyboard: no
message: "Bikin VPN sendiri, YouTube long-form: why the office PC can't be reached (NAT, CGNAT), three ways to remote it, the VPS meeting point, WireGuard, what you need, the series map"
destination: youtube
aspect: 1920x1080
language: id
audience: "Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls"
length: 176s
angle: long-form tutorial, captions only
series: "Bikin VPN sendiri (TikTok cut: videos/vpn-sendiri-1-kenapa)"
---

## Intent

The YouTube version of the TikTok cut in videos/vpn-sendiri-1-kenapa. The user, 2026-10-21: "for youtube, we dont use short,
because its bad for our organic & continuity traffic. for youtube, we use long about 3 minute"; picked three ~3 min
videos (one per part) and captions only (no voice). 16:9, because YouTube counts vertical videos up to 3 minutes as Shorts.

## Notes

- Built by make.mjs from a spec (scene kit: videos/vpn-sendiri-shared/wide.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML.
- Must not mention san_vpn. Design truth: frame.md (Broadside, its primary 16:9 frame).
- Post: youtube.md (chapters match the scene start times; YouTube refuses < and > in descriptions).
