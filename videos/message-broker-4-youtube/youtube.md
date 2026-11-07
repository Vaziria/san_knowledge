---
video: renders/video.mp4
tags: [message broker, bikin message broker sendiri, golang, go, backend, webhook, retry, backoff, kafka, rabbitmq, nats, programming, it indonesia]
at: 2026-11-18 19:00
---

# Webhook Gagal? Coba Lagi 1, 2, 4, 8 Detik, Lalu Menyerah | Bikin Message Broker Sendiri Bagian 4

#messagebroker #golang #backend #itindonesia

Bagian 4 dari seri bikin message broker sendiri dengan Go: broker mulai mendatangi subscriber lewat webhook. Di video ini: anatomi satu panggilan webhook, kegagalan mana yang layak dicoba lagi (timeout, koneksi putus, 5xx, 408, 429) dan mana yang langsung dilewati, jeda yang dimulai dari 1 detik dan berlipat dua sampai paling lama 5 menit, kenapa pesan dilewati setelah 10 kali gagal, percobaan ulang yang disimpan di slot subscriber dan berlaku juga untuk stream pull, percobaan dengan program penerima di Windows, dan kenapa satu webhook yang lambat masih menahan subscriber lain.

0:00 Pembuka: pull lawan webhook
0:24 Anatomi satu panggilan webhook
0:44 Kegagalan mana yang dicoba lagi
1:10 Jeda yang berlipat dua
1:31 Kenapa menyerah setelah 10 kali
1:48 Tahan restart, berlaku juga untuk pull
2:06 Mencoba dengan penerima
2:25 Webhook lambat menahan yang lain
2:42 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan percobaannya: https://medium.com/p/6964d35a524b

Bagian 5: worker pool, supaya webhook yang lambat tidak menahan subscriber lain.
