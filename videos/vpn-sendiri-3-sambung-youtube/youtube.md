---
video: renders/video.mp4
tags: [vpn, wireguard, bikin vpn sendiri, remote desktop, remote pc kantor, windows, vps, jaringan, it indonesia]
at: 2026-10-24 19:00
---

# Sambungkan PC Rumah dan Kantor, Lalu Remote Desktop | Bikin VPN Sendiri Bagian 3

#vpn #wireguard #remotedesktop #itindonesia

Server VPN-nya sudah nyala (bagian 2). Sekarang PC kantor (10.8.0.3) dan PC rumah (10.8.0.2) disambungkan, lalu Remote Desktop dari rumah.

0:00 Pembuka
0:20 1. Pasang aplikasi WireGuard
0:37 2. Config PC kantor
1:03 3. Config PC rumah
1:15 4. Daftarkan di server
1:40 5. Activate dan cek
1:58 6. Remote Desktop
2:20 Kalau belum jalan

Ringkasnya:
1. Aplikasi WireGuard dari wireguard.com/install, lalu Add Tunnel, Add empty tunnel. Kuncinya dibuat otomatis, salin public key-nya
2. PC kantor: Address = 10.8.0.3/24; di [Peer]: PublicKey = public key server, Endpoint = IP VPS-mu:51820, AllowedIPs = 10.8.0.0/24, PersistentKeepalive = 25
3. PC rumah sama, Address-nya 10.8.0.2
4. Di server, satu [Peer] untuk tiap PC (PublicKey PC itu, AllowedIPs 10.8.0.3/32 dan 10.8.0.2/32), lalu sudo systemctl restart wg-quick@wg0
5. Klik Activate di kedua PC; di VPS, sudo wg harus menunjukkan latest handshake
6. Di PC kantor nyalakan Remote Desktop (Windows Pro), lalu dari rumah: ping 10.8.0.3 dan mstsc /v:10.8.0.3

Kalau belum jalan:
- Ping gagal: Windows Firewall memblokir ping secara bawaan, izinkan di PC kantor
- Gak ada handshake: cek Endpoint, port 51820/UDP, dan kuncinya
- Sebagian jaringan kantor memblokir UDP, dan WireGuard butuh UDP

Mentok di langkah mana? Tulis di komentar.
