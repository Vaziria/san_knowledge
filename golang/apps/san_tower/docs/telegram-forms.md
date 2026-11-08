# Telegram forms: the design

Fill in data through a Telegram chat so a tower workflow can use it (for
example, upload a movie to MovieMov) without opening the tower or the site.
The user (San) and the assistant agreed the design step by step on
2026-11-08. San's answers are quoted. The README (Workflows › Telegram
forms) documents the steps as built, and `examples/telegram-moviemov` uses
them.

## 1. Start: "c", both a /start menu and commands

- **Telegram Command** is a trigger with a Commands table (Command and
  Description). Each command has its own output (`/upload`), with Text
  being what follows it (`/hapus night-train` gives `night-train`). The
  commands of the active workflows, plus `/cancel`, make up the bot's /
  menu (setMyCommands).
- **Telegram Menu** sends a message with buttons. Each button is a command
  "as if typed" (`upload`, or `hapus night-train`). A tap starts that
  command's output of the same Telegram Command trigger, so the menu
  button and the command lead into the same branch, and each flow is built
  once. `/start` is a command like the others: link its output to the
  menu.
- This differs from the first sketch, where the menu had one branch per
  button. A button that is a command needs no second set of links, and a
  menu can never drift from the commands.

## 2. Who: "a", only listed Telegram users

- **Allowed Users** (Telegram user ids) is a setting of the Telegram Bot
  connection (`TelegramBotSettings.allowed_users`). It covers every
  workflow on that bot.
- Someone not on the list gets "Maaf, tidak punya akses (ID kamu: <id>)",
  so San can copy the id into the list. Nothing runs.
- **An empty list** keeps old workflows working:
  - Telegram Message takes every message, as before.
  - Telegram Command refuses every command. Its trigger state in the
    editor says "Allowed Users is empty on the bot connection…".
  - Telegram Menu and Form fail with the same reason in the run's log.
  - A form anyone could fill is not what San asked for, so the new steps
    are closed by default. The old trigger keeps its old behaviour until
    San fills the list.
- With a list, the Telegram Message trigger also takes only the listed
  people's messages, plus a channel's posts, which have no person. This is
  "only listed users", for every workflow on the bot.

## 3. Questions: "b", written by hand in the Telegram Form

- Questions are a table edited row by row on the step: Name, Question,
  Type, Required, Default, Choices, Min, Max, Starts With, File Types and
  Field.
- Types:
  - Text, Number, Link, Link or Iframe
  - Choice, and Multiple Choice (buttons with ✅, then Selesai)
  - List (comma-separated)
  - Yes/No
  - Photo (a Telegram photo, or an image sent as a file)
  - File
  - Date, and Date Time in WIB
- The questions are asked in order. A wrong answer is asked again, with
  the reason first ("Link harus diawali https://").
- An optional question gets a **Lewati** button. A Default gets a "Pakai
  Draft" button.
- Photos and files are downloaded (getFile) into the run's own folder,
  which is removed when the run ends.

## 4. Answers: "a", one output per question

- Each question is an output named after it, and Answers holds them all.
- Renaming a question in the editor moves its links. Removing it drops
  them.
- A photo or file answer is a path, so a thumbnail links straight into
  MovieMov Upload Movie's Thumbnail.
- This needed settings-dependent sockets, which the tower lacked. They are
  `StepKind.outputs_from` (a `SocketsFrom` per table setting) and
  `Setting.columns` (a table setting).
  - The engine works them out in `engine.StepOutputs` (link checks,
    import, runs).
  - The editor works them out in `stepOutputs` in `catalog.ts`.
  - Kinds without them are unchanged.

## 5. Finish: "a", a summary with buttons before the job runs

- The summary reads "Cek dulu ya:", then each answer, then [Kirim] [Ubah]
  [Batal].
- Ubah asks "Mau ubah yang mana?" with a button per question, asks that
  one question again, and then shows the summary again.
- Kirim fires Done.

## 6. Problems: "Ok" to all

- **Cancel.** /cancel or Batal ends the form at any time with "Dibatalkan"
  and fires the Cancelled output.
- **Timeout.** Timeout defaults to 10m and can be changed per form, from
  10s to 24h. When it runs out the bot says "Waktu habis, form
  dibatalkan" and fires Timed Out.
- **The site refuses** (for example MovieMov's 422 "slug: sudah dipakai"):
  - Each question has an optional **Field**: the site's fields it is sent
    as (`title, slug`, `embed_url`).
  - MovieMov Upload and Update have a **Refused** output with Reason and
    **Errors** (messages by field). Link Refused to the form's **Fix**
    input and Errors to the form's **Errors** input.
  - The run goes back into the form step. It says "Ditolak: <reason>",
    asks the questions whose fields were refused, and fires Done again, so
    the upload is retried.
  - After Tries (default 3) the bot says "Masih ditolak setelah 3 kali
    coba: … Form dibatalkan." and the step fails (its Failed output and
    Error).
- **A new command while a form is open.** The bot asks "Form yang tadi
  belum selesai. Batalkan dan mulai baru?" [Ya] [Tidak]. Ya cancels the
  form and runs the command. Tidak says "Oke, lanjut." and asks again.
- **An old button tapped later.** The tap is answered "Form ini sudah
  selesai", the button is removed, and nothing runs.
  - Buttons carry `f:<run>:<step>:<n>:` and an action in callback_data,
    well within Telegram's 64 bytes.
  - Menu buttons carry `m:<command>`.
- **A tower restart during a form** loses the form.
  - Each open form is a row in the store (TelegramConversation), removed
    when the form ends.
  - At start, every row left over is a form the stop cut off. The bot
    tells that chat "Maaf, form terputus, silakan mulai lagi".

## Routing

- Each person (bot + chat + user) has at most one open form.
- While it is open, that person's messages and button taps go to the
  waiting form step, not to new trigger runs. Other people and other chats
  work normally.
- Every tap is answered (answerCallbackQuery). The menu or question
  message is edited to show the pick, without its buttons.
- getUpdates now asks for `callback_query` as well as `message` and
  `channel_post`.
- The form step waits inside its run, the way OpenAI Reply waits for its
  request. The bot's poller hands that person's updates to the form.
