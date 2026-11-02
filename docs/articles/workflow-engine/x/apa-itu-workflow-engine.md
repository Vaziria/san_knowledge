---
at: 2026-11-06 19:00
---

Banyak otomatisasi kecil lahir sebagai script yang dijalankan cron. Lalu suatu malam layanan AI terganggu, dan kamu baru tahu besok.

Di sinilah workflow engine berguna. Ini cara kerjanya, dan kapan script dengan cron sebenarnya sudah cukup.

---

Saat ada yang tidak lancar, script dengan cron membawa empat masalah:

- gagal tanpa jejak
- mengulang berarti mengulang semuanya
- tidak pandai menunggu manusia
- setiap script menulis ulang token, cara mencoba ulang, dan log

---

Workflow adalah graf. Setiap langkah adalah node, dan edge menentukan node mana yang berjalan berikutnya.

Trigger memulai run. Engine mengikuti edge dari node ke node, lalu mencatat input, output, dan error setiap node di log run.

---

Contoh satu run. Jam 19:00 trigger menyala, node baca mengambil catatan baru, node draf meminta AI membuat draf, lalu node setuju mengirimnya ke Telegram.

Run berhenti dan disimpan di database sampai kamu menekan Setujui. Inilah human-in-the-loop.

---

Engine bisa mencoba ulang node yang gagal, tapi itu hanya aman untuk node yang idempotent.

Pesan Telegram yang sudah terkirim lalu dicoba ulang membuatmu menerima dua pesan. Node seperti itu perlu memeriksa dulu apakah pekerjaannya sudah terjadi.

---

Lima engine yang akan sering kamu temui: Zapier tanpa kode, n8n yang bisa kamu jalankan sendiri, Node-RED untuk perangkat dan sensor, Airflow dengan kode Python, dan Temporal yang tetap berlanjut walau server mati.

---

Kelemahannya, engine adalah satu program lagi yang harus dirawat. Script dengan cron tetap cukup untuk pekerjaan yang boleh diulang dari awal.

Artikel lengkapnya, "Apa itu workflow engine, dan kapan script dengan cron saja tidak cukup?":
{{MEDIUM_URL}}
