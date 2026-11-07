---
video: renders/video.mp4
at: 2026-11-12 19:00
---

#messagebroker #golang #backend #itindonesia #fyp

Bikin message broker sendiri dengan Go. Pesan ditulis sekali di log, tiap subscriber cuma punya satu angka: posisi baca. Protokolnya 4 alamat HTTP, bukan AMQP. Tiap record dijaga CRC-32C, dan fsync sekali tiap detik. Bagian 1 dari 6.
