---
video: renders/video.mp4
tags: [message broker, bikin message broker sendiri, golang, go, backend, worker pool, goroutine, webhook, concurrency, kafka, programming, it indonesia]
at: 2026-11-20 19:00
---

# Satu Webhook Lambat, Semua Ikut Nunggu? Pakai Worker Pool | Bikin Message Broker Sendiri Bagian 5

#messagebroker #golang #backend #itindonesia

Bagian 5 dari seri bikin message broker sendiri dengan Go. Di bagian 4, satu webhook yang lambat membuat subscriber webhook lain ikut menunggu. Di video ini: head-of-line blocking, worker pool lawan satu goroutine per subscriber, anatomi worker pool, kenapa urutan pesan tetap terjaga dengan paling banyak satu worker per subscriber, siapa yang dapat giliran saat semua worker sibuk, benchmark 1, 2, 4, dan 8 worker, dan percobaan dengan webhook yang butuh 4 detik per pesan.

0:00 Pembuka: head-of-line blocking
0:24 Worker pool atau goroutine per subscriber
0:41 Anatomi worker pool
1:08 Urutan pesan per subscriber
1:27 Giliran saat semua worker sibuk
2:02 Berapa worker yang dibutuhkan
2:23 Dicoba dengan webhook yang lambat
2:42 Kelemahan dan bagian terakhir

Artikel lengkapnya di Medium, dengan kode Go, test, benchmark, dan percobaannya: https://medium.com/p/22761fe95d8c

Bagian 6, bagian terakhir: tiga cara menjalankan broker, lalu kapan RabbitMQ, NATS, atau Kafka lebih tepat.
