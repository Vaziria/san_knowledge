---
workflow: general-video
flow: automation
storyboard: no
message: "Build your own VPN server on a VPS in 5 steps: install WireGuard, make a key pair, write wg0.conf, open the port, turn it on"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls"
length: 34s
angle: code-cards
series: "Bikin VPN sendiri, part 2 of 3 (part 1: videos/vpn-sendiri-1-kenapa, part 3: videos/vpn-sendiri-3-sambung)"
---

## Intent

Part 2 of the series the user asked for on 2026-10-21 (see part 1's brief): big code cards that
viewers can follow and save, opening with a 2-second kinetic hook. No voice.

## Notes

- Steps (Ubuntu/Debian VPS): `sudo apt install wireguard`; `wg genkey | tee server.key | wg pubkey >
  server.pub`; `/etc/wireguard/wg0.conf` with Address 10.8.0.1/24, ListenPort 51820, PrivateKey and
  `PostUp = sysctl -w net.ipv4.ip_forward=1` (so home and office can reach each other through the VPS);
  `ufw allow 51820/udp` and `ufw route allow in on wg0 out on wg0`, plus the provider's firewall;
  `systemctl enable --now wg-quick@wg0`, checked with `sudo wg`.
- Peers are added in part 3, once the PCs have keys.
- Must not mention san_vpn. Design truth: frame.md (Broadside).
- The cards are built from a spec by make.mjs (shared code in videos/vpn-sendiri-shared/cards.mjs);
  edit the spec and rerun `node make.mjs`, not index.html.
