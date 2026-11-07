---
video: renders/video.mp4
tags: [message broker, bikin message broker sendiri, golang, go, backend, rabbitmq, amqp, kafka, http, fsync, programming, it indonesia]
at: 2026-11-11 19:00
---

# AMQP Punya 53 Method, Broker Kita Cukup 4 Alamat HTTP | Bikin Message Broker Sendiri Bagian 1

#messagebroker #golang #backend #itindonesia

Seri baru: bikin message broker kecil dengan Go dari nol, cuma pakai library standar, dalam enam bagian. Di bagian 1: kenapa broker ini gak meniru queue RabbitMQ, kenapa protokolnya bikin sendiri di atas HTTP dan bukan AMQP 0-9-1, lalu cara pesan disimpan di log: record dengan header 24 byte dan CRC-32C, write lawan fsync, dan record yang terpotong saat listrik padam.

0:00 Pembuka: apa itu message broker
0:21 Rencana enam bagian
0:33 Satu log, satu posisi baca
0:55 AMQP 0-9-1
1:15 Protokol sendiri di atas HTTP
1:37 Kenapa log, dan anatomi satu record
2:04 write lawan fsync
2:24 Record yang terpotong
2:44 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/e403943bae69

Bagian 2: broker mengingat posisi baca setiap subscriber, juga setelah mati mendadak.
