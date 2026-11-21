---
video: renders/video.mp4
tags: [database, key-value store, bikin database sendiri, compaction, free list, lru cache, mmap, golang, go, backend, badger, bbolt, programming, it indonesia]
at: 2026-12-04 19:00
---

# Compaction, Free List, dan Data 9 GB di RAM 7,4 GB | Bikin Database Key-Value Sendiri Bagian 5

#database #golang #backend #itindonesia

Bagian 5 seri bikin database key-value sendiri dengan Go. Copy-on-write di Bagian 4 meninggalkan halaman lama di file index dan record lama di file data. Di bagian ini: free list dengan daftar tunggu satu commit, kenapa free list disimpan di file, ambang compaction 75% dibandingkan saran Badger, compaction yang tidak menghentikan pembaca dengan dua kunci, cache LRU, lalu kvku diisi data 9,3 GB di laptop ber-RAM sekitar 7,4 GB. Terakhir, mmap ditimbang sekali lagi, kali ini dengan angka.

0:00 Pembuka: sampah di kvku
0:20 Free list dan daftar tunggu
0:55 Kapan file data dipadatkan
1:22 Compaction tanpa menghentikan pembaca
1:42 Hasil test compaction
2:00 Cache LRU
2:13 Data lebih besar dari RAM
2:33 Menimbang mmap dengan angka
2:53 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/948816b47681

Bagian 6, bagian terakhir: kvku diukur berdampingan dengan bbolt dan Badger, lalu seri ditutup.
