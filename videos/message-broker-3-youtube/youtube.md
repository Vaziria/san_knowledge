---
video: renders/video.mp4
tags: [message broker, bikin message broker sendiri, golang, go, backend, http2, push pull, ack, kafka, rabbitmq, nats, programming, it indonesia]
at: 2026-11-15 19:00
---

# Push atau Pull? Pesan dan Ack Lewat Satu Request HTTP/2 | Bikin Message Broker Sendiri Bagian 3

#messagebroker #golang #backend #itindonesia

Bagian 3 dari seri bikin message broker sendiri dengan Go: pesan mulai sampai ke program lain. Subscriber membuka satu stream HTTP/2 ke broker, menerima pesan satu per satu, dan mengirim ack untuk setiap pesan. Di video ini: push lawan pull, anatomi stream pull, kenapa bukan WebSocket, prefetch 1 dan timeout ack, satu stream per subscriber dengan ping HTTP/2, loop pengantar, dan percobaan brokerku dengan pembaca di Windows.

0:00 Pembuka: push atau pull
0:27 Anatomi stream pull
0:51 Kenapa satu request HTTP/2
1:11 Prefetch 1 dan timeout ack
1:35 Satu stream per subscriber
1:57 Loop pengantar
2:19 Mencoba brokerku dan pembaca
2:39 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan percobaannya: https://medium.com/p/87222de2c9f8

Bagian 4: subscriber webhook dan percobaan ulang, dengan jeda yang dimulai dari 1 detik dan berlipat dua sampai paling lama 5 menit.
