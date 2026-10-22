---
workflow: general-video
flow: automation
storyboard: no
message: "Connect the office and home PCs to your VPN server: WireGuard app, client config, a [Peer] per PC on the server, then ping and Remote Desktop"
destination: tiktok
aspect: 1080x1920
language: id
audience: "Indonesian devs, IT admins and power users stuck behind CGNAT / office firewalls"
length: 35s
angle: code-cards
series: "Bikin VPN sendiri, part 3 of 3 (part 1: videos/vpn-sendiri-1-kenapa, part 2: videos/vpn-sendiri-2-server)"
---

## Intent

Part 3 of the series the user asked for on 2026-10-21 (see part 1's brief): big code cards, a
2-second kinetic hook, ending on a working ping and Remote Desktop, then the setup-help CTA the
user's first VPN video used (Saweria in bio, DM). No voice.

## Notes

- Steps: the WireGuard app (wireguard.com/install), Add Tunnel > Add empty tunnel (keys made for you);
  office PC config: Address 10.8.0.3/24, [Peer] with the server's public key, Endpoint IP_VPS:51820,
  AllowedIPs 10.8.0.0/24, PersistentKeepalive 25 (keeps the tunnel open behind CGNAT); home PC the
  same with 10.8.0.2; on the server one [Peer] per PC (AllowedIPs /32) and `systemctl restart
  wg-quick@wg0`; Activate on both, `ping 10.8.0.3`, `mstsc /v:10.8.0.3`.
- Caveats for the post text (not the video): Windows Firewall blocks ping by default; Remote Desktop
  needs Windows Pro with Remote Desktop on; some office networks block UDP.
- The CTA is the user's own from the first VPN video; drop it if they say so at the preview.
- Must not mention san_vpn. Design truth: frame.md (Broadside). Built by make.mjs from a spec.
