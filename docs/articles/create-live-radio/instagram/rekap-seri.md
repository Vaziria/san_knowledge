---
at: 2026-11-03 19:00
---

#raspberrypi #youtube #livestreaming #ffmpeg #golang #programming #itindonesia

Channel musik santai 24 jam di YouTube ternyata bisa berjalan di sebuah Raspberry Pi, komputer kecil seukuran kartu nama.

Di carousel ini: bagaimana lagu, video latar, dan teks live masuk ke satu ffmpeg, bagaimana komentar penonton dijawab di layar, dan apa yang dikorbankan desain seperti ini.

Saya membangunnya dari nol dengan Go dan ffmpeg dalam seri 5 bagian. Seri lengkapnya ada di Medium: medium.com/@archetipe

![Sampul: judul Radio musik 24 jam di YouTube, dari sebuah Raspberry Pi, di atas gambar layar radio berbentuk jendela terminal gelap yang mengetik perintah play untuk setiap lagu.](rekap-seri/slide-01.png)
![Diagram: di dalam Raspberry Pi, folder musik, video latar, teks live, dan program pengendali masuk ke ffmpeg. ffmpeg mengirim siaran ke server YouTube lewat RTMP, lalu ke penonton. Komentar dari live chat dibaca lagi oleh program.](rekap-seri/slide-02.png)
![Layar radio berbentuk terminal yang menyapa penonton: setelah perintah greet dengan komentar penonton, terminal menjawab Hello dengan nama penonton itu.](rekap-seri/slide-03.png)
![Diagram radio yang utuh di dalam Raspberry Pi: cron menjalankan program radio, yang berisi playlist, pengumpan suara, teks layar, pemilih encoder, dan pembaca live chat, semuanya menuju ffmpeg lalu YouTube.](rekap-seri/slide-04.png)
![Tiga kelemahan: berhenti saat listrik atau internet rumah putus, upload sekitar 50 GB sebulan, dan sapaan bisa terlambat hampir satu menit karena kuota API.](rekap-seri/slide-05.png)
![Penutup: seri lengkapnya 5 bagian di Medium, medium.com/@archetipe, mulai dari Bagian 1, Apa itu radio live 24 jam di YouTube.](rekap-seri/slide-06.png)
