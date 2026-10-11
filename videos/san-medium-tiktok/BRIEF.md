---
workflow: product-launch-video
flow: automation
storyboard: no
message: "One command turns your markdown article into a Medium draft with images and captions — san_medium; tip on Saweria and DM to get it"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian writers and devs who publish on Medium"
length: 20-30s
angle: dm-thread
---

## Intent

One of a pair of TikToks (with san-vpn-tiktok) in the same series style: concept 4, "the DM
thread". The whole video is a dark chat between a fictional friend ("Rina") and the user (right
side, no name). Rina complains that posting to Medium means copy-pasting text and uploading
images one by one, and Medium no longer gives API tokens; the user answers with san_medium — a
command card with its progress lines, a mock editor filling itself in, a schedule card — and the
thread ends on the CTA. Casual Indonesian, short messages, readable with the sound off.

User's ask, verbatim: "create 2 video tiktok that showcase san_vpn, and san_medium. and if viewer
like to have, sawer it saweria in bio and dm me".

## Customizations

- CTA: "sawer di Saweria (link di bio), terus DM aku" (pay-to-get; the san_medium repo is not
  public).
- Sound: chat sounds only (message "ting", soft keyboard clicks). No music bed — the user adds a
  trending TikTok sound in the app.
- Look: dark chat, one accent colour for the tool, large monospace for command cards. Not
  WhatsApp's look or logo; the editor mock is generic (no Medium logo).

## Notes

- Claims only what golang/packages/san_medium/README.md backs: markdown → Medium draft, title,
  text, images with alt text and captions, uses your own signed-in Chrome, schedule publishing
  (Medium publishes it itself), list/read/delete drafts. Medium no longer issues API tokens.
- Command output is taken from the README example, trimmed to fit a phone.
- 8 bubbles in ~25 s: max 4 lines per card, large type, chat clear of TikTok's right rail and
  bottom caption band.
