---
video: renders/video.mp4
tags: [database, key-value store, bikin database sendiri, benchmark, bbolt, badger, sqlite, pebble, lsm tree, b+tree, golang, go, backend, programming, it indonesia]
at: 2026-12-06 19:00
---

# kvku Lawan bbolt dan Badger, Lalu Menutup Seri | Bikin Database Key-Value Sendiri Bagian 6

#database #golang #backend #itindonesia

Bagian 6, bagian terakhir seri bikin database key-value sendiri dengan Go. kvku diukur berdampingan dengan bbolt dan Badger di laptop yang sama: Put, Get, Range, Open, dan ukuran file. Di bagian ini: tiga desain key-value, cara mengukur yang adil, kenapa SyncWrites Badger di Windows tidak menunggu disk, perbaikan kecil yang membuat Range sepuluh kali lebih cepat, kelemahan kvku secara keseluruhan, kapan memakai SQLite, bbolt, Badger, atau Pebble, dan rekap enam bagian.

0:00 Pembuka: seberapa jauh kvku?
0:20 Tiga cara menyimpan key-value
0:40 Cara mengukur yang adil
0:53 Hasil: Put, Get, Range, Open
1:20 Badger di Windows gak menunggu disk
1:38 Ukuran file
1:51 Range sepuluh kali lebih cepat
2:11 Kelemahan kvku
2:27 Kapan SQLite, bbolt, Badger, atau Pebble
2:43 Rekap enam bagian
2:56 Penutup seri

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/82f2c0ac6a9e

Ini bagian terakhir seri. Terima kasih sudah mengikuti sampai akhir. Kalau belum, mulai dari Bagian 1: kenapa map di memori gak cukup untuk database.
