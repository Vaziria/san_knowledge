---
video: renders/video.mp4
tags: [database, key-value store, bikin database sendiri, b+tree, mmap, golang, go, backend, page cache, innodb, bbolt, programming, it indonesia]
at: 2026-11-29 19:00
---

# B+tree di Halaman 16 KB, Lalu Menimbang mmap | Bikin Database Key-Value Sendiri Bagian 3

#database #golang #backend #itindonesia

Bagian 3 seri bikin database key-value sendiri dengan Go. B+tree pindah dari memori ke file yang dibagi menjadi halaman 16 KB, seperti InnoDB, supaya program gak perlu membaca seluruh file saat mulai. Di bagian ini: kenapa halaman berukuran tetap, anatomi file kvku dari halaman meta sampai leaf, susunan byte di setiap halaman, ReadAt lawan mmap dan kenapa kvku belum memakai mmap, urutan Sync dan risiko menimpa halaman di tempatnya, lalu file berisi satu juta key yang dijawab dengan membaca tiga halaman.

0:00 Pembuka: B+tree yang masih di memori
0:20 Kenapa halaman berukuran tetap
0:50 Anatomi file kvku
1:17 Susunan byte di halaman
1:37 ReadAt atau mmap?
1:57 Kenapa kvku belum pakai mmap
2:17 Sync dan risiko menimpa halaman
2:37 Satu juta key di file
2:55 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/81f8c661065e

Bagian 4: update dan delete tanpa merusak data, dengan copy-on-write dan dua halaman meta.
