# Kenapa AI agent lupa proyekmu setiap sesi, dan kapan ia butuh knowledge graph?

*AI agent seperti Claude Code, Codex, dan Cursor memulai setiap sesi tanpa ingatan tentang proyekmu. Artikel ini membahas kenapa itu terjadi, bagaimana knowledge graph dari dokumen proyek membantu, dan kapan grep, sebuah CLAUDE.md, atau RAG sudah cukup.*

AI agent untuk menulis kode, seperti Claude Code, Codex, dan Cursor, kini bisa membaca file, menjalankan perintah, dan mengubah kode di proyekmu. AI agent adalah program yang memakai model AI untuk mengerjakan tugas langkah demi langkah. Tetapi setiap kali kamu membuka sesi baru, ia kembali tidak tahu keputusan yang kemarin sudah kalian bahas.

Artikel ini membahas kenapa itu terjadi, tiga cara yang biasa dipakai untuk mengatasinya, dan knowledge graph proyek yang akan kita bangun sebagai jawaban. Saya memakai contoh sebuah toko online yang dokumennya ditulis dalam markdown, yaitu format teks dengan tanda sederhana untuk heading dan link. Heading adalah judul sebuah bagian dokumen, seperti judul-judul di artikel ini.

## Kenapa AI agent lupa isi proyekmu di setiap sesi baru?

AI agent lupa karena semua yang ia ketahui selama bekerja tersimpan di context window, dan context window itu kosong lagi di awal setiap sesi. Context window adalah seluruh teks yang bisa dibaca model saat menyusun jawaban, termasuk jawaban itu sendiri.

Panjang context window dihitung dalam token, yaitu potongan kecil teks yang dibaca model. Pesanmu, file yang dibaca agent, dan hasil perintahnya, semuanya menambah token. Dokumentasi Claude Code menulis bahwa setiap sesi Claude Code dimulai dengan context window yang baru.

Dokumentasi Cursor juga menulis bahwa model bahasa tidak menyimpan ingatan dari satu jawaban ke jawaban berikutnya. Yang terbawa ke sesi baru hanyalah yang dimuat otomatis di awal sesi, terutama file instruksi, yaitu file aturan proyek yang dibahas di bagian berikutnya.

Bukankah context window yang makin besar akan menyelesaikan masalah ini? Dokumentasi Claude menyebut context window kini bisa mencapai 1 juta token, tergantung modelnya. Tetapi dokumentasi yang sama menyebut bahwa makin banyak token, makin turun ketepatan model dalam mengingat isinya, gejala yang disebut context rot.

Jadi, yang penting adalah memilih isi yang tepat untuk tugas yang sedang dikerjakan. Tanpa cara memilih itu, muncul tiga masalah. Ketiganya adalah:

- **Agent membaca ulang hal yang sama.** Di setiap sesi, agent membuka lagi file yang kemarin sudah ia baca, dan setiap file menghabiskan token.
- **Kode menjelaskan apa, bukan kenapa.** Alasan sebuah keputusan, seperti kenapa refund harus disetujui admin, biasanya tertulis di dokumen, bukan di kode. Agent yang hanya membaca kode bisa "merapikan" sesuatu yang sengaja dibuat begitu.
- **Jawaban agent sulit diperiksa.** Saat agent berkata "refund diproses dalam 7 hari kerja", kamu tidak tahu ia membacanya di mana, atau hanya menebak.

![Dua kotak context window berdampingan. Sesi 1 berisi file instruksi, pesan kamu, isi 12 file yang dibaca agent, hasil perintah, dan kesimpulan bahwa refund butuh persetujuan admin. Setelah sesi berakhir, sesi 2 hanya berisi file instruksi, dan sisanya kosong, sehingga agent harus membaca ulang file yang sama dari folder proyek.](assets/sesi-baru-context-kosong.png)

*Yang dibaca dan disimpulkan di sesi 1 tidak terbawa ke sesi 2.*

## Bukankah CLAUDE.md atau AGENTS.md sudah cukup?

Cara pertama adalah file instruksi tadi. Claude Code membaca CLAUDE.md di awal setiap sesi, Codex membaca AGENTS.md sebelum mengerjakan apa pun, dan Cursor memasukkan rules-nya, yaitu file aturan di folder proyek, di awal context window. Situs AGENTS.md menyebut format ini "README untuk agent", yaitu file pengantar proyek yang ditulis khusus untuk agent, dan menyebut lebih dari 60 ribu proyek open source sudah memakainya.

File seperti ini tepat untuk hal yang dibutuhkan di setiap sesi. Seperti perintah build, cara menjalankan test, dan aturan penamaan. Isinya ditulis manusia, jadi bisa dipercaya.

Masalahnya, file ini dimuat utuh ke setiap sesi, sehingga harus kecil. Dokumentasi Claude Code menganjurkan setiap CLAUDE.md di bawah 200 baris, karena file yang lebih panjang memakan lebih banyak context window dan membuat instruksinya kurang dipatuhi. Codex, secara bawaan, berhenti menambah isi AGENTS.md setelah ukuran gabungannya mencapai 32 KiB.

Claude Code juga punya auto memory, yaitu catatan yang ditulis Claude sendiri dari koreksimu. Tetapi catatan ini pun dibatasi, karena hanya 200 baris pertama atau 25KB pertamanya yang dimuat di awal sesi.

Jadi, file instruksi cocok untuk aturan yang berlaku di setiap sesi, tetapi tidak muat untuk seluruh pengetahuan proyek. Dokumen desain, catatan keputusan, dan hasil riset butuh cara lain, supaya diambil hanya saat dibutuhkan.

## Kenapa AI agent tidak mencari dengan grep saja?

Cara kedua adalah membiarkan agent mencari sendiri dengan grep, yaitu perintah yang mencari baris teks yang cocok dengan sebuah pola. Anthropic menulis bahwa Claude Code memakai cara campuran. CLAUDE.md dimasukkan ke context window sejak awal, sedangkan glob, yaitu pencarian file menurut pola nama, dan grep dipakai untuk mengambil file tepat saat dibutuhkan.

Kelebihan cara ini nyata. Grep selalu membaca isi file yang terbaru, dan tidak butuh biaya untuk membangun apa pun. Anthropic menyebut cara ini menghindari masalah indeks yang usang.

Tapi grep mencari kata, bukan makna, dan tidak tahu hubungan antardokumen. Jika dokumenmu menulis "pengembalian dana" sementara agent mencari "refund", grep tidak menemukan apa-apa. Anthropic juga menyebut eksplorasi seperti ini lebih lambat daripada mengambil data yang sudah diolah sebelumnya.

Jadi, grep cocok untuk mencari nama yang persis. Seperti nama fungsi, pesan error, atau nama variabel lingkungan. Untuk pertanyaan "kenapa" dan "apa yang berhubungan dengan ini", agent butuh peta yang sudah disiapkan.

## Kenapa AI agent tidak memakai RAG dengan embedding saja?

Cara ketiga adalah RAG, singkatan dari retrieval-augmented generation, yaitu mengambil potongan teks yang relevan dulu, lalu memberikannya ke model sebagai bahan jawaban. Istilah ini berasal dari makalah Patrick Lewis dan rekan-rekannya, yang pertama kali terbit pada Mei 2020. Kebanyakan RAG mencari potongan teks itu dengan embedding.

Embedding adalah deretan angka yang mewakili makna sebuah teks. Dokumentasi OpenAI menjelaskan bahwa jarak antara dua embedding mengukur seberapa berhubungan kedua teksnya. Karena itu, pertanyaan tentang "refund" bisa menemukan paragraf tentang "pengembalian dana", dan inilah yang tidak bisa dilakukan grep.

Masalahnya, potongan teks itu kehilangan tempatnya di dokumen. Microsoft Research menulis bahwa RAG biasa kesulitan "menghubungkan titik-titik", yaitu menjawab pertanyaan yang butuh informasi dari beberapa tempat. Indeks embedding juga harus dihitung ulang saat dokumen berubah, sehingga bisa usang.

Jadi, RAG cocok untuk teks yang tidak punya struktur, seperti arsip email atau transkrip rapat. Untuk dokumen proyek yang punya heading dan link, struktur itu terlalu berharga untuk dibuang.

## Anatomi knowledge graph sebuah proyek

Tiga cara tadi meninggalkan tiga celah. File instruksi harus kecil, grep tidak tahu makna dan hubungan, dan RAG membuang struktur dokumen. Knowledge graph proyek yang akan kita bangun mencoba menutup ketiganya.

Knowledge graph adalah cara menyimpan pengetahuan sebagai node dan edge. Node adalah satu hal yang bisa ditunjuk, seperti sebuah dokumen, dan edge adalah hubungan bernama antara dua node, seperti "bagian ini merujuk ke bagian itu". Dokumen markdown sudah berisi node dan edge itu, dalam bentuk heading dan link.

Knowledge graph ini punya tiga jenis node. Ketiganya adalah:

- **Dokumen**, satu untuk setiap file markdown. Judulnya diambil dari heading pertama.
- **Bagian**, satu untuk setiap heading lain, yaitu teks di bawah sebuah heading sampai heading berikutnya. Setiap bagian menyimpan lokasinya, berupa nama file dan nomor baris heading-nya.
- **Domain**, yaitu kelompok topik besar di proyekmu, seperti "Pembayaran" atau "Layanan pelanggan".

Node-node itu dihubungkan oleh tiga jenis edge. Ketiganya adalah:

- **Bagian dari**, dari sebuah bagian ke dokumennya, atau ke bagian induknya.
- **Merujuk ke**, dari bagian yang berisi sebuah link ke dokumen atau bagian yang dituju link itu.
- **Termasuk domain**, dari dokumen atau bagian ke domain-nya.

Edge "merujuk ke" adalah yang paling berharga, karena ia lahir dari link yang sudah ditulis manusia. Misalnya, bagian "Keluhan pelanggan" di file `support.md` berisi sebuah link. Isinya adalah:

```markdown
Jika pembeli meminta dananya kembali, ikuti [alur refund](pembayaran.md#refund).
```

Link itu menjadi satu edge dari bagian "Keluhan pelanggan" ke bagian "Refund" di `pembayaran.md`. Edge ini dibuat tanpa AI, tanpa menebak, dan tanpa embedding.

Setiap node juga menyimpan ringkasan singkat, beberapa kata kunci, lokasinya, dan fingerprint teksnya, yaitu angka pendek yang ikut berubah setiap kali teksnya berubah.

![Graph dua dokumen. Di atas, domain Pembayaran dan Layanan pelanggan. Di tengah, dokumen pembayaran.md dan support.md, masing-masing terhubung ke domainnya. Di bawah, bagian Metode bayar dan Refund milik pembayaran.md, serta Keluhan pelanggan dan Jam layanan milik support.md. Edge berwarna merujuk dari Keluhan pelanggan ke Refund, berasal dari link markdown di support.md. Kartu di bawah menunjukkan isi node Refund: lokasi, ringkasan, kata kunci, dan fingerprint.](assets/anatomi-knowledge-graph.png)

*Struktur dibaca dari heading dan link. AI hanya mengisi ringkasan dan kata kunci.*

### Bukankah knowledge graph biasanya dibangun oleh AI?

Microsoft GraphRAG, salah satu pendekatan yang terkenal, memakai model AI untuk mengekstrak knowledge graph dari teks mentah. Cara itu cocok untuk teks tanpa struktur. Tetapi dokumentasi GraphRAG sendiri memperingatkan bahwa cara ini bisa menghabiskan banyak sumber daya model AI.

Knowledge graph yang kita bangun mengambil jalan yang lebih murah. Struktur, yaitu dokumen, bagian, dan link, dibaca dari markdown tanpa AI. Model AI hanya dipakai untuk menulis ringkasan, kata kunci, dan domain, dan itu pun hanya untuk bagian yang berubah.

Kelemahannya, graph ini hanya sebaik dokumenmu. Dokumen tanpa heading menjadi satu node besar, dan dua dokumen yang berhubungan tetapi tidak saling link tidak punya edge di antaranya.

## Cara kerja sinkronisasi knowledge graph adalah "hanya yang berubah yang diringkas ulang"

Knowledge graph harus mengikuti dokumen yang terus berubah, dan meringkas ulang semuanya setiap kali ada perubahan akan mahal. Proses menyamakan graph dengan isi file disebut sinkronisasi. Kuncinya adalah fingerprint tadi, yang dihitung dari teks sebuah bagian dengan fungsi hash.

Fungsi hash menghasilkan angka yang sama untuk teks yang sama, dan angka yang hampir pasti berbeda jika satu huruf saja berubah. Jadi, membandingkan dua fingerprint sudah cukup untuk tahu apakah sebuah bagian berubah.

Sinkronisasi berjalan dalam enam langkah:

1. Baca setiap file markdown, lalu pecah menjadi dokumen dan bagian menurut heading-nya.
2. Hitung fingerprint teks setiap bagian.
3. Bandingkan dengan fingerprint yang tersimpan di graph. Bagian yang fingerprint-nya sama tidak disentuh sama sekali.
4. Tandai bagian yang fingerprint-nya berbeda sebagai "perlu ringkasan". Ringkasan lamanya tetap disimpan sampai ada yang baru.
5. Hapus file dan heading yang sudah hilang dari graph. Heading yang hanya diganti namanya, dengan teks yang sama, membawa ringkasan lamanya.
6. Setelah semua dokumen ada, baca ulang link di setiap bagian menjadi edge "merujuk ke". Link ke web, gambar, dan teks di dalam blok kode diabaikan.

Keenam langkah itu tidak memakai AI, sehingga cepat dan hasilnya selalu sama. Setelah itu, langkah AI yang terpisah membaca bagian yang ditandai, lalu menulis ringkasan dan kata kuncinya, dengan satu panggilan model untuk setiap dokumen yang punya bagian bertanda.

![Tabel lima bagian dari pembayaran.md dengan fingerprint lama dan baru. Metode bayar dan Biaya admin punya fingerprint yang sama, sehingga tidak disentuh. Refund berubah dan Refund sebagian baru muncul, sehingga keduanya ditandai perlu ringkasan dan dikirim ke langkah AI. Cicilan hilang dari file, sehingga dihapus dari graph. Lima bagian diperiksa, tetapi hanya dua yang dikirim ke AI.](assets/sinkronisasi-fingerprint.png)

*Biaya AI mengikuti jumlah bagian yang berubah, bukan ukuran proyek.*

Hasilnya, jika kamu mengubah satu paragraf di dokumen yang punya 40 bagian, hanya satu bagian yang diringkas ulang. Ringkasan yang ditulis manusia juga tidak pernah ditimpa AI, sehingga perbaikanmu atas ringkasan yang keliru akan bertahan.

### Bukankah ringkasan lama menjadi salah setelah teks sebuah bagian berubah?

Memang, di antara saat teks diubah dan saat langkah AI berjalan, ringkasan lama masih menggambarkan teks yang lama. Karena itulah bagian tersebut ditandai, sehingga agent dan kamu tahu ringkasannya belum diperbarui. Pencarian juga membaca teks file yang terbaru, bukan hanya ringkasannya, sehingga isi yang baru tetap bisa ditemukan sebelum diringkas.

## Cara AI agent bertanya ke knowledge graph adalah "jawaban dengan alamat"

AI agent tidak membaca knowledge graph secara langsung, tetapi lewat MCP. MCP, singkatan dari Model Context Protocol, adalah standar open source untuk menghubungkan aplikasi AI ke sistem lain, yang dibuka oleh Anthropic pada 25 November 2024. Sebuah server MCP menyediakan tool, yaitu fungsi yang bisa dipanggil sendiri oleh model saat ia memutuskan membutuhkannya.

Karena MCP adalah standar, Claude Code, Codex, dan Cursor bisa terhubung ke server MCP yang sama. Jadi, satu knowledge graph bisa dibaca oleh agent mana pun yang kamu pakai.

Server MCP knowledge graph kita menyediakan dua tool untuk bertanya:

- **Pencarian** mengembalikan node yang mengandung semua kata yang ditanyakan. Tool ini cocok saat agent sudah tahu istilahnya.
- **Penjelasan** membuang kata umum seperti "yang" dan "bagaimana", lalu menerima node yang cocok dengan kata mana pun. Hasilnya dilengkapi domain, node tetangga yang terhubung lewat edge, dan potongan teks asli jika ringkasannya belum ada.

Keduanya menilai judul, kata kunci, ringkasan, dan teks file terbaru, dan setiap hasil membawa lokasinya, berupa nama file dan nomor baris.

Kita ikuti satu permintaan, yaitu memperpanjang batas waktu refund menjadi 14 hari kerja. Langkah-langkahnya adalah:

1. Agent memanggil tool penjelasan dengan kata "batas waktu refund".
2. Graph mengembalikan bagian "Refund" di `pembayaran.md:42` beserta ringkasannya. Ia juga mengembalikan tetangganya, bagian "Keluhan pelanggan" di `support.md:17`, yang merujuk ke sana.
3. Agent membuka kedua lokasi itu saja, bukan seluruh folder dokumen.
4. Agent mengubah kode dan memperbarui kedua bagian dokumen, lalu menyebut lokasi yang ia baca di jawabannya.

Langkah 2 adalah inti dari "jawaban dengan alamat". Agent tahu harus membaca di mana, termasuk dokumen lain yang ikut terdampak lewat edge "merujuk ke".

![Diagram urutan dengan empat jalur, yaitu AI agent, server MCP, knowledge graph, dan file proyek. Pertama, agent memanggil tool penjelasan dengan kata batas waktu refund. Kedua, server MCP mencari node yang cocok di knowledge graph. Ketiga, jawaban berwarna kembali ke agent berisi Refund di pembayaran.md baris 42 dan Keluhan pelanggan di support.md baris 17. Keempat, agent hanya membuka baris 42 dan 17 di file proyek.](assets/agent-bertanya-lewat-mcp.png)

*Agent hanya membuka lokasi yang disebut jawabannya.*

### Kenapa jawaban knowledge graph harus membawa lokasi file dan baris?

Bukankah ringkasan sudah cukup sebagai jawaban? Ringkasan memang hemat token, tetapi ditulis AI, dan AI bisa salah meringkas. Lokasi membuat agent membaca kalimat aslinya sebelum bertindak, dan membuat kamu bisa memeriksa jawabannya sendiri.

Kelemahannya, agent tetap membaca file aslinya, sehingga tokennya tidak sehemat jika ia hanya membaca ringkasan.

## Kelemahan knowledge graph proyek, dan kapan kamu tidak membutuhkannya

Knowledge graph membuat agent lebih jarang membaca ulang dan jawabannya lebih mudah diperiksa, tetapi ia satu sistem lagi yang harus dirawat. Ada empat kelemahan yang perlu kamu timbang. Keempatnya adalah:

- **Ringkasan AI ada biayanya.** Sinkronisasi pertama di proyek besar meringkas semua bagian, dan setiap perubahan berikutnya menambah panggilan ke model. Biayanya kecil dibanding meringkas ulang semuanya, tetapi tidak nol.
- **Ringkasan bisa usang atau salah.** Ringkasan tertinggal dari teks sampai langkah AI berjalan lagi, dan model AI bisa salah meringkas. Karena itulah agent perlu membaca lokasi aslinya.
- **Sinkronisasi butuh disiplin.** Graph hanya sebaru sinkronisasi terakhir. Proyek yang dokumennya jarang diperbarui akan punya graph yang rapi tetapi keliru.
- **Satu komponen lagi di setiap sesi.** Dokumentasi Claude menyebut definisi tool ikut dihitung di context window setiap permintaan. Server MCP dan database graph-nya juga harus dijalankan dan di-backup.

Tidak setiap proyek butuh knowledge graph. Grep sudah cukup untuk proyek kecil, atau untuk pertanyaan tentang nama yang persis ada di kode. CLAUDE.md atau AGENTS.md sudah cukup jika semua yang perlu diketahui agent muat dalam sekitar 200 baris, dan RAG dengan embedding lebih cocok untuk teks tanpa heading dan link.

Jadi, knowledge graph proyek cocok jika proyekmu punya banyak dokumen markdown yang saling merujuk, jika alasan di balik keputusan tertulis di dokumen, dan jika kamu ingin setiap jawaban agent bisa diperiksa sampai ke barisnya. Knowledge graph kurang cocok jika dokumenmu sedikit, jarang diperbarui, atau tidak pernah saling link. Graph tidak bisa lebih baik daripada dokumen yang ia baca.
