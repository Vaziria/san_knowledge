---
video: renders/video.mp4
at: 2026-11-21 19:00
---

#messagebroker #golang #goroutine #itindonesia #fyp

Satu webhook lambat, semua ikut nunggu? Satu goroutine diganti worker pool. Tiap subscriber pegang paling banyak satu worker, jadi urutan pesannya terjaga, dan giliran dimulai dari yang nunggu duluan. Dua worker sudah cukup buat satu webhook yang lambat. Bagian 5 dari 6.
