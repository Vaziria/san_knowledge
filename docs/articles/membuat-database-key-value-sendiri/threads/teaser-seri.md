---
at: 2026-11-24 19:00
topic: Programming
---

Hampir setiap aplikasi mulai dengan menyimpan data di memori. Sebuah map, kunci dan nilainya, cepat dan sederhana.

Lalu programnya di-restart, yaitu dimatikan lalu dinyalakan lagi, dan semua isinya hilang. Mulai besok, seri baru Membuat Database Key-Value Sendiri membangun tempat penyimpanan di disk dari nol dengan Go.

---

Key-value store adalah database yang paling sederhana. Setiap data punya kunci, dan kamu hanya melakukan tiga hal, yaitu menyimpan nilai untuk sebuah kunci, membacanya lagi, dan menghapusnya.

Tapi di balik tiga hal itu ada pertanyaan yang sama dengan database besar. Bagaimana data bertahan saat listrik padam, dan bagaimana data ditemukan dengan cepat saat ukurannya jauh lebih besar dari memori.

---

Kenapa tidak menulis map itu ke sebuah file JSON saja?

Untuk data kecil, itu memang cukup. Masalahnya, setiap perubahan menulis ulang seluruh file, dan program yang mati di tengah penulisan bisa meninggalkan file setengah jadi. Saat datanya tumbuh, membaca satu kunci juga berarti membuka semuanya.

---

Jawaban yang dipakai banyak database adalah indeks yang terurut. Dengan kunci yang terurut, satu kunci ditemukan dalam sedikit langkah, dan semua kunci di antara dua nilai bisa dibaca berurutan.

Seri ini memakai B+tree, struktur yang menyimpan kunci di node-node yang lebar, sehingga pohonnya rendah dan setiap pencarian hanya menyentuh sedikit halaman di disk.

---

Seri ini terdiri dari enam bagian, dan setiap bagian menambah satu kemampuan.

Bagian 1 membahas kenapa map di memori tidak cukup.
Bagian 2 membangun B+tree, strukturnya dan cara mencarinya.
Bagian 3 menyimpannya ke file dalam halaman 16 KB, dan membacanya lewat mmap.

---

Bagian 4 mengubah dan menghapus data tanpa merusak data yang sudah ada.
Bagian 5 membahas compaction, dan data yang lebih besar dari RAM.
Bagian 6 mengukur hasilnya, membandingkannya dengan BoltDB dan Badger, lalu menutup seri.

Semuanya ditulis dari nol dengan Go, hanya dengan library standar.

---

Database buatan sendiri tidak dimaksudkan untuk menggantikan database yang sudah matang. Tujuannya memahami keputusan di dalamnya, supaya saat memilih database, kamu tahu pertanyaan apa yang harus diajukan.

Bagian 1 terbit besok di Medium.
