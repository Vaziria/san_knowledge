---
workflow: product-launch-video
flow: automation
storyboard: no
message: "Remote your office PC from home with no public IP — san_vpn; tip on Saweria and DM for setup help"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls"
length: 20-30s
angle: dm-thread
---

## Intent

One of a pair of TikToks (with san-medium-tiktok) in the same series style: concept 4, "the DM
thread". The whole video is a dark chat between a fictional friend ("Dimas") and the user (right
side, no name). Dimas asks how to reach the office PC from home when the office has no public IP
and the home ISP uses CGNAT; the user answers with san_vpn — a command card, a status/ping card,
the key benefit — and the thread ends on the CTA. Casual Indonesian, short messages, readable
with the sound off.

User's ask, verbatim: "create 2 video tiktok that showcase san_vpn, and san_medium. and if viewer
like to have, sawer it saweria in bio and dm me".

## Customizations

- CTA (san_vpn repo is public on GitHub, so the offer is setup help): "sawer di Saweria (link di
  bio), DM aku, aku bantu setup sampai jalan" — wording may be tightened, meaning kept.
- Sound: chat sounds only (message "ting", soft keyboard clicks). No music bed — the user adds a
  trending TikTok sound in the app.
- Look: dark chat, one accent colour for the tool, large monospace for command cards. Not
  WhatsApp's look or logo; generic chat UI.

## Notes

- Claims only what golang/packages/san_vpn/README.md backs: no public IP needed (dev tunnel
  relay), works behind CGNAT and office firewalls (anywhere outbound HTTPS works), WireGuard end
  to end (relay can't read it), names like office.vpn, Windows / Linux / Raspberry Pi.
- Command output is taken from the README examples, trimmed to fit a phone.
- 8 bubbles in ~25 s: max 4 lines per card, large type, chat clear of TikTok's right rail and
  bottom caption band.
- Do not claim Cloud Run or Windows names are proven (README marks them untested).
