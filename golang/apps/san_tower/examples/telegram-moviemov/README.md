# Telegram MovieMov example

This workflow runs MovieMov from a Telegram chat, for the people on the
bot's Allowed Users only (the user, 2026-11-08: "fill in data through a
Telegram chat so a tower workflow can use it, e.g. upload a movie to
MovieMov, without opening the tower or the site"). It was built in a tower
and exported (ExportWorkflows), so it is exactly the tower's format. See the
tower's README (Workflows › Telegram forms, and The MovieMov steps) and
`docs/telegram-forms.md` for the design.

| File | Workflow |
| --- | --- |
| `telegram-moviemov.workflow.json` | Telegram MovieMov |

## What it does

- **`/start`** sends "MovieMov: mau ngapain?" with three buttons: [Upload
  film] [Cek laporan] [Hapus film]. Each button is a command (`upload`,
  `laporan`, `hapus`), so the buttons and the commands lead to the same
  steps.
- **`/upload`** (or Upload film) runs a Telegram Form, then Vidara
  Thumbnail and MovieMov Upload Movie, then the bot says "Sudah diupload:
  <url>". The form's questions:

  | Name | Question | Type | Required | Checks |
  | --- | --- | --- | --- | --- |
  | `judul` | Judul filmnya apa? | Text | yes | at most 255 characters; Field `title, slug` |
  | `link` | Link film (https) atau kode iframe-nya? | Link or Iframe | yes | starts with https://; Field `embed_url` |
  | `kategori` | Kategorinya? | Multiple Choice (Action, Drama, Horor, Komedi, Thriller, Sci-Fi) | no (Lewati) | at most 10 |
  | `tag` | Tag-nya? | List | no (Lewati) | at most 20 |
  | `thumbnail` | Kirim thumbnail-nya (Lewati: pakai thumbnail dari Vidara) | Photo | no (Lewati) | at most 2 MB, a photo or an image file |
  | `status` | Statusnya? | Choice (Draft, Published) | yes | default Draft ("Pakai Draft") |

  Each answer is linked to the upload's input: Title, Embed URL,
  Categories, Tags and Status Value (Status From Input). The thumbnail
  goes through a **Vidara Thumbnail** step first (the user, 2026-11-08:
  when the question is skipped, Vidara's thumbnail is used), whose
  Thumbnail is the upload's:
  - A photo sent is its Own Thumbnail, passed on as it is; Vidara is not
    read.
  - Skipped (Lewati), it reads the `link` answer's Vidara video and, with
    Mode Download, saves its thumbnail in the run's folder, which MovieMov
    Upload Movie sends as a file, so the site keeps its own copy. Vidara's
    thumbnails are small and wide (320x132), not 16:9. Mode External Link
    would give only Vidara's picture link (link its Thumbnail URL to the
    upload's Thumbnail URL then), which breaks when Vidara moves domains.
  - A link that is not a Vidara video's, or a video without a thumbnail,
    goes up without one (Not Found, then Done).
  - When Vidara's page does not load, the bot says "Gagal ambil thumbnail
    dari Vidara: …. Coba /upload lagi dan kirim fotonya." and nothing is
    uploaded.

  When the site refuses the movie (422, such as "The slug has already been
  taken."), Refused and Errors go back to the form's Fix and Errors. The
  bot says "Ditolak: …", asks that question again, and uploads again, at
  most 3 times. Any other failure says "Gagal upload: …".
- **`/laporan`** (or Cek laporan) runs MovieMov Reports. The bot sends a
  line per reported movie ("• Night Train (night-train): 2 laporan"), then
  "N film dilaporkan rusak.".
- **`/hapus`** (or Hapus film) runs a form that asks "Slug filmnya?" and
  then "Yakin hapus film ini?" [Ya] [Tidak]. `/hapus night-train` answers
  the slug from the command's text. The result goes through a Prefix
  Filter on `Yes`, then MovieMov Delete Movie. The bot then says "Sudah
  dihapus: <slug>", "Film tidak ditemukan: <slug>", or "Tidak jadi
  dihapus.".
- At any time, `/cancel` or Batal says "Dibatalkan". No answer within 10
  minutes says "Waktu habis, form dibatalkan".

## Import and switch on

1. Make the two connections, named as the file names them so the import
   picks them:
   - **MovieMov Bot**: a Telegram Bot connection (Connections page ›
     Telegram Bot).
     - Token: the token from @BotFather.
     - **Allowed Users**: your Telegram id. If you do not know it, save
       the connection first, send the bot `/start`, and copy the id from
       its answer "Maaf, tidak punya akses (ID kamu: …)".
   - **MovieMov**: a MovieMov connection.
     - Base URI: `https://moviemov.id/api`.
     - Username and Password: the admin's.
2. Go to Workflows page › Import and pick `telegram-moviemov.workflow.json`.
3. Open the workflow and switch it **Active**. The Telegram Command
   trigger's state reads "telegram @yourbot /start /upload /laporan
   /hapus". The bot's / menu then lists the commands, with /cancel.

A bot has one reader. Use this example's commands on a bot that no other
program reads, or add its steps to the workflow that already uses your bot.
Two active workflows can share a bot, as long as their commands differ.
