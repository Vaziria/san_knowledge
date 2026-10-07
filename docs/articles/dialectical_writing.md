# Dialectical writing

Articles teach by argument. Each idea is stated, tested against the strongest objection or alternative a sceptical reader would raise, and then resolved, with the conditions under which it holds and what it costs. This is the old pattern of thesis, antithesis and synthesis. The reader leaves with a conclusion they can trust, because they have seen it survive the best case against it.

This file is part of [guideline.md](guideline.md). The other rules still apply: "Educational only", the [wordlist](foreign_terms_wordlist.md), and facts only from sources.

## The reader starts from zero

Write as if the reader knows nothing when they begin. What they know is only what the article has already told them, built step by step as they read.

- **Headings name their subject.** A heading must make sense on its own, to someone who reads only the headings. Don't lean on "-nya" or on a subject left implied.
  - "Apa masalah yang diselesaikan oleh VPN?", not "Masalah yang diselesaikan"
  - "Apa yang membuat VPN privat", not "Apa yang membuatnya privat"
  - "Tiga protokol yang akan kamu temui di dalam VPN", not "Tiga protokol yang akan kamu temui"
  - "Cara kerja VPN adalah …", not "Cara kerjanya adalah …"
- **Sections open with the noun, not a pointer back.** Write "Cara kerja VPN bertumpu pada …", not "Cara kerjanya bertumpu pada …".
- **Explain each term the first time it appears,** by what it does, in a short clause ("…, yaitu …") or in a sentence of its own. This is an explanation, not a translation; the [wordlist](foreign_terms_wordlist.md) still rules out pairs like "terowongan (tunnel)". After that, use the term freely.
  - "tunnel, yaitu jalur tertutup yang membawa data dari satu komputer ke komputer lain melewati internet"
  - "Port adalah nomor yang menentukan program mana di sebuah komputer yang menerima koneksi."
- **Bridge from what the reader already knows.** Before a claim that builds on an earlier section, or turns against it, restate that piece in one sentence. Then show the gap, then make the claim. Never drop a claim the reader has no ground for.
  - Not: "Tunnel saja hanyalah pembungkus. Ada tiga sifat …", which comes right after the reader learned that the tunnel wraps and encrypts.
  - Write: "Di bagian sebelumnya, kita melihat tunnel membungkus paket di dalam paket lain supaya bisa menyeberangi internet. Tetapi pembungkusan saja belum membuat jaringan itu privat. Paket yang hanya dibungkus masih bisa dibaca, dipalsukan, atau diubah … Ketiga sifat itu adalah:"
  - Then each item answers one part of the gap: "Enkripsi mencegah paket dibaca", "Autentikasi mencegah paket dipalsukan", "Integritas mencegah paket diubah".
- **Every choice carries its reason.** Write "Seri ini dibangun di atas WireGuard, karena …", not a bare "Seri ini dibangun di atas WireGuard." Name its cost too, and where that cost comes back.
- **Anatomy before mechanism.** Before explaining how something works, show what it is made of: its parts, what each one does, and how they connect, with a diagram. Then walk one example through those parts, step by step.
  - In Part 1, "Anatomi VPN dan peran tunnel di dalamnya" names the four parts on each computer and the tunnel that joins them. Only then does "Cara kerja VPN …" follow one packet through them.
  - A central concept (the tunnel) gets its role spelled out: what it connects, what it looks like from above, and what it really is underneath.
- **Lay the ground before the problem.** If a section needs basics (paket, header, alamat, router), give them first, then make the claim.
- **Nothing before its explanation, diagrams included.**
  - A term in a diagram is explained in the text above it, or left out of the diagram.
  - A forward reference carries a one-line explanation: "WireGuard adalah salah satu protokol VPN yang dibahas di bagian berikutnya."

## The pattern

1. **Thesis:** the claim, or the usual answer. "WireGuard adalah jawaban yang biasa."
2. **Antithesis:** the strongest objection, limit or alternative. "Tapi WireGuard butuh satu sisi yang bisa dijangkau lewat UDP, dan di balik CGNAT tidak ada yang bisa."
3. **Synthesis:** what holds, under which conditions, and at what price. "Bungkus WireGuard di dalam koneksi keluar. Harganya: semua paket lewat relay."

The synthesis is never just the thesis again. It is a narrower, conditional and more honest version of it.

## Where it applies

- **The article as a whole:** the problem, why the usual answers fall short, the approach, and what it costs.
- **Each design choice:** the choice, the alternative not taken and why, and the price paid.
- **Each comparison** (protocols, topologies, tools): no winner without the condition under which it wins.
- **A series:** the open objection at the end of one part is what the next part answers. "Berikutnya dalam seri ini" names that objection.

## Rules

- **Steelman the other side.** Present the alternative at its best, as its own users would describe it. Never a strawman.
- **Voice the reader's question.** Say the objection out loud, as a heading or an opening sentence: "Kenapa tidak sewa VPS saja?", "Bukankah WireGuard sudah cukup?"
- **One objection per point: the strongest.** Don't stack weak ones to make a point look well defended.
- **Every synthesis names its cost.** What it gives up, in plain terms: latency, bandwidth, money, trust, complexity. A design article ends with its own section on costs.
- **Conditions over verdicts.** "Cocok jika …, kurang cocok jika …" instead of "X lebih baik".
- **Evidence decides.** Numbers, behaviour, and sources that were actually opened, never taste. When the evidence is missing, say the question is still open.
- **No fake balance.** If an alternative is simply worse for the case at hand, say so and say why.
- **Close every objection you open.** Answer it, or name it as an open question for a later part. An objection left hanging reads as a hole in the argument.

## Sentences

- **No colons in running text.** Finish the claim as its own sentence, then continue in the next one.
  - Examples start with "Seperti". Write "Alamat itu berlaku sama di mana pun komputernya berada. Seperti di rumah, di LAN kantor, atau di Wi-Fi hotel." Not "…berada: di rumah, di LAN kantor, atau di Wi-Fi hotel."
  - An explanation becomes a full sentence. Write "Ukurannya kecil. Implementasi Linux aslinya sekitar 4.000 baris kode…" Not "Ukurannya kecil: implementasi Linux aslinya…"
  - An enumeration names its items in a sentence. Write "Cara kerjanya bertumpu pada dua komponen. Yang pertama adalah …, dan yang kedua adalah …"
  - This covers prose lines inside diagrams too.
  - Colons may stay in short diagram labels, image alt text, and a line that introduces a bullet list or a code block.
  - A line that introduces a code block is a lead-in too, ending in "adalah:". Write "Perintahnya adalah:" or "Isinya adalah:", not "…dengan perintah ini:".
- **Headings: "X adalah "Y"", not "X: Y".** Link the two halves with "adalah", and put the key phrase in quotes so it stands out.
  - Write `## Cara kerja VPN adalah "paket di dalam paket"`, not `## Cara kerja VPN: paket di dalam paket`.
  - Write `## Masalah saat membuat VPN sendiri adalah "harus ada yang bisa dijangkau"`, not `## Masalah saat membuat VPN sendiri: harus ada yang bisa dijangkau`.
- **A list after a claim gets a lead-in.** Don't follow a claim straight with bullets. End the claim, then add a short sentence that says how the list relates to it, ending in a colon.
  - Reasons: "Komputer di jaringan yang berbeda tidak bisa begitu saja saling terhubung. Alasannya adalah:"
  - Items: "Kata ini dipakai untuk dua produk berbeda yang berbagi satu mekanisme. Kedua produk itu adalah:"
  - Ways or steps: "Caranya adalah:"
  - Examples: "Contohnya adalah:"
  - A lead-in that already names the list needs nothing more, as in "Ada tiga cara yang umum untuk mengatasinya:"
  - A list never starts straight under a heading. Put a claim and a lead-in first: "Tunnel yang sama bisa disusun dalam tiga bentuk, tergantung siapa terhubung ke siapa. Ketiganya adalah:"

## Phrases

| Move | Indonesian |
|---|---|
| Give examples, after the claim | "Seperti …" |
| Lead into a list of reasons | "Alasannya adalah:" |
| Lead into a list of items | "Kedua … itu adalah:", "Ketiganya adalah:" |
| Lead into a list of ways or steps | "Caranya adalah:" |
| Raise an objection | "Tapi …", "Masalahnya …", "Kenapa tidak … saja?", "Bukankah …?" |
| Concede | "Itu benar untuk …, tetapi …", "Memang …, hanya saja …" |
| Resolve | "Jadi, …", "Hasilnya, …" |
| Name the cost | "Harganya: …", "Yang dikorbankan: …" |
| Set conditions | "Cocok jika …, kurang cocok jika …" |
| Leave it open | "Pertanyaan ini kita jawab di Bagian …" |

## Example: Part 1, 'Masalah saat membuat VPN sendiri adalah "harus ada yang bisa dijangkau"'

- **Thesis:** a classic VPN needs one side that can be reached, through a server with an IP publik and an open port.
- **Antithesis:** behind CGNAT, a home connection has no IP publik, and port forwarding on the ISP's router is impossible.
- **Synthesis:** three ways around it, each with its condition and cost:
  - rent a server: it works, but every packet goes through it, and it costs money
  - hole punching: it often works, but some CGNATs defeat it
  - a relay: it works wherever outbound connections work
- **Open objection for the next part:** how to build the relay. "Berikutnya dalam seri ini" points there.

## Checklist before handing over

- Does each major section state a claim, its strongest objection, and a resolution?
- Is every alternative described the way its own users would describe it?
- Does every resolution name its conditions and its cost?
- Is any objection left unanswered? Answer it, or name it as an open question for a later part.
