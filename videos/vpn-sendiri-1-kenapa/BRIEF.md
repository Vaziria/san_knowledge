---
workflow: general-video
flow: automation
storyboard: no
message: "Your office PC can't be reached because neither side has a public IP; give both a meeting point (a small VPS) and lock it with WireGuard: your own VPN"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls"
length: 26s
angle: kinetic-type
series: "Bikin VPN sendiri, part 1 of 3 (part 2: videos/vpn-sendiri-2-server, part 3: videos/vpn-sendiri-3-sambung)"
---

## Intent

The user, 2026-10-21: "lets create youtube video and tiktok, im runout scheduled video", then
"dont expose my san_vpn, better we create video to lead the viewer to create their own vpn". Picked
WireGuard + a cheap VPS, three videos, a mixed style ("yes mixed"): this part is kinetic text with
fast cuts, parts 2 and 3 are big code cards. No voice (captions only), readable muted; the user may
add a trending sound in the app. The VPN topic follows the 43K-view office-PC TikTok (14 Oct).

## Notes

- Must not mention or show san_vpn (the user's own tool).
- Claims stay true for a hub-and-spoke WireGuard setup: traffic is encrypted on each hop, but the VPS
  forwards it, so no "end to end" claim. No prices for the VPS.
- Design truth: frame.md (Broadside: ink-black / fire-orange, lowercase Barlow 900 display, IBM Plex
  Mono chrome), copied from videos/san-vpn-tiktok.
- TikTok safe zones: content below the top ~8%, above the bottom 17%, and left of x 907 below 40%.
