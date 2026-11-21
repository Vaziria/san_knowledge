---
video: renders/video.mp4
tags: [database, key-value store, bikin database sendiri, b+tree, btree, golang, go, backend, struktur data, algoritma, programming, it indonesia]
at: 2026-11-27 19:00
---

# B+tree dari Nol: Range 300 Kali Lebih Cepat | Bikin Database Key-Value Sendiri Bagian 2

#database #golang #backend #itindonesia

Bagian 2 seri bikin database key-value sendiri dengan Go. Map yang jadi index di Bagian 1 gak nyimpan urutan, jadi Range harus memeriksa semua key. Di bagian ini kita bandingkan slice yang diurutkan, binary search tree, dan B+tree, lalu membangun B+tree di memori: anatomi root, branch, dan leaf, cara kerja Get, Put dengan pembelahan node, dan Range di rantai leaf. B+tree diuji dengan map sebagai kunci jawaban, dibandingkan dengan map lewat benchmark, lalu dipakai sebagai index kvku.

0:00 Pembuka: Range di Bagian 1
0:20 Slice urut atau binary search tree?
0:40 Fan-out: banyak key per node
0:55 Anatomi B+tree dan Get
1:22 Put dan pembelahan node
1:43 Range di rantai leaf
1:59 Menguji dengan map sebagai kunci jawaban
2:19 B+tree lawan map
2:36 Range 300 kali lebih cepat
2:53 Kelemahan dan bagian berikutnya

Artikel lengkapnya di Medium, dengan kode Go, test, dan hasil benchmark-nya: https://medium.com/p/57981f8b7d57

Bagian 3: B+tree disimpan di file yang dibagi menjadi halaman 16 KB, lalu menimbang mmap.
