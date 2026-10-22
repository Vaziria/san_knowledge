---
workflow: general-video
flow: automation
storyboard: no
message: "Bikin VPN sendiri, YouTube long-form: both PCs on the VPN: the WireGuard app, office and home configs, a [Peer] per PC on the server, Activate and the handshake, Remote Desktop, ping and mstsc; what to check if it doesn't work"
destination: youtube
aspect: 1920x1080
language: id
audience: "Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls"
length: 166s
angle: long-form tutorial, captions only
series: "Bikin VPN sendiri (TikTok cut: videos/vpn-sendiri-3-sambung)"
---

## Intent

The YouTube version of the TikTok cut in videos/vpn-sendiri-3-sambung. The user, 2026-10-21: "for youtube, we dont use short,
because its bad for our organic & continuity traffic. for youtube, we use long about 3 minute"; picked three ~3 min
videos (one per part) and captions only (no voice). 16:9, because YouTube counts vertical videos up to 3 minutes as Shorts.

## Notes

- Built by make.mjs from a spec (scene kit: videos/vpn-sendiri-shared/wide.mjs, diagrams.mjs); edit the spec and rerun
  `node make.mjs`, never the generated HTML.
- Must not mention san_vpn. Design truth: frame.md (Broadside, its primary 16:9 frame).
- Post: youtube.md (chapters match the scene start times; YouTube refuses < and > in descriptions).
