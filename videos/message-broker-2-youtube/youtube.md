---
video: renders/video.mp4
tags: [message broker, bikin message broker sendiri, golang, go, backend, kafka, posisi baca, mmap, fsync, metadata, programming, it indonesia]
at: 2026-11-13 19:00
---

# Broker Mati Mendadak, Subscriber Lanjut dari Mana? | Bikin Message Broker Sendiri Bagian 2

#messagebroker #golang #backend #itindonesia

Bagian 2 dari seri bikin message broker sendiri dengan Go. Broker mulai mengingat topic, subscriber, dan posisi baca setiap subscriber: kenapa posisi cukup satu angka, kenapa slot berukuran tetap dan bukan mmap, urutan fsync log lalu metadata, apa yang diperiksa saat broker dibuka lagi, dan kenapa ID dicadangkan per 1.000.

0:00 Pembuka: yang harus diingat broker
0:21 Posisi baca cukup satu angka
0:39 Tiga cara menyimpan posisi
0:59 Kenapa bukan mmap
1:14 Anatomi file metadata
1:36 Urutan menulis ke disk
1:56 Saat broker dibuka lagi
2:27 Kenapa ID tidak boleh kembar
2:47 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go dan test-nya: https://medium.com/p/7844af967263

Bagian 1: protokol sendiri atau AMQP, lalu menyimpan pesan di log. Bagian 3: subscriber yang menarik pesannya sendiri lewat satu request HTTP/2.
