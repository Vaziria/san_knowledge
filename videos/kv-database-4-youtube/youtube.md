---
video: renders/video.mp4
tags: [database, key-value store, bikin database sendiri, b+tree, copy-on-write, golang, go, backend, tombstone, fsync, lmdb, bbolt, programming, it indonesia]
at: 2026-12-02 19:00
---

# Update dan Delete Tanpa Merusak Data: Copy-on-Write | Bikin Database Key-Value Sendiri Bagian 4

#database #golang #backend #itindonesia

Bagian 4 seri bikin database key-value sendiri dengan Go. Di Bagian 3, Sync menimpa halaman di tempatnya, jadi listrik padam di tengah commit bisa meninggalkan leaf baru dengan induk lama. Di bagian ini: journal lawan copy-on-write, menyalin jalur dari leaf sampai root, kenapa rantai leaf harus dilepas, dua meta A dan B yang ditulis bergantian, value yang pindah ke file data yang cuma bertambah, tombstone, dan delete yang menggabung atau meminjam dari node saudara. Lalu 94 listrik padam yang disimulasikan, program yang dihentikan paksa, dan kvku dari terminal.

0:00 Pembuka: halaman yang ditimpa
0:20 Journal atau copy-on-write?
0:44 Copy-on-write di B+tree
1:04 Kenapa rantai leaf dilepas
1:19 Dua meta, A dan B
1:39 Value di file data, dan tombstone
1:59 Delete: gabung atau pinjam
2:17 Listrik padam dan kill
2:37 Mencoba kvku dari terminal
2:55 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/6adc9847b43b

Bagian 5: free list, compaction yang tidak menghentikan pembaca, dan data yang lebih besar dari RAM.
