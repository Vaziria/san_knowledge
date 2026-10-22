---
video: renders/video.mp4
tags: [vpn, wireguard, bikin vpn sendiri, server vpn, vps, linux, ubuntu, wg0.conf, ufw, jaringan, it indonesia]
at: 2026-10-23 19:00
---

# Bikin Server VPN Sendiri di VPS dengan WireGuard | Bikin VPN Sendiri Bagian 2

#vpn #wireguard #linux #vps

Enam langkah di VPS Ubuntu atau Debian, dari nol sampai server WireGuard-nya nyala.

0:00 Pembuka
0:19 1. Masuk ke VPS
0:36 2. Install WireGuard
0:52 3. Bikin sepasang kunci
1:14 4. Tulis config wg0.conf
1:44 5. Buka port
2:03 6. Nyalakan dan cek
2:21 Kalau gagal

Ringkasnya:
1. ssh user@IP_VPS
2. sudo apt update, lalu sudo apt install wireguard
3. umask 077, lalu bikin server.key dan server.pub dengan wg genkey dan wg pubkey (perintah lengkapnya di video)
4. /etc/wireguard/wg0.conf: Address = 10.8.0.1/24, ListenPort = 51820, PrivateKey = isi server.key, PostUp = sysctl -w net.ipv4.ip_forward=1
5. sudo ufw allow 51820/udp dan sudo ufw route allow in on wg0 out on wg0, plus buka UDP 51820 di firewall panel VPS-mu
6. sudo systemctl enable --now wg-quick@wg0, cek dengan sudo wg

server.key itu kunci rahasia: jangan pernah dibagi.

Bagian 1: kenapa PC kantor gak bisa diremote. Bagian 3: sambungkan PC rumah dan kantor, lalu Remote Desktop.
