---
video: renders/video.mp4
tags: [database, key-value store, bikin database sendiri, golang, go, backend, redis, bitcask, fsync, log, index, programming, it indonesia]
at: 2026-11-25 19:00
---

# Kenapa Map di Memori Gak Cukup untuk Database? | Bikin Database Key-Value Sendiri Bagian 1

#database #golang #backend #itindonesia

Seri baru: bikin database key-value kecil dengan Go dari nol, cuma pakai library standar, dalam enam bagian. Di bagian 1: apa itu database key-value, kenapa map di memori hilang saat program berhenti, dan versi pertama kvku yang menyimpan data di log di disk dengan map sebagai index, seperti Redis AOF dan Bitcask. Lalu write lawan fsync, kenapa index harus urut untuk Range, anatomi record dengan header 12 byte dan CRC-32C, tulisan yang terpotong saat listrik padam, dan file berisi satu juta key yang makin lama dibuka.

0:00 Pembuka: apa itu database key-value
0:20 Rencana enam bagian
0:32 Kenapa map di memori gak cukup
0:52 Log di disk, map sebagai index
1:12 Kapan data sampai ke disk
1:32 Kenapa index harus urut
1:51 Anatomi record
2:19 Mencoba kvku dari terminal
2:37 Satu juta key, dan kill -9
2:55 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/d2c94671f110

Bagian 2: B+tree di memori, dengan node, leaf, pembelahan node, dan Range yang urut.
